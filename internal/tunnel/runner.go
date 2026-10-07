package tunnel

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
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

// ErrNotPausable 命名隧道不支持暂停
var ErrNotPausable = errors.New("命名隧道不支持暂停，请使用启动/停止")

// startWaitURL 等待 cloudflared 就绪（临时域名或连接注册成功）的最长时间
const startWaitURL = 30 * time.Second

// registeredPattern 匹配命名隧道与 Cloudflare 边缘连接建立的日志（命名隧道没有临时域名输出）
var registeredPattern = regexp.MustCompile(`Registered tunnel connection`)

// configPattern 匹配命名隧道云端下发配置的日志行，捕获其中转义后的 JSON 配置（含 ingress 主机名）
var configPattern = regexp.MustCompile(`Updated to new configuration config="(.*)"`)

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
	in.failReason = ""
	in.stopping = false
	in.exitCh = make(chan struct{})
	in.mu.Unlock()

	// 快捷隧道在 cloudflared 与本地服务之间插入本地代理，用于随时暂停/恢复且不丢失域名；
	// 命名隧道（Token 模式）的 ingress 由 Cloudflare 云端下发，本地无法插入代理，故不支持暂停。
	named := cfg.IsNamed()
	baseArgs := []string{"tunnel", "--no-autoupdate", "--protocol", "auto", "--edge-ip-version", cfg.EdgeIPArg(), "--retries", "20"}

	var proxy *localProxy
	var args []string
	if named {
		args = append(baseArgs, "run", "--token", cfg.Token)
	} else {
		proxy = newLocalProxy(cfg.Target())
		proxyAddr, err := proxy.start()
		if err != nil {
			m.markError(in, err)
			return err
		}
		args = append(baseArgs, "--url", proxyAddr)
	}

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
	exitCh := in.exitCh
	in.mu.Unlock()
	// 分隔历史日志，便于区分新一轮启动
	in.appendLog("────────── 启动隧道 ──────────")
	if named {
		in.appendLog("启动 cloudflared 命名隧道（Token 模式）")
	} else {
		in.appendLog("启动 cloudflared，目标 " + cfg.Target())
	}

	// 快捷隧道等待临时域名，命名隧道等待连接注册成功；两路都扫描以适配版本差异
	readyCh := make(chan string, 1)
	go scanOutput(stdout, in, readyCh, named)
	go scanOutput(stderr, in, readyCh, named)
	go m.wait(in, cmd, exitCh)

	select {
	case url := <-readyCh:
		in.mu.Lock()
		in.url = url
		if !named && cfg.Paused {
			// 恢复暂停偏好：域名照常分配，访客看到维护页面
			proxy.setPaused(true)
			in.state = StatePaused
		} else {
			in.state = StateRunning
		}
		in.mu.Unlock()
		if url != "" {
			in.appendLog("隧道地址: " + url)
		} else {
			in.appendLog("命名隧道连接已建立")
		}
		return nil
	case <-exitCh:
		// 进程已退出（崩溃/被强杀），wait() 已把真实原因写进 lastError
		in.mu.Lock()
		reason := in.failReason
		if reason == "" {
			reason = in.lastError
		}
		if reason == "" {
			reason = "cloudflared 意外退出"
		}
		in.mu.Unlock()
		return errors.New(reason)
	case <-time.After(startWaitURL):
		// 已识别出具体原因（如限流）时直接用它，而不是笼统的"查看日志"；
		// 若 wait() 已把状态置为 Error 并写入了友好文案，直接复用，避免重复打日志
		in.mu.Lock()
		reason := in.failReason
		alreadyMarked := in.state == StateError && in.lastError != ""
		in.mu.Unlock()
		if reason == "" {
			if named {
				reason = "等待命名隧道连接超时，请检查 Token 是否正确"
			} else {
				reason = "等待隧道地址超时，请查看日志"
			}
		}
		err := errors.New(reason)
		// 只停进程，不动 pending/requeue，避免把排队中的下一轮重启一起取消
		m.stopProcess(id)
		if !alreadyMarked {
			m.markError(in, err)
		}
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

// wait 回收子进程并更新状态；exitCh 由调用方传入，避免读 in.exitCh 时与新一轮 Start 并发冲突
func (m *Manager) wait(in *instance, cmd *exec.Cmd, exitCh chan struct{}) {
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
		if in.failReason != "" {
			// 已从输出识别出具体原因（如限流），直接展示友好文案
			in.lastError = in.failReason
		} else {
			in.lastError = "cloudflared 异常退出: " + err.Error()
		}
	} else {
		in.state = StateStopped
	}
	msg := in.lastError
	in.mu.Unlock()

	// 通知 Start() 的 select：进程已退出，不要傻等超时
	if exitCh != nil {
		close(exitCh)
	}

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

// scanOutput 逐行读取 cloudflared 输出，记录日志并提取隧道就绪信号。
// 快捷隧道就绪信号为临时域名；命名隧道无域名输出，就绪信号为空串（以连接注册成功为准）。
func scanOutput(r io.Reader, in *instance, readyCh chan<- string, named bool) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 512*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		in.appendLog(line)
		if reason := classifyFailure(line); reason != "" {
			in.mu.Lock()
			in.failReason = reason
			in.mu.Unlock()
		}
		if url := urlPattern.FindString(line); url != "" {
			select {
			case readyCh <- url:
			default:
			}
		} else if named {
			if host := parseHostname(line); host != "" {
				// 云端下发配置里带公网主机名，据此自动获知命名隧道的访问地址
				u := "https://" + host
				in.mu.Lock()
				logURL := false
				if in.state == StateStarting || in.state == StateRunning {
					if in.url != u {
						in.url = u
						// 启动阶段由 Start 统一打印地址，避免重复
						logURL = in.state == StateRunning
					}
				}
				in.mu.Unlock()
				if logURL {
					in.appendLog("隧道地址: " + u)
				}
				select {
				case readyCh <- u:
				default:
				}
			} else if registeredPattern.MatchString(line) {
				select {
				case readyCh <- "":
				default:
				}
			}
		}
	}
	// 新增：检查Scanner读取错误，消除scannererr警告
	if err := sc.Err(); err != nil {
		in.appendLog(fmt.Sprintf("scan output error: %v", err))
	}
}

