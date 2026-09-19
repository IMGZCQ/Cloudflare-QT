// Package config 负责解析项目内的目录布局。
// 约定：cloudflared 可执行文件、配置数据都放在项目目录内，
// 便于整体拷贝/打包（后期可通过 CFQT_HOME 环境变量覆盖根目录）。
package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const (
	// EnvHome 覆盖项目根目录
	EnvHome = "CFQT_HOME"
	// EnvSock 覆盖 unix socket 路径
	EnvSock = "CFQT_SOCK"
	// EnvAddr 覆盖 TCP 监听地址
	EnvAddr = "CFQT_ADDR"
	// EnvBasePath 覆盖统一网关路径前缀
	EnvBasePath = "CFQT_BASE_PATH"
	// EnvDataDir 覆盖持久化数据目录，用于指向 NAS 共享目录
	EnvDataDir = "CFQT_DATA_DIR"
	// EnvMirrors 覆盖 cloudflared 下载加速源，逗号分隔；
	// 填 off 或 direct 表示只用 GitHub 官方直连
	EnvMirrors = "CFQT_MIRRORS"

	// DefaultAddr 默认只监听回环地址，避免无认证接口暴露到局域网。
	// 需要局域网直连时显式设置 CFQT_ADDR=0.0.0.0:9970。
	DefaultAddr = "127.0.0.1:9970"
)

var (
	rootOnce sync.Once
	rootDir  string
)

// Root 返回项目根目录。
// 优先级：CFQT_HOME > 从工作目录向上查找 wails.json > 可执行文件所在目录。
func Root() string {
	rootOnce.Do(func() { rootDir = resolveRoot() })
	return rootDir
}

func resolveRoot() string {
	if v := strings.TrimSpace(os.Getenv(EnvHome)); v != "" {
		if abs, err := filepath.Abs(v); err == nil {
			return abs
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		if dir, ok := findUp(cwd, "wails.json"); ok {
			return dir
		}
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if found, ok := findUp(dir, "wails.json"); ok {
			return found
		}
		return dir
	}
	return "."
}

// findUp 从 start 开始逐级向上查找包含 marker 的目录
func findUp(start, marker string) (string, bool) {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// BinDir 存放 cloudflared 可执行文件的目录
func BinDir() string { return filepath.Join(Root(), "bin") }

// DataDir 存放持久化数据的目录
func DataDir() string {
	if v := strings.TrimSpace(os.Getenv(EnvDataDir)); v != "" {
		if abs, err := filepath.Abs(v); err == nil {
			return abs
		}
	}
	return filepath.Join(Root(), "data")
}

// StoreFile 隧道配置持久化文件
func StoreFile() string { return filepath.Join(DataDir(), "tunnels.json") }

// CloudflaredPath 项目内 cloudflared 可执行文件路径
func CloudflaredPath() string {
	name := "cloudflared"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(BinDir(), name)
}

// SockPath unix socket 路径
func SockPath() string {
	if v := strings.TrimSpace(os.Getenv(EnvSock)); v != "" {
		return v
	}
	return filepath.Join(Root(), "Cloudflare-QT.sock")
}

// Addr TCP 监听地址
func Addr() string {
	if v := strings.TrimSpace(os.Getenv(EnvAddr)); v != "" {
		return v
	}
	return DefaultAddr
}

// BasePath fnOS 统一网关路径前缀，例如 /app/cfquicktunnel
func BasePath() string {
	v := strings.TrimSpace(os.Getenv(EnvBasePath))
	if v == "" {
		return ""
	}
	return "/" + strings.Trim(v, "/")
}

// EnsureDirs 创建运行所需目录
func EnsureDirs() error {
	for _, dir := range []string{BinDir(), DataDir()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}
