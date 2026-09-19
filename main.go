// CF-QuickTunnel 桌面端入口（Wails）。
// 无头服务端入口见 cmd/server，两者共用 internal 下的核心逻辑与同一套前端。
package main

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"cfquicktunnel/frontend"
	"cfquicktunnel/internal/api"
	"cfquicktunnel/internal/logbuf"
	"cfquicktunnel/internal/store"
	"cfquicktunnel/internal/tunnel"
)

func main() {
	log.SetOutput(io.MultiWriter(os.Stderr, logbuf.New(500)))

	st, err := store.Open()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	mgr := tunnel.NewManager(st)

	err = wails.Run(&options.App{
		Title:  "Cloudflare快捷隧道",
		Width:  1000,
		Height: 700,
		AssetServer: &assetserver.Options{
			Assets:  frontend.Dist(),
			Handler: api.APIHandler(mgr),
		},
		OnStartup: func(ctx context.Context) {
			go func() {
				if st := mgr.BinaryStatus(); !st.Ready {
					_ = mgr.DownloadBinary(ctx)
				}
			}()
			go mgr.StartAll(ctx)
		},
		OnShutdown: func(ctx context.Context) {
			mgr.StopAll()
		},
	})
	if err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}