// ingressConfig 云端下发的 ingress 配置（只关心公网主机名）
type ingressConfig struct {
	Ingress []struct {
		Hostname string `json:"hostname"`
	} `json:"ingress"`
}

// parseHostname 从“Updated to new configuration”日志行中提取第一个公网主机名；无法解析返回空串。
// 日志里的 config 是转义后的 JSON 字符串（形如 config="{\"ingress\":[...]}"），需先反转义再解析。
func parseHostname(line string) string {
	m := configPattern.FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	raw, err := strconv.Unquote(`"` + m[1] + `"`)
	if err != nil {
		return ""
	}
	var cfg ingressConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return ""
	}
	for _, rule := range cfg.Ingress {
		if rule.Hostname != "" {
			return rule.Hostname
		}
	}
	return ""
}

// classifyFailure 从 cloudflared 输出中识别明确的失败原因，返回友好提示；无法识别返回空串
func classifyFailure(line string) string {
	lower := strings.ToLower(line)
	// 429 + 1015：trycloudflare 快速隧道限流
	if strings.Contains(line, "429") && strings.Contains(line, "1015") {
		return "因短时间建立隧道过多，已被 Cloudflare 限流（429），请稍后再试"
	}
	if strings.Contains(lower, "provisioning failed") && strings.Contains(line, "429") {
		return "因短时间建立隧道过多，已被 Cloudflare 限流（429），请稍后再试"
	}
	return ""
}
