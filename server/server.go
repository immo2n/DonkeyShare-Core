package server

import (
	"filelink/config"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

type Server struct {
	cfg *config.Config
}

func NewServer(cfg *config.Config) (*Server, error) {
	return &Server{
		cfg: cfg,
	}, nil
}

func (s *Server) LogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if s.cfg.ShowLogs {
			log.Printf("%s - %s %s (%s)", r.RemoteAddr, r.Method, r.URL.Path, time.Since(start))
		}
	})
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	api := r.URL.Query().Get("api")
	if s.cfg.ReadOnly {
		switch api {
		case "upload", "upload-chunk", "create-dir", "delete", "move", "copy":
			RespondJSON(w, http.StatusForbidden, map[string]interface{}{
				"ok":    false,
				"error": "Write operations are disabled in read-only mode",
			})
			return
		}
	}
	switch api {
	case "list":
		s.HandleList(w, r)
	case "upload":
		s.HandleUpload(w, r)
	case "upload-chunk":
		s.HandleUploadChunk(w, r)
	case "create-dir":
		s.HandleCreateDir(w, r)
	case "delete":
		s.HandleDelete(w, r)
	case "download":
		s.HandleDownload(w, r)
	case "move":
		s.HandleMove(w, r)
	case "copy":
		s.HandleCopy(w, r)
	default:
		s.HandleFileOrUI(w, r)
	}
}

func SafeJoin(root, rel string) (string, error) {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		realRoot = filepath.Clean(root)
	}
	rel = strings.ReplaceAll(rel, "\\", "/")
	candidate := filepath.Clean(filepath.Join(realRoot, rel))
	realCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		realCandidate = candidate
	}
	if realCandidate == realRoot {
		return realCandidate, nil
	}
	if !strings.HasPrefix(realCandidate, realRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("path escape detected: %s is outside %s", realCandidate, realRoot)
	}
	return realCandidate, nil
}

func encodePath(p string) string {
	p = strings.Trim(p, "/")
	if p == "" {
		return ""
	}
	segs := strings.Split(p, "/")
	for i, seg := range segs {
		segs[i] = url.PathEscape(seg)
	}
	return strings.Join(segs, "/")
}
