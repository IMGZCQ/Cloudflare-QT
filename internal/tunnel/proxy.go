package tunnel

import (
	"crypto/tls"
	_ "embed"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
)

//go:embed bg.png
var bgPNG []byte

// localProxy 是插入 cloudflared 与本地服务之间的可控代理层。
// 暂停时返回维护页面，恢复时转发到真实服务，cloudflared 进程和域名不受影响。
type localProxy struct {
	reverseProxy *httputil.ReverseProxy
	paused       atomic.Bool
	server       *http.Server
}

// newLocalProxy 创建代理，target 为完整 URL（含路径和查询参数）。
func newLocalProxy(target string) *localProxy {
	targetURL, _ := url.Parse(target)

	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.URL.Path = targetURL.Path + req.URL.Path
			if targetURL.RawQuery != "" {
				if req.URL.RawQuery != "" {
					req.URL.RawQuery = targetURL.RawQuery + "&" + req.URL.RawQuery
				} else {
					req.URL.RawQuery = targetURL.RawQuery
				}
			}
			req.Host = targetURL.Host
		},
	}

	// HTTPS 本地服务多为自签证书，代理跳过校验
	if targetURL.Scheme == "https" {
		rp.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	return &localProxy{reverseProxy: rp}
}

// start 在本地随机端口监听，返回 cloudflared --url 需要的地址。
func (p *localProxy) start() (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	p.server = &http.Server{Handler: http.HandlerFunc(p.handle)}
	go p.server.Serve(ln)
	return fmt.Sprintf("http://127.0.0.1:%d", ln.Addr().(*net.TCPAddr).Port), nil
}

func (p *localProxy) stop() {
	if p.server != nil {
		p.server.Close()
		p.server = nil
	}
}

func (p *localProxy) setPaused(paused bool) { p.paused.Store(paused) }

func (p *localProxy) handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/bg.png" {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(bgPNG)
		return
	}
	if p.paused.Load() {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(maintenancePage))
		return
	}
	p.reverseProxy.ServeHTTP(w, r)
}

