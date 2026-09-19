// CF-QuickTunnel 无头服务端：监听 unix socket 与本地 TCP 端口，
// 对外提供隧道管理界面与 REST 接口，用于 fnOS 统一网关接入。
package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"cfquicktunnel/frontend"
	"cfquicktunnel/internal/api"
	"cfquicktunnel/internal/config"
	"cfquicktunnel/internal/logbuf"
	"cfquicktunnel/internal/store"
	"cfquicktunnel/internal/tunnel"
)

// 编译时通过 -ldflags "-X main.Version=xxx" 注入
var Version = "dev"

func main() {
	log.SetFlags(log.LstdFlags)
	log.SetPrefix("[cfquicktunnel] ")
	log.SetOutput(io.MultiWriter(os.Stderr, logbuf.New(500)))
	log.Printf("CF-QuickTunnel %s 启动", Version)

	st, err := store.Open()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	mgr := tunnel.NewManager(st)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	handler := api.New(mgr, frontend.Dist(), config.BasePath())
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 15 * time.Second}

	listeners := make([]net.Listener, 0, 2)
	if ln, err := net.Listen("tcp", config.Addr()); err != nil {
		log.Printf("TCP 监听失败 %s: %v", config.Addr(), err)
	} else {
		log.Printf("HTTP 监听: http://%s", ln.Addr())
		listeners = append(listeners, ln)
	}
	if runtime.GOOS != "windows" {
		if ln, err := listenUnix(config.SockPath()); err != nil {
			log.Printf("Unix socket 监听失败 %s: %v", config.SockPath(), err)
		} else {
			log.Printf("Unix socket 监听: %s", config.SockPath())
			listeners = append(listeners, ln)
		}
	}
	if len(listeners) == 0 {
		log.Fatal("没有可用监听地址，退出")
	}

	for _, ln := range listeners {
		go func(ln net.Listener) {
			if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Printf("服务异常: %v", err)
			}
		}(ln)
	}

	go func() {
		if st := mgr.BinaryStatus(); !st.Ready {
			_ = mgr.DownloadBinary(ctx)
		}
	}()
	go mgr.StartAll(ctx)

	<-ctx.Done()
	log.Print("收到退出信号，正在清理")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
	mgr.StopAll()
	os.Remove(config.SockPath())
}

// listenUnix 重建 socket 文件并放开权限，便于统一网关以其他用户访问
func listenUnix(path string) (net.Listener, error) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o666); err != nil {
		ln.Close()
		return nil, err
	}
	return ln, nil
}
