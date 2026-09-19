//go:build windows

package tunnel

import (
	"os"
	"os/exec"
	"syscall"
)

// createNoWindow 对应 Windows 的 CREATE_NO_WINDOW，子进程不分配控制台窗口
const createNoWindow = 0x08000000

// terminate Windows 不支持 SIGTERM，直接结束进程
func terminate(p *os.Process) error {
	return p.Kill()
}

// hideWindow 让 cloudflared 子进程在后台运行，不弹出黑色控制台窗口
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
}
