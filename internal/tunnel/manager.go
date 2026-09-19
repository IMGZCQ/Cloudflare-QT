package tunnel

import (
	"context"
	"errors"
	"os/exec"
	"regexp"
	"sync"
	"time"

	"cfquicktunnel/internal/store"
)

// State 隧道运行状态
type State string

const (
	StateStopped  State = "stopped"
	StateStarting State = "starting"
	StateRunning  State = "running"
	StatePaused   State = "paused"
	StateError    State = "error"
)

// maxLogLines 每条隧道保留的最大日志行数
const maxLogLines = 200

// urlPattern 匹配 cloudflared 输出里的临时隧道地址
var urlPattern = regexp.MustCompile(`https://[a-z0-9][a-z0-9-]*\.trycloudflare\.com`)

// Item 前端展示用的隧道视图（配置 + 运行态）
type Item struct {
	store.Tunnel
	State     State  `json:"state"`
	URL       string `json:"url"`
	PID       int    `json:"pid"`
	StartedAt int64  `json:"startedAt"`
	LastError string `json:"lastError"`
}

// instance 单条隧道的运行态
type instance struct {
	mu        sync.Mutex
	state     State
	url       string
	pid       int
	startedAt int64
	lastError string
	logs      []string
	cmd       *exec.Cmd
	proxy     *localProxy
	stopping  bool
	queued    bool               // 已排入自动启动队列，尚未轮到
	pending   bool               // 后台启动任务进行中
	requeue   bool               // 后台启动期间配置又变了，需再跑一轮
	cancel    context.CancelFunc // 取消后台启动任务
}

func (in *instance) appendLog(line string) {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.logs = append(in.logs, time.Now().Format("15:04:05")+" "+line)
	// 环形裁剪：超出时复制到新切片，避免底层数组无限增长
	if len(in.logs) > maxLogLines {
		kept := make([]string, maxLogLines)
		copy(kept, in.logs[len(in.logs)-maxLogLines:])
		in.logs = kept
	}
}

func (in *instance) snapshot() (State, string, int, int64, string) {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.pending || in.queued {
		// 后台重启或排队等待中：统一对外显示“启动中”，避免旧地址与中间态闪现
		return StateStarting, "", in.pid, in.startedAt, ""
	}
	return in.state, in.url, in.pid, in.startedAt, in.lastError
}

// Manager 组合配置仓库与 cloudflared 进程管理
type Manager struct {
	st  *store.Store
	bin binary

	mu   sync.Mutex
	inst map[string]*instance
}

// NewManager 创建管理器
func NewManager(st *store.Store) *Manager {
	return &Manager{st: st, inst: make(map[string]*instance)}
}

// BinaryStatus 返回 cloudflared 状态
func (m *Manager) BinaryStatus() BinaryStatus { return m.bin.Status() }

// DownloadBinary 手动触发下载
func (m *Manager) DownloadBinary(ctx context.Context) error { return m.bin.Download(ctx) }

// getInstance 获取（或创建）运行态记录
func (m *Manager) getInstance(id string) *instance {
	m.mu.Lock()
	defer m.mu.Unlock()
	in, ok := m.inst[id]
	if !ok {
		in = &instance{state: StateStopped}
		m.inst[id] = in
	}
	return in
}

// List 返回全部隧道视图
func (m *Manager) List() []Item {
	cfgs := m.st.List()
	items := make([]Item, 0, len(cfgs))
	for _, cfg := range cfgs {
		items = append(items, m.itemOf(cfg))
	}
	return items
}

func (m *Manager) itemOf(cfg store.Tunnel) Item {
	state, url, pid, startedAt, lastErr := m.getInstance(cfg.ID).snapshot()
	return Item{
		Tunnel:    cfg,
		State:     state,
		URL:       url,
		PID:       pid,
		StartedAt: startedAt,
		LastError: lastErr,
	}
}

// Get 返回单条隧道视图
func (m *Manager) Get(id string) (Item, error) {
	cfg, ok := m.st.Get(id)
	if !ok {
		return Item{}, store.ErrNotFound
	}
	return m.itemOf(cfg), nil
}

