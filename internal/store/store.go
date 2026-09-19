// Package store 提供隧道配置的 JSON 持久化。
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"cfquicktunnel/internal/config"
)

// Tunnel 一条隧道配置（临时隧道的公网地址每次启动都会变化，因此不持久化）
type Tunnel struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Scheme    string `json:"scheme"` // http | https
	Host      string `json:"host"`   // 本地地址，如 127.0.0.1
	Port      int    `json:"port"`
	Path      string `json:"path"` // 可选路径前缀，如 /123/456/
	AutoStart bool   `json:"autoStart"`
	Paused    bool   `json:"paused"` // 暂停偏好，重启后恢复暂停状态
	CreatedAt int64  `json:"createdAt"`
}

// Target 拼出 cloudflared --url 需要的本地地址
func (t Tunnel) Target() string {
	return fmt.Sprintf("%s://%s:%d%s", t.Scheme, t.Host, t.Port, t.Path)
}

// Normalize 校验并补全字段
func (t *Tunnel) Normalize() error {
	t.Name = strings.TrimSpace(t.Name)
	t.Host = strings.TrimSpace(t.Host)
	t.Scheme = strings.ToLower(strings.TrimSpace(t.Scheme))
	t.Path = strings.TrimSpace(t.Path)

	if t.Scheme == "" {
		t.Scheme = "http"
	}
	if t.Scheme != "http" && t.Scheme != "https" {
		return errors.New("协议只支持 http 或 https")
	}
	if t.Host == "" {
		t.Host = "127.0.0.1"
	}
	if strings.ContainsAny(t.Host, " /\\?#") {
		return errors.New("本地地址格式不合法")
	}
	if t.Path != "" {
		if !strings.HasPrefix(t.Path, "/") {
			t.Path = "/" + t.Path
		}
		if strings.Contains(t.Path, "#") {
			return errors.New("路径格式不合法")
		}
	}
	if t.Port < 1 || t.Port > 65535 {
		return errors.New("端口需在 1-65535 之间")
	}
	if t.Name == "" {
		t.Name = t.Host + ":" + strconv.Itoa(t.Port)
	}
	return nil
}

// ParseTarget 从完整 URL 解析出 Tunnel 字段。
// 支持 http://127.0.0.1:8080/path?query 格式。
func ParseTarget(raw string) (Tunnel, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return Tunnel{}, fmt.Errorf("地址解析失败: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return Tunnel{}, errors.New("协议仅支持 http / https")
	}
	if u.Hostname() == "" {
		return Tunnel{}, errors.New("请输入完整地址")
	}
	port := 80
	if u.Scheme == "https" {
		port = 443
	}
	if p := u.Port(); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return Tunnel{}, errors.New("端口需在 1-65535 之间")
		}
		port = n
	}
	path := u.EscapedPath()
	if path == "/" {
		path = ""
	}
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	return Tunnel{
		Scheme: u.Scheme,
		Host:   u.Hostname(),
		Port:   port,
		Path:   path,
	}, nil
}

type fileData struct {
	Tunnels []Tunnel `json:"tunnels"`
}

// Store 线程安全的隧道配置仓库
type Store struct {
	mu   sync.RWMutex
	path string
	data fileData
}

// Open 读取（或初始化）持久化文件
func Open() (*Store, error) {
	if err := config.EnsureDirs(); err != nil {
		return nil, err
	}
	s := &Store{path: config.StoreFile()}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, s.flush()
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(raw))) > 0 {
		if err := json.Unmarshal(raw, &s.data); err != nil {
			return nil, fmt.Errorf("解析 %s 失败: %w", s.path, err)
		}
	}
	return s, nil
}

// List 返回全部隧道配置的副本
func (s *Store) List() []Tunnel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Tunnel, len(s.data.Tunnels))
	copy(out, s.data.Tunnels)
	return out
}

// Get 按 ID 查询
func (s *Store) Get(id string) (Tunnel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.data.Tunnels {
		if t.ID == id {
			return t, true
		}
	}
	return Tunnel{}, false
}

// Add 新增一条配置，返回带 ID 的结果
func (s *Store) Add(t Tunnel) (Tunnel, error) {
	if err := t.Normalize(); err != nil {
		return Tunnel{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	t.ID = newID()
	t.CreatedAt = time.Now().Unix()
	s.data.Tunnels = append(s.data.Tunnels, t)
	if err := s.flush(); err != nil {
		return Tunnel{}, err
	}
	return t, nil
}

// Update 覆盖已有配置（ID、创建时间保持不变）
func (s *Store) Update(id string, t Tunnel) (Tunnel, error) {
	if err := t.Normalize(); err != nil {
		return Tunnel{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := -1
	for i, exist := range s.data.Tunnels {
		if exist.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Tunnel{}, ErrNotFound
	}
	t.ID = id
	t.CreatedAt = s.data.Tunnels[idx].CreatedAt
	s.data.Tunnels[idx] = t
	if err := s.flush(); err != nil {
		return Tunnel{}, err
	}
	return t, nil
}

// Delete 删除配置
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, t := range s.data.Tunnels {
		if t.ID == id {
			s.data.Tunnels = append(s.data.Tunnels[:i], s.data.Tunnels[i+1:]...)
			return s.flush()
		}
	}
	return ErrNotFound
}

// ErrNotFound 隧道不存在
var ErrNotFound = errors.New("隧道不存在")

// flush 先写临时文件再原子重命名，避免写入中断导致配置损坏
func (s *Store) flush() error {
	if s.data.Tunnels == nil {
		s.data.Tunnels = []Tunnel{}
	}
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Clean(s.path))
}

// newID 生成随机 ID（6 字节随机数的 hex 编码），避免高并发下时间戳碰撞
func newID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		// 极端情况下（如系统熵池耗尽）退回到时间戳
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b[:])
}
