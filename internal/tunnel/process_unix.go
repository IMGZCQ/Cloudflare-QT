//go:build !windows

package tunnel

import (
	"os"
	"os/exec"
	"syscall"
)

// terminate 先发送 SIGTERM，让 cloudflared 有机会优雅退出
func terminate(p *os.Process) error {
	return p.Signal(syscall.SIGTERM)
}

// hideWindow 非 Windows 平台没有控制台窗口概念，无需处理
func hideWindow(*exec.Cmd) {}
