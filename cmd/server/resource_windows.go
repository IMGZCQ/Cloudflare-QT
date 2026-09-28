//go:build windows

package main

// 此文件仅用于确保 go build 时把 icon.syso 链接进可执行文件。
// icon.syso 由构建脚本通过 rsrc 生成，包含 Windows 图标与版本信息。
