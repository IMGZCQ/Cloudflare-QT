//go:build !windows

package main

import "context"

// setWindowIcon 在非 Windows 平台为空实现
func setWindowIcon(ctx context.Context) {}
