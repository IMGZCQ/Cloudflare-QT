// Package tunnel 管理 cloudflared 可执行文件与临时隧道进程。
package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"cfquicktunnel/internal/config"
)

// githubURL cloudflared 官方 latest 下载地址模板
const githubURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/%s"

// defaultMirrors 下载源前缀，按优先级排列（实测国内直连速度），
// 空串表示 GitHub 官方直连，放在最后兜底。
// 可用 CFQT_MIRRORS 覆盖：逗号分隔的前缀列表，填 off/direct 表示只用官方源。
var defaultMirrors = []string{
	"https://gh.catmak.name/",
	"https://ghproxy.imciel.com/",
	"https://gh.monlor.com/",
	"https://github.tbap.top/",
	"https://ghfast.top/",
	"https://cdn.gh-proxy.com/",
	"",
}

// minBinarySize cloudflared 实际体积在 30MB 以上，
// 用来识别加速源返回的错误页面等异常响应
const minBinarySize = 5 << 20

// srcTimeout 单个下载源的整体超时
const srcTimeout = 10 * time.Minute

// downloadClient 缩短连接与响应头超时，让失效的源尽快失败以便切换下一个
var downloadClient = &http.Client{
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
	},
}

// BinaryStatus cloudflared 可执行文件状态
type BinaryStatus struct {
	Path        string `json:"path"`
	Ready       bool   `json:"ready"`
	Version     string `json:"version"`
	Downloading bool   `json:"downloading"`
	Progress    int    `json:"progress"`
	Message     string `json:"message"`
}

type binary struct {
	mu          sync.Mutex
	downloading bool
	progress    int
	message     string
	version     string
}

// assetName 按当前平台推导 release 资产名
func assetName() (string, error) {
	switch runtime.GOOS {
	case "linux":
		switch runtime.GOARCH {
		case "amd64", "arm64", "arm", "386":
			return "cloudflared-linux-" + runtime.GOARCH, nil
		}
	case "windows":
		switch runtime.GOARCH {
		case "amd64", "386":
			return "cloudflared-windows-" + runtime.GOARCH + ".exe", nil
		}
	}
	return "", fmt.Errorf("暂不支持自动下载 %s/%s，请手动放置 cloudflared 到 %s",
		runtime.GOOS, runtime.GOARCH, config.CloudflaredPath())
}

// Status 返回可执行文件状态
func (b *binary) Status() BinaryStatus {
	b.mu.Lock()
	defer b.mu.Unlock()

	path := config.CloudflaredPath()
	st := BinaryStatus{
		Path:        path,
		Downloading: b.downloading,
		Progress:    b.progress,
		Message:     b.message,
		Version:     b.version,
	}
	if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Size() > 0 {
		st.Ready = true
		if st.Version == "" {
			st.Version = probeVersion(path)
			b.version = st.Version
		}
	}
	return st
}

// Ensure 确保可执行文件就绪，缺失时自动下载
func (b *binary) Ensure(ctx context.Context) error {
	if st := b.Status(); st.Ready {
		return nil
	}
	return b.Download(ctx)
}

// Download 下载 cloudflared 到项目 bin 目录
func (b *binary) Download(ctx context.Context) error {
	asset, err := assetName()
	if err != nil {
		return err
	}

	b.mu.Lock()
	if b.downloading {
		b.mu.Unlock()
		return fmt.Errorf("正在下载中，请稍候")
	}
	b.downloading = true
	b.progress = 0
	b.message = "开始下载 " + asset
	b.mu.Unlock()

	err = b.doDownload(ctx, asset)

	b.mu.Lock()
	b.downloading = false
	if err != nil {
		b.message = "下载失败: " + err.Error()
	} else {
		b.progress = 100
		// message 已由 doDownload 写入命中的下载源
		b.version = probeVersion(config.CloudflaredPath())
	}
	b.mu.Unlock()
	return err
}

// mirrors 返回按优先级排列的下载源前缀
func mirrors() []string {
	v := strings.TrimSpace(os.Getenv(config.EnvMirrors))
	if v == "" {
		return defaultMirrors
	}
	if v == "off" || v == "direct" {
		return []string{""}
	}
	var list []string
	for _, s := range strings.Split(v, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !strings.HasSuffix(s, "/") {
			s += "/"
		}
		list = append(list, s)
	}
	// 自定义列表末尾仍保留官方源兜底
	return append(list, "")
}

// sourceName 提取源的展示名
func sourceName(prefix string) string {
	if prefix == "" {
		return "GitHub 官方"
	}
	if u, err := url.Parse(prefix); err == nil && u.Host != "" {
		return u.Host
	}
	return prefix
}

// doDownload 按优先级逐个下载源尝试，任一源成功即返回
func (b *binary) doDownload(ctx context.Context, asset string) error {
	if err := config.EnsureDirs(); err != nil {
		return err
	}

	origin := fmt.Sprintf(githubURL, asset)
	list := mirrors()
	var lastErr error
	for i, prefix := range list {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := sourceName(prefix)
		b.mu.Lock()
		b.progress = 0
		b.message = fmt.Sprintf("正在从 %s 下载 %s（源 %d/%d）", name, asset, i+1, len(list))
		b.mu.Unlock()

		err := b.fetch(ctx, prefix+origin)
		if err == nil {
			b.mu.Lock()
			b.message = "已从 " + name + " 下载完成"
			b.mu.Unlock()
			return nil
		}
		lastErr = fmt.Errorf("%s: %w", name, err)
	}
	return fmt.Errorf("所有下载源均失败，最后一个错误 %w", lastErr)
}

// fetch 从单个地址下载并落到 cloudflared 目标路径
func (b *binary) fetch(ctx context.Context, url string) error {
	ctx, cancel := context.WithTimeout(ctx, srcTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := downloadClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", resp.Status)
	}
	if resp.ContentLength > 0 && resp.ContentLength < minBinarySize {
		return fmt.Errorf("响应体积异常（%d 字节），疑似错误页面", resp.ContentLength)
	}

	dst := config.CloudflaredPath()
	tmp := dst + ".download"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}

	total := resp.ContentLength
	written, err := io.Copy(f, &progressReader{r: resp.Body, total: total, onProgress: func(p int) {
		b.mu.Lock()
		b.progress = p
		b.mu.Unlock()
	}})
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil && written < minBinarySize {
		err = fmt.Errorf("下载内容不完整（%d 字节）", written)
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}

	// Windows 下无法覆盖正在运行的文件，先尝试删除旧文件
	os.Remove(dst)
	if err := os.Rename(tmp, filepath.Clean(dst)); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Chmod(dst, 0o755)
}

type progressReader struct {
	r          io.Reader
	total      int64
	read       int64
	last       int
	onProgress func(int)
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.r.Read(buf)
	if n > 0 && p.total > 0 {
		p.read += int64(n)
		pct := int(p.read * 100 / p.total)
		if pct > 100 {
			pct = 100
		}
		if pct != p.last {
			p.last = pct
			p.onProgress(pct)
		}
	}
	return n, err
}

// probeVersion 执行 cloudflared --version 读取版本号
func probeVersion(path string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--version")
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
}