const maintenancePage = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
<title>服务维护中</title>
<style>
:root{
  --font-family-regular: -apple-system, BlinkMacSystemFont, "PingFang SC", "Hiragino Sans GB",
    "Microsoft YaHei", "Helvetica Neue", Helvetica, Arial, sans-serif;
  --ease-spring: cubic-bezier(.34, 1.56, .64, 1);
  --ease-out: cubic-bezier(.16, 1, .3, 1);
}
*{ box-sizing: border-box; margin: 0; padding: 0; }
html, body{ height: 100%; }
body{
  background: #000;
  color: #fff;
  font-family: var(--font-family-regular);
  -webkit-font-smoothing: antialiased;
  overflow: hidden;
}
.stage{
  position: relative;
  width: 100vw;
  height: 100vh;
  height: 100dvh;
  min-height: 520px;
  min-width: 320px;
  overflow: hidden;
  background: radial-gradient(circle at 50% 60%,
    rgba(9,24,38,.9) 0, rgba(3,8,14,.9) 40%, #000 72%);
}
.logo{
  position: absolute;
  left: 50%;
  top: clamp(48px, 11.5vh, 100px);
  z-index: 5;
  width: 65px; height: 65px;
  opacity: 0;
  transform: translateX(-50%) translateY(-80px) scale(.4);
}
.logo img{ display:block; width:100%; height:100%; object-fit:contain; }
.hero{
  position: absolute;
  left: 50%;
  top: clamp(136px, 20vh, 180px);
  z-index: 3;
  width: min(84vw, 651px);
  pointer-events: none;
  user-select: none;
  opacity: 0;
  transform: translateX(calc(-50% - 100px)) scale(.7);
}
.hero img{ display:block; width:100%; height:auto; object-fit:contain; }
.panel{
  position: absolute;
  left: 50%;
  top: clamp(386px, 66.6vh, 600px);
  z-index: 5;
  width: min(90vw, 651px);
  transform: translateX(-50%);
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
}
.title{
  font-size: 24px;
  font-weight: 600;
  line-height: 30px;
  letter-spacing: .48px;
  opacity: 0;
  transform: translateX(70px);
}
.desc{
  margin-top: 15px;
  font-size: 20px;
  font-weight: 300;
  line-height: 25px;
  letter-spacing: .4px;
  color: rgba(255,255,255,.72);
  opacity: 0;
  transform: translateY(40px);
}
.btn{
  position: relative;
  margin-top: 31px;
  display: inline-flex;
  height: 54px;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  border: 2px solid rgba(255,255,255,.2);
  background: transparent;
  padding: 6px 36px;
  color: #fff;
  font-family: inherit;
  font-size: 16px;
  font-weight: 600;
  line-height: normal;
  cursor: pointer;
  transition: border-color .28s var(--ease-out),
              background-color .28s var(--ease-out),
              transform .28s var(--ease-spring),
              box-shadow .28s var(--ease-out);
  opacity: 0;
  transform: translateY(30px) scale(.5);
  text-decoration: none;
}
.btn:hover{
  border-color: rgba(255,255,255,.48);
  background: rgba(255,255,255,.08);
  transform: translateY(-2px) scale(1.02);
  box-shadow: 0 10px 28px -10px rgba(120,200,255,.55);
}
.btn:active{ transform: translateY(0) scale(.96); }
.loaded .logo{ animation: fromTop 1.1s cubic-bezier(.22,.68,.35,1) .1s both; }
.loaded .hero{ animation: fromLeft 1.2s cubic-bezier(.22,.68,.35,1) .35s both; }
.loaded .title{ animation: fromRight 1s cubic-bezier(.22,.68,.35,1) .62s both; }
.loaded .desc{ animation: fromBelow 1s cubic-bezier(.22,.68,.35,1) .82s both; }
.loaded .btn{ animation: fromBelowPop 1.05s cubic-bezier(.22,.68,.35,1) 1s both; }
@keyframes fromTop{
  0%   { opacity:0; transform: translateX(-50%) translateY(-80px) scale(.4); }
  40%  { opacity:1; }
  65%  { transform: translateX(-50%) translateY(0) scale(1); }
  80%  { transform: translateX(-50%) translateY(-10px) scale(1.06); }
  92%  { transform: translateX(-50%) translateY(3px)  scale(.97); }
  100% { opacity:1; transform: translateX(-50%) translateY(0) scale(1); }
}
@keyframes fromLeft{
  0%   { opacity:0; transform: translateX(calc(-50% - 100px)) scale(.7); }
  40%  { opacity:1; }
  65%  { transform: translateX(-50%) scale(1); }
  80%  { transform: translateX(calc(-50% + 10px)) scale(1.04); }
  92%  { transform: translateX(calc(-50% - 3px)) scale(.98); }
  100% { opacity:1; transform: translateX(-50%) scale(1); }
}
@keyframes fromRight{
  0%   { opacity:0; transform: translateX(70px); }
  40%  { opacity:1; }
  65%  { transform: translateX(0); }
  80%  { transform: translateX(-8px); }
  92%  { transform: translateX(2px); }
  100% { opacity:1; transform: translateX(0); }
}
@keyframes fromBelow{
  0%   { opacity:0; transform: translateY(40px); }
  40%  { opacity:1; }
  65%  { transform: translateY(0); }
  80%  { transform: translateY(-6px); }
  92%  { transform: translateY(2px); }
  100% { opacity:1; transform: translateY(0); }
}
@keyframes fromBelowPop{
  0%   { opacity:0; transform: translateY(30px) scale(.5); }
  40%  { opacity:1; }
  65%  { transform: translateY(0) scale(1); }
  80%  { transform: translateY(-8px) scale(1.06); }
  92%  { transform: translateY(2px) scale(.97); }
  100% { opacity:1; transform: translateY(0) scale(1); }
}
@media (max-width: 767px), (max-height: 640px){
  .logo{ top: 24px; width: 52px; height: 52px; }
  .hero{ width: min(112vw, 520px); }
  .panel{ top: auto; bottom: max(42px, env(safe-area-inset-bottom)); }
  .title{ font-size: 20px; }
  .desc{ font-size: 15px; }
  .btn{ margin-top: 24px; height: 46px; padding: 6px 28px; }
}
@media (prefers-reduced-motion: reduce){
  .loaded .logo{ animation: fromTop 1.1s cubic-bezier(.22,.68,.35,1) .1s both !important; }
  .loaded .hero{ animation: fromLeft 1.2s cubic-bezier(.22,.68,.35,1) .35s both !important; }
  .loaded .title{ animation: fromRight 1s cubic-bezier(.22,.68,.35,1) .62s both !important; }
  .loaded .desc{ animation: fromBelow 1s cubic-bezier(.22,.68,.35,1) .82s both !important; }
  .loaded .btn{ animation: fromBelowPop 1.05s cubic-bezier(.22,.68,.35,1) 1s both !important; }
}
</style>
</head>
<body class="loaded">
<main class="stage">
  <div class="logo">
    <img src="data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iNjUiIGhlaWdodD0iNjUiIHZpZXdCb3g9IjAgMCA2NSA2NSIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZD0iTTU2LjY1NzYgOC42Njc5N0M1Ni42NTc2IDguNjY3OTcgNTkuNjIyMyAxNC41MDUxIDU4LjI2NDEgMTcuOTA1NkM1Ny41Njg4IDE5LjY0NjMgNTUuNTUzMSAyMS43MjExIDUzLjM0NDEgMjEuNzIxMUgyNC40MDc0QzIwLjI0ODMgMjEuNzIxMSAxNi44NzY3IDI1LjA5MjcgMTYuODc2NyAyOS4yNTE4VjUwLjQzOEMxNi44NzY3IDUzLjU5ODkgMTkuNDM5MSA1Ni4xNjEzIDIyLjYgNTYuMTYxM0gzMC40NzY5QzMyLjUyODcgNTYuMTYxMyAzNC4xOTIgNTQuNDk4IDM0LjE5MiA1Mi40NDYyVjQ3LjU2ODRIMzkuNjI3MkM0MS42NzkgNDcuNTY4NCA0My4zNDIzIDQ1LjkwNTEgNDMuMzQyMyA0My44NTMzVjM5LjU5MzlDNDguMDA4IDM5LjU5MzkgNTIuNDA4NiAzNy40MjUxIDU1LjI1MDggMzMuNzI1TDU1LjUyNDYgMzMuMzY4NUg0MC44MzIxQzM4Ljc4MDMgMzMuMzY4NSAzNy4xMTY5IDM1LjAzMTggMzcuMTE2OSAzNy4wODM2VjQwLjUzOThDMzcuMTE2OSA0MC45ODM1IDM2Ljc1NzMgNDEuMzQzMSAzNi4zMTM3IDQxLjM0MzFIMzEuNjgxOEMyOS42MyA0MS4zNDMxIDI3Ljk2NjcgNDMuMDA2NCAyNy45NjY3IDQ1LjA1ODJWNDguNDkxMkMyNy45NjY3IDQ5LjI4OTEgMjcuMzE5OCA0OS45MzYgMjYuNTIxOSA0OS45MzZIMjQuNTQ2OEMyMy43NDg5IDQ5LjkzNiAyMy4yNDE1IDQ5LjI4OTEgMjMuMjQxNSA0OC40OTEyVjI5LjQ4MjdDMjMuMjQxNSAyOC43NjE4IDIzLjgyNTkgMjguMTc3MyAyNC41NDY4IDI4LjE3NzNINTUuMzUyM0M1OS41MTEzIDI4LjE3NzMgNjIuODgyOSAyNC41NzQ5IDYyLjg4MjkgMjAuNDE1OEM2Mi44ODI5IDE2LjYzNjYgNjEuMzE2NSAxMy4wMjY0IDU4LjU1NjcgMTAuNDQ0Nkw1Ni42NTc2IDguNjY3OTdaIiBmaWxsPSJ3aGl0ZSIvPgo8cGF0aCBkPSJNNi43ODUxOCAxNy45MzQ0QzUuNDI2OTcgMTQuNTM0IDguMzkxNzIgOC42OTY4NCA4LjM5MTcyIDguNjk2ODRMNi40OTI1OSAxMC40NzM0QzMuNzMyNzQgMTMuMDU1MiAyLjE2NjM4IDE2LjY2NTQgMi4xNjYzOCAyMC40NDQ3QzIuMTY2MzggMjQuNjAzNyA3LjI4MzM2IDI4LjE3NzMgMTEuNDQyNCAyOC4xNzczVjI4LjE0NzdDMTEuNDQyNCAyNC45OTM4IDEyLjQ0MDEgMjMuMzM2MSAxMy45MjY5IDIxLjY2OEM5LjAzMTExIDIxLjI2NzIgNy4zOTg3OSAxOS40NzA3IDYuNzg1MTggMTcuOTM0NFoiIGZpbGw9IndoaXRlIi8+Cjwvc3ZnPgo=" alt="logo">
  </div>
  <div class="hero">
    <img src="/bg.png" alt="background">
  </div>
  <section class="panel">
    <h1 class="title">服务维护中</h1>
    <p class="desc">隧道已暂停，请联系管理员！</p>
    <a class="btn" href="javascript:location.reload()">刷新页面</a>
  </section>
</main>
</body>
</html>`
