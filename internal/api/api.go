// Package api 提供隧道管理的 HTTP 接口与静态资源服务。
package api

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"cfquicktunnel/internal/store"
	"cfquicktunnel/internal/tunnel"
)

// Server 聚合 REST 接口与前端静态资源
type Server struct {
	mgr      *tunnel.Manager
	mux      *http.ServeMux
	basePath string
}

// New 创建 HTTP handler。dist 为前端构建产物（可为 nil，仅提供 API）。
func New(mgr *tunnel.Manager, dist fs.FS, basePath string) *Server {
	s := &Server{mgr: mgr, mux: http.NewServeMux(), basePath: strings.TrimSuffix(basePath, "/")}
	s.routes()
	if dist != nil {
		s.mux.Handle("/", spaHandler(dist))
	}
	return s
}

// APIHandler 仅返回 /api 路由，供 Wails 桌面模式作为静态资源中间件使用
func APIHandler(mgr *tunnel.Manager) http.Handler {
	s := &Server{mgr: mgr, mux: http.NewServeMux()}
	s.routes()
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/tunnels", s.handleList)
	s.mux.HandleFunc("POST /api/tunnels", s.handleCreate)
	s.mux.HandleFunc("PUT /api/tunnels/{id}", s.handleUpdate)
	s.mux.HandleFunc("DELETE /api/tunnels/{id}", s.handleDelete)
	s.mux.HandleFunc("POST /api/tunnels/{id}/start", s.handleStart)
	s.mux.HandleFunc("POST /api/tunnels/{id}/stop", s.handleStop)
	s.mux.HandleFunc("POST /api/tunnels/{id}/pause", s.handlePause)
	s.mux.HandleFunc("POST /api/tunnels/{id}/resume", s.handleResume)
	s.mux.HandleFunc("GET /api/tunnels/{id}/logs", s.handleLogs)
	s.mux.HandleFunc("GET /api/binary", s.handleBinary)
	s.mux.HandleFunc("POST /api/binary/download", s.handleBinaryDownload)
}

// ServeHTTP 处理统一网关前缀后分发请求
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.basePath != "" && strings.HasPrefix(r.URL.Path, s.basePath) {
		if r.URL.Path == s.basePath {
			http.Redirect(w, r, s.basePath+"/", http.StatusMovedPermanently)
			return
		}
		trimmed := strings.TrimPrefix(r.URL.Path, s.basePath)
		if trimmed == "" {
			trimmed = "/"
		}
		r2 := r.Clone(r.Context())
		r2.URL.Path = trimmed
		r = r2
	}
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	payload := map[string]any{
		"items":  s.mgr.List(),
		"binary": s.mgr.BinaryStatus(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	// 用内容哈希作为 ETag，让前端在无变化时跳过响应体反序列化与渲染
	sum := sha1.Sum(raw)
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache")
	if match := r.Header.Get("If-None-Match"); match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Target string `json:"target"`
		store.Tunnel
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("请求体格式错误"))
		return
	}
	cfg := body.Tunnel
	if body.Target != "" {
		parsed, err := store.ParseTarget(body.Target)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		parsed.Name = cfg.Name
		parsed.AutoStart = cfg.AutoStart
		parsed.EdgeIPVersion = cfg.EdgeIPVersion
		// 新建时 Paused 默认为 false，不接收前端提交
		cfg = parsed
	} else {
		// 新建时强制 Paused=false，避免继承无关值
		cfg.Paused = false
	}
	item, err := s.mgr.Create(cfg)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Target string `json:"target"`
		store.Tunnel
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("请求体格式错误"))
		return
	}
	cfg := body.Tunnel
	if body.Target != "" {
		parsed, err := store.ParseTarget(body.Target)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		parsed.Name = cfg.Name
		parsed.AutoStart = cfg.AutoStart
		parsed.EdgeIPVersion = cfg.EdgeIPVersion
		cfg = parsed
	}
	// Paused 由 Manager.Update 从旧值继承，这里不接收前端提交，避免被误覆盖
	item, err := s.mgr.Update(r.PathValue("id"), cfg)
	if err != nil {
		writeError(w, statusOf(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.mgr.Delete(r.PathValue("id")); err != nil {
		writeError(w, statusOf(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.mgr.Start(r.Context(), id); err != nil {
		writeError(w, statusOf(err), err)
		return
	}
	item, err := s.mgr.Get(id)
	if err != nil {
		writeError(w, statusOf(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.mgr.Stop(id); err != nil && !errors.Is(err, tunnel.ErrNotRunning) {
		writeError(w, statusOf(err), err)
		return
	}
	item, err := s.mgr.Get(id)
	if err != nil {
		writeError(w, statusOf(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handlePause(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.mgr.Pause(id); err != nil {
		writeError(w, statusOf(err), err)
		return
	}
	item, err := s.mgr.Get(id)
	if err != nil {
		writeError(w, statusOf(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleResume(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.mgr.Resume(id); err != nil {
		writeError(w, statusOf(err), err)
		return
	}
	item, err := s.mgr.Get(id)
	if err != nil {
		writeError(w, statusOf(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	from, _ := strconv.ParseUint(r.URL.Query().Get("from"), 10, 64)
	logs, total, err := s.mgr.Logs(r.PathValue("id"), from)
	if err != nil {
		writeError(w, statusOf(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": logs, "total": total})
}

func (s *Server) handleBinary(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.mgr.BinaryStatus())
}

func (s *Server) handleBinaryDownload(w http.ResponseWriter, r *http.Request) {
	if err := s.mgr.DownloadBinary(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, s.mgr.BinaryStatus())
}

func statusOf(err error) int {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, tunnel.ErrAlreadyRunning), errors.Is(err, tunnel.ErrNotRunning), errors.Is(err, tunnel.ErrNotPaused):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]any{"error": err.Error()})
}

// spaHandler 提供前端静态资源，未命中的路径回退到 index.html
func spaHandler(dist fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		if _, err := fs.Stat(dist, name); err != nil {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}