// Create 新增隧道配置，autoStart 为真时在后台启动
func (m *Manager) Create(cfg store.Tunnel) (Item, error) {
	saved, err := m.st.Add(cfg)
	if err != nil {
		return Item{}, err
	}
	item := m.itemOf(saved)
	if saved.AutoStart {
		m.startAsync(saved.ID)
		item.State = StateStarting
	}
	return item, nil
}

// Update 修改隧道配置。仅当本地目标变化时才重启运行中的隧道，改名、自动启动等改动不打断连接。
// 重启放到后台进行，接口立即返回“启动中”，前端不必等待临时地址。
// Paused 字段不由前端表单提交，这里总是继承旧值，避免编辑名称时把暂停状态悄悄清掉。
func (m *Manager) Update(id string, cfg store.Tunnel) (Item, error) {
	old, ok := m.st.Get(id)
	if !ok {
		return Item{}, store.ErrNotFound
	}
	// 继承持久化的暂停偏好，防止编辑其他字段时被覆盖
	cfg.Paused = old.Paused
	if err := cfg.Normalize(); err != nil {
		return Item{}, err
	}

	// 目标变化时，运行中/启动中/已暂停都需要重启（暂停偏好在重启后由 Start 恢复）
	state, _, _, _, _ := m.getInstance(id).snapshot()
	live := state == StateRunning || state == StateStarting || state == StatePaused
	restart := live && cfg.Target() != old.Target()

	// 先落库：配置校验或写入失败时不会连带停掉正在运行的隧道
	saved, err := m.st.Update(id, cfg)
	if err != nil {
		return Item{}, err
	}

	item := m.itemOf(saved)
	if restart {
		m.startAsync(saved.ID)
		item.State = StateStarting
		item.URL = ""
		item.LastError = ""
	}
	return item, nil
}

// Delete 停止并删除隧道
func (m *Manager) Delete(id string) error {
	if err := m.Stop(id); err != nil && !errors.Is(err, ErrNotRunning) {
		return err
	}
	if err := m.st.Delete(id); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.inst, id)
	m.mu.Unlock()
	return nil
}

// Logs 返回隧道日志
func (m *Manager) Logs(id string) ([]string, error) {
	if _, ok := m.st.Get(id); !ok {
		return nil, store.ErrNotFound
	}
	in := m.getInstance(id)
	in.mu.Lock()
	defer in.mu.Unlock()
	out := make([]string, len(in.logs))
	copy(out, in.logs)
	return out, nil
}

// Pause 暂停隧道：cloudflared 进程保持运行，本地代理切换为维护页面。
func (m *Manager) Pause(id string) error {
	if _, ok := m.st.Get(id); !ok {
		return store.ErrNotFound
	}
	in := m.getInstance(id)
	in.mu.Lock()
	if in.state != StateRunning {
		in.mu.Unlock()
		return ErrNotRunning
	}
	// 先落库，再切内存状态：避免落库失败后 UI 显示已暂停但重启后丢失
	if err := m.persistPaused(id, true); err != nil {
		in.mu.Unlock()
		return err
	}
	if in.proxy != nil {
		in.proxy.setPaused(true)
	}
	in.state = StatePaused
	in.mu.Unlock()
	in.appendLog("隧道已暂停，本地服务断开")
	return nil
}

// persistPaused 把暂停偏好写入配置，供程序重启后恢复暂停状态
func (m *Manager) persistPaused(id string, paused bool) error {
	cfg, ok := m.st.Get(id)
	if !ok || cfg.Paused == paused {
		return nil
	}
	cfg.Paused = paused
	_, err := m.st.Update(id, cfg)
	return err
}

// Resume 恢复隧道：本地代理恢复转发到真实服务。
func (m *Manager) Resume(id string) error {
	if _, ok := m.st.Get(id); !ok {
		return store.ErrNotFound
	}
	in := m.getInstance(id)
	in.mu.Lock()
	if in.state != StatePaused {
		in.mu.Unlock()
		return ErrNotPaused
	}
	if err := m.persistPaused(id, false); err != nil {
		in.mu.Unlock()
		return err
	}
	if in.proxy != nil {
		in.proxy.setPaused(false)
	}
	in.state = StateRunning
	in.mu.Unlock()
	in.appendLog("隧道已恢复，本地服务已连接")
	return nil
}
