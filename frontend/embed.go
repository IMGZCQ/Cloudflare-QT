// Package frontend 通过 go:embed 嵌入前端构建产物（frontend/dist），
// 供桌面端与无头服务端共用同一份静态资源。
// 构建顺序：先在 frontend/ 执行 npm run build，再 go build。
package frontend

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var assets embed.FS

// Dist 返回以 index.html 为根的静态资源文件系统
func Dist() fs.FS {
	sub, err := fs.Sub(assets, "dist")
	if err != nil {
		// dist 由 go:embed 保证存在，这里不会触发
		panic(err)
	}
	return sub
}
