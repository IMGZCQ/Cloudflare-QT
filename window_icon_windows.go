//go:build windows

package main

import (
	"context"
	"os"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	shell32              = syscall.NewLazyDLL("shell32.dll")
	procSendMessageW     = user32.NewProc("SendMessageW")
	procEnumWindows      = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible  = user32.NewProc("IsWindowVisible")
	procExtractIconExW   = shell32.NewProc("ExtractIconExW")
)

const (
	WM_SETICON = 0x0080
	ICON_SMALL = 0
	ICON_BIG   = 1
)

// setWindowIcon 把可执行文件里的图标设置到 Wails 窗口标题栏。
// 通过 ExtractIconExW 从当前 exe 提取图标，然后遍历当前进程窗口发送 WM_SETICON。
func setWindowIcon(ctx context.Context) {
	// 获取当前 exe 路径
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	exePathPtr, err := syscall.UTF16PtrFromString(exePath)
	if err != nil {
		return
	}

	// 从 exe 提取大小图标
	var hIconLarge, hIconSmall uintptr
	ret, _, _ := procExtractIconExW.Call(
		uintptr(unsafe.Pointer(exePathPtr)),
		0, // 第一个图标
		uintptr(unsafe.Pointer(&hIconLarge)),
		uintptr(unsafe.Pointer(&hIconSmall)),
		1,
	)
	if ret == 0 || hIconLarge == 0 {
		return
	}
	if hIconSmall == 0 {
		hIconSmall = hIconLarge
	}

	// 获取当前进程 ID
	pid := syscall.Getpid()

	// 重试几次，等待窗口创建完成
	for i := 0; i < 20; i++ {
		hwnd := findMainWindow(pid)
		if hwnd != 0 {
			procSendMessageW.Call(hwnd, WM_SETICON, ICON_SMALL, hIconSmall)
			procSendMessageW.Call(hwnd, WM_SETICON, ICON_BIG, hIconLarge)
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// findMainWindow 找到当前进程的第一个可见顶层窗口
func findMainWindow(pid int) uintptr {
	var result uintptr
	cb := syscall.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		var wpid uint32
		procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&wpid)))
		if int(wpid) == pid {
			visible, _, _ := procIsWindowVisible.Call(hwnd)
			if visible != 0 {
				result = hwnd
				return 0 // 停止枚举
			}
		}
		return 1 // 继续枚举
	})
	procEnumWindows.Call(cb, 0)
	return result
}
