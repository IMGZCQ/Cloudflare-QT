package tunnel

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"cfquicktunnel/internal/config"
	"cfquicktunnel/internal/store"
)

// ErrNotRunning 隧道未在运行
var ErrNotRunning = errors.New("隧道未在运行")

// ErrAlreadyRunning 隧道已在运行
var ErrAlreadyRunning = errors.New("隧道已在运行")

// ErrNotPaused 隧道未暂停
var ErrNotPaused = errors.New("隧道未暂停")

// startWaitURL 等待 cloudflared 输出临时域名的最长时间
const startWaitURL = 30 * time.Second

// Start 启动临时隧道：cloudflared tunnel --protocol http2 --url <target>
func (m *Manager) Start(ctx context.Context, id string) error {
	cfg, ok := m.st.Get(id)
	if !ok {
		return store.ErrNotFound
	}
	if err := m.bin.Ensure(ctx); err != nil {
		return err
	}

	in := m.getInstance(id)
	in.mu.Lock()
	if in.state == StateRunning || in.state == StateStarting || in.state == StatePaused {
		in.mu.Unlock()
		return ErrAlreadyRunning
	}
	in.state = StateStarting
	in.url = ""
	in.lastError = ""
	in.stopping = false
	in.mu.Unlock()

	// 启动本地代理：cloudflared 指向代理，而非直接指向本地服务
	// 这样可以随时暂停/恢复，而不丢失域名
	proxy := newLocalProxy(cfg.Target())
	proxyAddr, err := proxy.start()
	if err != nil {
		m.markError(in, err)
		return err
	}

	args := []string{"tunnel", "--no-autoupdate", "--protocol", "auto", "--edge-ip-version", cfg.EdgeIPArg(), "--retries", "20", "--url", proxyAddr}
	cmd := exec.Command(config.CloudflaredPath(), args...)
	hideWindow(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		proxy.stop()
		m.markError(in, err)
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		proxy.stop()
		m.markError(in, err)
		return err
	}
	if err := cmd.Start(); err != nil {
		proxy.stop()
		m.markError(in, err)
		return err
	}

	in.mu.Lock()
	in.cmd = cmd
	in.proxy = proxy
	in.pid = cmd.Process.Pid
	in.startedAt = time.Now().Unix()
	in.mu.Unlock()
	in.appendLog("启动 cloudflared，目标 " + cfg.Target())

	// cloudflared 把隧道地址写到 stderr，两路都扫描以适配版本差异
	urlCh := make(chan string, 1)
	go scanOutput(stdout, in, urlCh)
	go scanOutput(stderr, in, urlCh)
	go m.wait(in, cmd)

	select {
	case url := <-urlCh:
		in.mu.Lock()
		in.url = url
		if cfg.Paused {
			// 恢复暂停偏好：域名照常分配，访客看到维护页面
			proxy.setPaused(true)
			in.state = StatePaused
		} else {
			in.state = StateRunning
		}
		in.mu.Unlock()
		in.appendLog("隧道地址: " + url)
		return nil
	case <-time.After(startWaitURL):
		err := errors.New("等待隧道地址超时，请查看日志")
		// 只停进程，不动 pending/requeue，避免把排队中的下一轮重启一起取消
		m.stopProcess(id)
		m.markError(in, err)
		return err
	case <-ctx.Done():
		m.stopProcess(id)
		return ctx.Err()
	}
}

// startAsync 在后台重启隧道，调用方立即返回，状态由前端轮询收敛。
// 后台任务运行期间对外统一呈现“启动中”，期间再次触发只会排队一次。
func (m *Manager) startAsync(id string) {
	in := m.getInstance(id)
	in.mu.Lock()
	if in.pending {
		in.requeue = true
		in.mu.Unlock()
		return
	}
	in.pending = true
	in.requeue = false
	ctx := in.newRunCtx()
	in.mu.Unlock()

	go func() {
		for {
			m.stopProcess(id)
			startErr := m.Start(ctx, id)

			in.mu.Lock()
			again := in.requeue
			in.requeue = false
			in.dropRunCtx()
			if again {
				ctx = in.newRunCtx()
			} else {
				in.pending = false
			}
			// Start 内部已记录过的失败（管道错误、等待地址超时）无需重复标记
			needMark := startErr != nil && in.state != StateError &&
				!errors.Is(startErr, context.Canceled) &&
				!errors.Is(startErr, ErrAlreadyRunning) &&
				!errors.Is(startErr, store.ErrNotFound)
			in.mu.Unlock()

			if needMark {
				m.markError(in, startErr)
			}
			if !again {
				return
			}
		}
	}()
}

// newRunCtx 建立可取消的后台启动上下文，调用前需持有 in.mu
func (in *instance) newRunCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	in.cancel = cancel
	return ctx
}

