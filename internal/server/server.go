package server

import (
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strings"

	"github.com/Gordonynh/ctfile-down/internal/ctfile"
)

//go:embed web
var webFS embed.FS

// Server serves the embedded web UI and the JSON API.
type Server struct {
	mgr *Manager
	mux *http.ServeMux
}

// New builds a Server writing downloads to saveDir.
func New(saveDir string) *Server {
	s := &Server{mgr: NewManager(saveDir)}
	s.routes()
	return s
}

func (s *Server) routes() {
	mux := http.NewServeMux()

	sub, _ := fs.Sub(webFS, "web")
	mux.Handle("/", http.FileServer(http.FS(sub)))

	mux.HandleFunc("/api/resolve", s.handleResolve)
	mux.HandleFunc("/api/download", s.handleDownload)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/tasks", s.handleTasks)
	mux.HandleFunc("/api/tasks/", s.handleTask)
	s.mux = mux
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// ListenAndServe starts the HTTP server on addr.
func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s)
}

func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		URL      string `json:"url"`
		Passcode string `json:"passcode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad request")
		return
	}
	link, err := ctfile.ParseLink(req.URL)
	if err != nil {
		writeCode(w, http.StatusBadRequest, "bad_link", err.Error())
		return
	}
	if req.Passcode != "" {
		link.Passcode = req.Passcode
	}
	client := ctfile.New(link)
	info, err := client.Resolve()
	if err != nil {
		if errors.Is(err, ctfile.ErrPasscodeRequired) {
			writeCode(w, http.StatusUnauthorized, "need_passcode", "该链接需要提取码，请填写后重试")
			return
		}
		writeCode(w, http.StatusBadGateway, "resolve_failed", err.Error())
		return
	}
	t, err := client.GetDownloadURL(info)
	if err != nil {
		writeCode(w, http.StatusBadGateway, "resolve_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"file_name":    info.FileName,
		"file_size":    t.FileSize,
		"size_display": info.SizeDisplay,
	})
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		URL      string `json:"url"`
		Passcode string `json:"passcode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad request")
		return
	}
	task, err := s.mgr.Start(req.URL, req.Passcode)
	if err != nil {
		if errors.Is(err, ctfile.ErrPasscodeRequired) {
			writeCode(w, http.StatusUnauthorized, "need_passcode", "该链接需要提取码，请填写后重试")
			return
		}
		writeCode(w, http.StatusBadRequest, "download_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, task)
}

// handleConfig 暴露只读运行时配置，供前端展示保存目录。
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"save_dir": s.mgr.SaveDir(),
	})
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, s.mgr.List())
}

func (s *Server) handleTask(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	if id == "" || strings.Contains(id, "/") {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		t := s.mgr.Get(id)
		if t == nil {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		writeJSON(w, http.StatusOK, t)
	case http.MethodPost: // cancel
		s.mgr.Cancel(id)
		writeJSON(w, http.StatusOK, map[string]string{"status": "canceled"})
	case http.MethodDelete: // remove
		s.mgr.Remove(id)
		writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// writeCode 返回带机器可读错误码的响应，便于前端区分处理（如需要提取码）。
func writeCode(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"code": code, "error": msg})
}
