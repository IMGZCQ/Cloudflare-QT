// Package favicon 负责抓取隧道本地地址的 favicon 并缓存到磁盘。
// 通过记录目标地址 hash 来避免重复抓取，地址变更时自动重新获取。
// 抓取策略：解析 HTML 中全部 <link rel="icon">，按 sizes 优选 32x32；
// 多候选并发下载，取第一个成功者；抓取失败也记录 hash，避免反复重试。
package favicon

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"cfquicktunnel/internal/config"
)

const (
	fetchTimeout   = 15 * time.Second
	maxConcurrency = 4
	userAgent      = "CF-QuickTunnel-Favicon/1.0"
)

// 匹配 <link> 标签整体，便于再提取内部属性
var linkPattern = regexp.MustCompile(`(?is)<link\b[^>]*>`)

// attrPattern 用于从单个标签里提取属性值
func attrPattern(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?is)\b` + regexp.QuoteMeta(name) + `\s*=\s*["']([^"']+)["']`)
}

var (
	relAttr   = attrPattern("rel")
	hrefAttr  = attrPattern("href")
	sizesAttr = attrPattern("sizes")
	typeAttr  = attrPattern("type")
)

// meta 持久化文件：记录每条隧道上次抓取的目标地址 hash 与保存的文件名
// File 为空串表示已抓取但失败（避免重启后反复请求死站）
type meta struct {
	TargetHash string `json:"targetHash"`
	File       string `json:"file"`
}

// Cache 管理 favicon 磁盘缓存
type Cache struct {
	mu   sync.Mutex
	dir  string
	meta map[string]meta
	// 防止同一隧道并发 Ensure
	inflight map[string]struct{}
}

// New 创建缓存管理器，加载已有元数据
func New() (*Cache, error) {
	dir := filepath.Join(config.DataDir(), "favicon")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	c := &Cache{dir: dir, meta: make(map[string]meta), inflight: make(map[string]struct{})}
	raw, err := os.ReadFile(c.metaPath())
	if err == nil {
		_ = json.Unmarshal(raw, &c.meta)
	}
	return c, nil
}

func (c *Cache) metaPath() string { return filepath.Join(c.dir, "meta.json") }

// FileInfo 返回某条隧道已缓存的图标文件路径与修改时间；不存在时返回空串
func (c *Cache) FileInfo(tunnelID string) (string, int64) {
	c.mu.Lock()
	m, ok := c.meta[tunnelID]
	c.mu.Unlock()
	if !ok || m.File == "" {
		return "", 0
	}
	p := filepath.Join(c.dir, m.File)
	st, err := os.Stat(p)
	if err != nil {
		return "", 0
	}
	return p, st.ModTime().Unix()
}

// Ensure 如果目标地址有变化（或从未抓取过）则异步抓取 favicon
// 注意：无论成功失败都会写入 meta，避免每次启动反复请求失败的站点
func (c *Cache) Ensure(tunnelID, target string) {
	target = strings.TrimSpace(target)
	if target == "" {
		return
	}
	hash := hashTarget(target)

	c.mu.Lock()
	m, ok := c.meta[tunnelID]
	if ok && m.TargetHash == hash {
		c.mu.Unlock()
		return
	}
	if _, running := c.inflight[tunnelID]; running {
		c.mu.Unlock()
		return
	}
	c.inflight[tunnelID] = struct{}{}
	c.mu.Unlock()

	go func() {
		defer func() {
			c.mu.Lock()
			delete(c.inflight, tunnelID)
			c.mu.Unlock()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()

		data, ext, err := fetchBestIcon(ctx, target)
		c.mu.Lock()
		defer c.mu.Unlock()

		if err != nil || len(data) == 0 {
			// 失败也记录 hash，避免下次启动再次抓取同一地址
			c.meta[tunnelID] = meta{TargetHash: hash, File: ""}
			_ = c.flushLocked()
			return
		}

		name := tunnelID + ext
		if err := os.WriteFile(filepath.Join(c.dir, name), data, 0o644); err != nil {
			return
		}
		// 清理旧文件
		if old, ok := c.meta[tunnelID]; ok && old.File != "" && old.File != name {
			_ = os.Remove(filepath.Join(c.dir, old.File))
		}
		c.meta[tunnelID] = meta{TargetHash: hash, File: name}
		_ = c.flushLocked()
	}()
}

// candidate 待下载的图标候选
type candidate struct {
	url   string
	score int // 越小越优先
}

// fetchBestIcon 下载目标地址 HTML，解析全部 icon 候选，按优先级并发下载，返回第一个成功者
func fetchBestIcon(ctx context.Context, target string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{Timeout: fetchTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	base := resp.Request.URL
	cands := collectCandidates(string(body), base)
	if len(cands) == 0 {
		// 兜底
		cands = []candidate{{url: base.Scheme + "://" + base.Host + "/favicon.ico", score: 100}}
	}
	return fetchConcurrently(ctx, client, cands)
}

// collectCandidates 从 HTML 中提取所有 <link rel="icon"> 候选，并按分数排序
func collectCandidates(html string, base *url.URL) []candidate {
	tags := linkPattern.FindAllString(html, 32)
	var out []candidate
	seen := make(map[string]bool)

	for _, tag := range tags {
		rel := strings.ToLower(firstSubmatch(relAttr, tag))
		if !strings.Contains(rel, "icon") {
			continue
		}
		// 排除 safari mask-icon（单色 SVG，<img> 显示为全黑）
		if strings.Contains(rel, "mask-icon") {
			continue
		}
		href := firstSubmatch(hrefAttr, tag)
		if href == "" {
			continue
		}
		if strings.HasPrefix(href, "data:") {
			continue // 内联 data URI 暂不支持，留待后续
		}
		ref, err := url.Parse(href)
		if err != nil {
			continue
		}
		full := base.ResolveReference(ref).String()
		if seen[full] {
			continue
		}
		seen[full] = true

		sizes := firstSubmatch(sizesAttr, tag)
		typ := strings.ToLower(firstSubmatch(typeAttr, tag))
		score := scoreCandidate(sizes, typ, full)
		out = append(out, candidate{url: full, score: score})
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].score < out[j].score })
	return out
}

func firstSubmatch(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// scoreCandidate 给候选打分：32x32 PNG 最优，其次 16x16，ICO 兜底
// sizes 可能为 "32x32"、"16x16 32x32"、"any" 等
func scoreCandidate(sizes, typ, rawURL string) int {
	score := 50 // 默认无 sizes

	// 解析 sizes（取最大边长作为参考）
	if sizes != "" && sizes != "any" {
		maxSide := 0
		for _, part := range strings.Fields(sizes) {
			kv := strings.Split(part, "x")
			if len(kv) != 2 {
				continue
			}
			n, _ := strconv.Atoi(kv[0])
			if n > maxSide {
				maxSide = n
			}
		}
		switch {
		case maxSide == 32:
			score = 10
		case maxSide == 16:
			score = 20
		case maxSide > 0 && maxSide <= 64:
			score = 30
		case maxSide <= 192:
			score = 40
		default:
			score = 45
		}
	}

	// 文件格式微调
	isIco := strings.Contains(typ, "icon") || strings.HasSuffix(strings.ToLower(rawURL), ".ico")
	isSvg := strings.Contains(typ, "svg") || strings.HasSuffix(strings.ToLower(rawURL), ".svg")
	if isIco && score > 30 {
		score = 30 // ico 通常包含多尺寸，作为合理兜底
	}
	if isSvg {
		score += 40 // SVG 放后面，部分场景 <img> 不显示
	}
	return score
}

// fetchConcurrently 并发下载候选，返回第一个成功的结果
func fetchConcurrently(ctx context.Context, client *http.Client, cands []candidate) ([]byte, string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type result struct {
		data []byte
		ext  string
	}
	ch := make(chan result, 1)

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrency)

	launch := func(c candidate) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			data, ext, err := downloadOne(ctx, client, c.url)
			if err != nil {
				return
			}
			select {
			case ch <- result{data: data, ext: ext}:
				cancel() // 通知其他 goroutine 停止
			case <-ctx.Done():
			}
		}()
	}

	// 先发前 maxConcurrency 个
	for i := 0; i < len(cands) && i < maxConcurrency; i++ {
		launch(cands[i])
	}
	// 其余的在前序失败时补充（简化：全部并发，由分数顺序自然竞争）
	for i := maxConcurrency; i < len(cands); i++ {
		launch(cands[i])
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	res, ok := <-ch
	if !ok {
		return nil, "", fmt.Errorf("全部候选下载失败")
	}
	return res.data, res.ext, nil
}

// downloadOne 下载单个图标并推断扩展名
func downloadOne(ctx context.Context, client *http.Client, rawURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil || len(data) == 0 {
		return nil, "", fmt.Errorf("空文件")
	}
	// 防止把 HTML 错误页当成图标
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(ct, "text/html") {
		return nil, "", fmt.Errorf("HTML 响应")
	}
	return data, guessExt(ct, rawURL), nil
}

// guessExt 根据 Content-Type 或 URL 后缀推断文件扩展名
func guessExt(contentType, rawURL string) string {
	ct := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch ct {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	case "image/x-icon", "image/vnd.microsoft.icon":
		return ".ico"
	}
	if u, err := url.Parse(rawURL); err == nil {
		ext := strings.ToLower(filepath.Ext(u.Path))
		switch ext {
		case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico":
			if ext == ".jpeg" {
				return ".jpg"
			}
			return ext
		}
	}
	return ".ico"
}

// flushLocked 持久化元数据（调用方需已持锁）
func (c *Cache) flushLocked() error {
	raw, err := json.MarshalIndent(c.meta, "", "  ")
	if err != nil {
		return err
	}
	tmp := c.metaPath() + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, c.metaPath())
}

// hashTarget 计算目标地址的短 hash
func hashTarget(target string) string {
	sum := sha1.Sum([]byte(target))
	return hex.EncodeToString(sum[:8])
}