// dropRunCtx 释放后台启动上下文，调用前需持有 in.mu
func (in *instance) dropRunCtx() {
	if in.cancel != nil {
		in.cancel()
		in.cancel = nil
	}
}

// Stop 停止隧道进程，同时取消可能在排队的后台启动任务
func (m *Manager) Stop(id string) error {
	in := m.getInstance(id)
	in.mu.Lock()
	in.queued = false
	in.requeue = false
	in.dropRunCtx()
	in.mu.Unlock()
	return m.stopProcess(id)
}

// stopProcess 结束 cloudflared 子进程
func (m *Manager) stopProcess(id string) error {
	if _, ok := m.st.Get(id); !ok {
		return store.ErrNotFound
	}
	in := m.getInstance(id)

	in.mu.Lock()
	cmd := in.cmd
	if cmd == nil || cmd.Process == nil {
		in.state = StateStopped
		in.url = ""
		in.pid = 0
		in.mu.Unlock()
		return ErrNotRunning
	}
	in.stopping = true
	proc := cmd.Process
	in.mu.Unlock()

	in.appendLog("停止隧道")
	if err := terminate(proc); err != nil {
		proc.Kill()
	}

	// 等 m.wait 回收子进程并清空 in.cmd；超时则强杀。
	// 不要在锁内空转，给 wait goroutine 让出调度机会。
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		in.mu.Lock()
		running := in.cmd != nil
		in.mu.Unlock()
		if !running {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	proc.Kill()
	// 强杀后再等一小段时间确保 wait 完成回收，避免 Windows 上进程对象泄漏
	grace := time.Now().Add(2 * time.Second)
	for time.Now().Before(grace) {
		in.mu.Lock()
		running := in.cmd != nil
		in.mu.Unlock()
		if !running {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

// StartAll 启动所有标记了自动启动的隧道。
// 先把待启动的隧道全部标记为"启动中"（queued=true，由 snapshot 转为对外状态），
// 再逐一启动；启动期间用户手动停止/删除会清 queued，循环时跳过。
func (m *Manager) StartAll(ctx context.Context) {
	cfgs := m.st.List()
	for _, cfg := range cfgs {
		if !cfg.AutoStart {
			continue
		}
		in := m.getInstance(cfg.ID)
		in.mu.Lock()
		in.queued = true
		in.mu.Unlock()
	}

	for _, cfg := range cfgs {
		if !cfg.AutoStart {
			continue
		}
		in := m.getInstance(cfg.ID)
		in.mu.Lock()
		queued := in.queued
		in.queued = false
		in.mu.Unlock()
		if !queued {
			// 排队期间被手动停止或删除，不再自动启动
			continue
		}
		if err := m.Start(ctx, cfg.ID); err != nil {
			in.appendLog("自动启动失败: " + err.Error())
		}
	}
}

// StopAll 停止全部隧道，用于进程退出前清理
func (m *Manager) StopAll() {
	for _, cfg := range m.st.List() {
		m.Stop(cfg.ID)
	}
}

// wait 回收子进程并更新状态
func (m *Manager) wait(in *instance, cmd *exec.Cmd) {
	err := cmd.Wait()

	in.mu.Lock()
	// 若 in.cmd 已被新一轮 Start 替换，说明这是旧进程的回收回调，不要覆盖新状态
	if in.cmd != cmd {
		in.mu.Unlock()
		return
	}
	in.cmd = nil
	in.pid = 0
	in.url = ""
	proxy := in.proxy
	in.proxy = nil
	stopping := in.stopping
	if stopping {
		in.state = StateStopped
		in.lastError = ""
	} else if err != nil {
		in.state = StateError
		in.lastError = "cloudflared 异常退出: " + err.Error()
	} else {
		in.state = StateStopped
	}
	msg := in.lastError
	in.mu.Unlock()

	if proxy != nil {
		proxy.stop()
	}

	if msg != "" {
		in.appendLog(msg)
	} else {
		in.appendLog("cloudflared 已退出")
	}
}

// markError 标记启动失败
func (m *Manager) markError(in *instance, err error) {
	in.mu.Lock()
	in.state = StateError
	in.lastError = err.Error()
	in.cmd = nil
	in.pid = 0
	in.url = ""
	in.mu.Unlock()
	in.appendLog("错误: " + err.Error())
}

// scanOutput 逐行读取 cloudflared 输出，记录日志并提取隧道地址
func scanOutput(r io.Reader, in *instance, urlCh chan<- string) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 512*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		in.appendLog(line)
		if url := urlPattern.FindString(line); url != "" {
			select {
			case urlCh <- url:
			default:
			}
		}
	}
	// 新增：检查Scanner读取错误，消除scannererr警告
	if err := sc.Err(); err != nil {
		in.appendLog(fmt.Sprintf("scan output error: %v", err))
	}
}
