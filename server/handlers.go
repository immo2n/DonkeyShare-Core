package server

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type FileItem struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Size   *int64 `json:"size"`
	Mtime  int64  `json:"mtime"`
	Hidden bool   `json:"hidden"`
}

type LimitsJSON struct {
	UploadMaxFilesize string `json:"upload_max_filesize"`
	PostMaxSize       string `json:"post_max_size"`
}

func RespondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) HandleList(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	showHidden := r.URL.Query().Get("hidden") == "1"

	relClean := strings.ReplaceAll(rel, "\\", "/")
	relClean = path.Clean("/" + relClean)
	relClean = strings.TrimPrefix(relClean, "/")
	if relClean == "." {
		relClean = ""
	}

	if relClean == ".Trash-FileLink" {
		trashPath := filepath.Join(s.cfg.SharedRoot, ".Trash-FileLink")
		if _, err := os.Stat(trashPath); os.IsNotExist(err) {
			_ = os.MkdirAll(trashPath, 0775)
		}
	}

	dirPath, err := SafeJoin(s.cfg.SharedRoot, relClean)
	if err != nil {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}

	info, err := os.Stat(dirPath)
	if err != nil || !info.IsDir() {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": "Not a directory",
		})
		return
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok":    false,
			"error": "Failed to read directory",
		})
		return
	}

	var items []FileItem
	for _, entry := range entries {
		name := entry.Name()
		if name == "." || name == ".." {
			continue
		}

		if name == ".Trash-FileLink" && relClean == "" {
			continue
		}

		isHidden := len(name) > 0 && name[0] == '.'
		if !showHidden && isHidden {
			continue
		}

		full := filepath.Join(dirPath, name)
		fileInfo, err := os.Stat(full)
		if err != nil {
			continue
		}

		isDir := fileInfo.IsDir()
		isFile := fileInfo.Mode().IsRegular()

		var fileType string
		if isDir {
			fileType = "dir"
		} else if isFile {
			fileType = "file"
		} else {
			fileType = "other"
		}

		var size *int64
		if isFile {
			sz := fileInfo.Size()
			size = &sz
		}

		items = append(items, FileItem{
			Name:   name,
			Type:   fileType,
			Size:   size,
			Mtime:  fileInfo.ModTime().Unix(),
			Hidden: isHidden,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Type != items[j].Type {
			if items[i].Type == "dir" {
				return true
			}
			if items[j].Type == "dir" {
				return false
			}
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"ok":    true,
		"path":  relClean,
		"items": items,
	})
}

func (s *Server) HandleDownload(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	filePath, err := SafeJoin(s.cfg.SharedRoot, rel)
	if err != nil || filePath == "" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	filename := filepath.Base(filePath)

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, strings.ReplaceAll(filename, `"`, `\"`)))
	w.Header().Set("Content-Transfer-Encoding", "binary")
	w.Header().Set("Connection", "close")

	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "Failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	_, _ = io.Copy(w, file)
}

func (s *Server) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		RespondJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":    false,
			"error": "Method not allowed",
		})
		return
	}

	if s.cfg.MaxUpload > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUpload)
	}

	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		if err.Error() == "http: request body too large" {
			RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
				"ok":    false,
				"error": "File exceeds upload limit",
				"limits": LimitsJSON{
					UploadMaxFilesize: s.cfg.MaxSizeStr,
					PostMaxSize:       s.cfg.MaxSizeStr,
				},
			})
			return
		}
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": "Failed to parse multipart form: " + err.Error(),
		})
		return
	}

	pathVal := r.FormValue("path")
	relClean := strings.ReplaceAll(pathVal, "\\", "/")
	relClean = path.Clean("/" + relClean)
	relClean = strings.TrimPrefix(relClean, "/")
	if relClean == "." {
		relClean = ""
	}

	destDir, err := SafeJoin(s.cfg.SharedRoot, relClean)
	if err != nil {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": "Invalid upload path: " + err.Error(),
		})
		return
	}
	if err := os.MkdirAll(destDir, 0775); err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok":    false,
			"error": "Failed to create destination folder: " + err.Error(),
		})
		return
	}

	var fileHeaders []*multipart.FileHeader
	for key, headers := range r.MultipartForm.File {
		if key == "files[]" || key == "file" || key == "files" {
			fileHeaders = append(fileHeaders, headers...)
		}
	}

	if len(fileHeaders) == 0 {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": "No files uploaded",
		})
		return
	}

	type SavedInfo struct {
		Name    string `json:"name"`
		Bytes   int64  `json:"bytes"`
		SavedTo string `json:"saved_to"`
	}

	var saved []SavedInfo

	for _, fh := range fileHeaders {
		origName := fh.Filename
		base := filepath.Base(origName)

		var builder strings.Builder
		for _, r := range base {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' || r == ' ' {
				builder.WriteRune(r)
			} else {
				builder.WriteRune('_')
			}
		}
		base = strings.TrimSpace(builder.String())
		if base == "" || base == "." || base == ".." {
			base = "upload.bin"
		}

		targetPath := filepath.Join(destDir, base)
		if _, err := os.Stat(targetPath); err == nil {
			ext := filepath.Ext(base)
			nameWithoutExt := strings.TrimSuffix(base, ext)
			timestamp := time.Now().Format("20060102_150405")
			targetPath = filepath.Join(destDir, fmt.Sprintf("%s_%s%s", nameWithoutExt, timestamp, ext))

			n := 1
			for {
				if _, err := os.Stat(targetPath); err != nil {
					break
				}
				targetPath = filepath.Join(destDir, fmt.Sprintf("%s_%s_%d%s", nameWithoutExt, timestamp, n, ext))
				n++
				if n > 1000 {
					break
				}
			}
		}

		src, err := fh.Open()
		if err != nil {
			RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"ok":    false,
				"error": "Failed to open uploaded file header: " + err.Error(),
			})
			return
		}

		dst, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0664)
		if err != nil {
			src.Close()
			RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"ok":    false,
				"error": "Failed to create target file on disk: " + err.Error(),
			})
			return
		}

		written, err := io.Copy(dst, src)
		src.Close()
		dst.Close()

		if err != nil {
			_ = os.Remove(targetPath)
			RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"ok":    false,
				"error": "Failed to write file to disk: " + err.Error(),
			})
			return
		}

		saved = append(saved, SavedInfo{
			Name:    filepath.Base(targetPath),
			Bytes:   written,
			SavedTo: targetPath,
		})
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"dest_dir": destDir,
		"saved":    saved,
		"limits": LimitsJSON{
			UploadMaxFilesize: s.cfg.MaxSizeStr,
			PostMaxSize:       s.cfg.MaxSizeStr,
		},
	})
}

func (s *Server) HandleFileOrUI(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Path

	filePath, err := SafeJoin(s.cfg.SharedRoot, reqPath)
	if err == nil && filePath != "" {
		info, err := os.Stat(filePath)
		if err == nil && !info.IsDir() {
			if r.URL.Query().Get("dl") == "1" {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, strings.ReplaceAll(filepath.Base(filePath), `"`, `\"`)))
				w.Header().Set("Content-Transfer-Encoding", "binary")
				w.Header().Set("Connection", "close")
			}
			http.ServeFile(w, r, filePath)
			return
		}
	}

	http.NotFound(w, r)
}

func (s *Server) HandleUploadChunk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		RespondJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":    false,
			"error": "Method not allowed",
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 20<<20)

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": "Failed to parse multipart form: " + err.Error(),
		})
		return
	}

	uuid := r.FormValue("uuid")
	indexStr := r.FormValue("index")
	totalStr := r.FormValue("total")
	filename := r.FormValue("filename")

	if uuid == "" || indexStr == "" || totalStr == "" || filename == "" {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": "Missing chunk metadata (uuid, index, total, filename)",
		})
		return
	}

	index, err := strconv.Atoi(indexStr)
	total, err2 := strconv.Atoi(totalStr)
	if err != nil || err2 != nil || index < 0 || total <= 0 || index >= total {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": "Invalid chunk indexing parameters",
		})
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": "Missing file chunk payload",
		})
		return
	}
	defer file.Close()

	tempChunksDir := filepath.Join(os.TempDir(), "filelink-chunks", uuid)
	if err := os.MkdirAll(tempChunksDir, 0755); err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok":    false,
			"error": "Failed to create temporary chunks cache: " + err.Error(),
		})
		return
	}

	chunkPath := filepath.Join(tempChunksDir, fmt.Sprintf("%d.tmp", index))
	out, err := os.OpenFile(chunkPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok":    false,
			"error": "Failed to open temporary chunk: " + err.Error(),
		})
		return
	}

	_, err = io.Copy(out, file)
	out.Close()
	if err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok":    false,
			"error": "Failed to write temporary chunk: " + err.Error(),
		})
		return
	}

	completed := true
	for i := 0; i < total; i++ {
		p := filepath.Join(tempChunksDir, fmt.Sprintf("%d.tmp", i))
		if _, err := os.Stat(p); err != nil {
			completed = false
			break
		}
	}

	if !completed {
		RespondJSON(w, http.StatusOK, map[string]interface{}{
			"ok":        true,
			"completed": false,
		})
		return
	}

	pathVal := r.FormValue("path")
	relClean := strings.ReplaceAll(pathVal, "\\", "/")
	relClean = path.Clean("/" + relClean)
	relClean = strings.TrimPrefix(relClean, "/")
	if relClean == "." {
		relClean = ""
	}

	destDir, err := SafeJoin(s.cfg.SharedRoot, relClean)
	if err != nil {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": "Invalid upload path: " + err.Error(),
		})
		return
	}
	if err := os.MkdirAll(destDir, 0775); err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok":    false,
			"error": "Failed to create destination directory: " + err.Error(),
		})
		return
	}

	base := filepath.Base(filename)
	var builder strings.Builder
	for _, r := range base {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' || r == ' ' {
			builder.WriteRune(r)
		} else {
			builder.WriteRune('_')
		}
	}
	base = strings.TrimSpace(builder.String())
	if base == "" || base == "." || base == ".." {
		base = "upload.bin"
	}

	targetPath := filepath.Join(destDir, base)
	if _, err := os.Stat(targetPath); err == nil {
		ext := filepath.Ext(base)
		nameWithoutExt := strings.TrimSuffix(base, ext)
		timestamp := time.Now().Format("20060102_150405")
		targetPath = filepath.Join(destDir, fmt.Sprintf("%s_%s%s", nameWithoutExt, timestamp, ext))

		n := 1
		for {
			if _, err := os.Stat(targetPath); err != nil {
				break
			}
			targetPath = filepath.Join(destDir, fmt.Sprintf("%s_%s_%d%s", nameWithoutExt, timestamp, n, ext))
			n++
			if n > 1000 {
				break
			}
		}
	}

	finalFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0664)
	if err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok":    false,
			"error": "Failed to create final file: " + err.Error(),
		})
		return
	}
	defer finalFile.Close()

	var totalBytes int64
	for i := 0; i < total; i++ {
		p := filepath.Join(tempChunksDir, fmt.Sprintf("%d.tmp", i))
		chunkFile, err := os.Open(p)
		if err != nil {
			RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"ok":    false,
				"error": fmt.Sprintf("Failed to open chunk %d: %v", i, err),
			})
			return
		}
		written, err := io.Copy(finalFile, chunkFile)
		chunkFile.Close()
		if err != nil {
			RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"ok":    false,
				"error": fmt.Sprintf("Failed to merge chunk %d: %v", i, err),
			})
			return
		}
		totalBytes += written
	}

	_ = os.RemoveAll(tempChunksDir)

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"ok":        true,
		"completed": true,
		"name":      filepath.Base(targetPath),
		"bytes":     totalBytes,
		"saved_to":  targetPath,
		"limits": LimitsJSON{
			UploadMaxFilesize: s.cfg.MaxSizeStr,
			PostMaxSize:       s.cfg.MaxSizeStr,
		},
	})
}

func (s *Server) HandleCreateDir(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		RespondJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":    false,
			"error": "Method not allowed",
		})
		return
	}

	rel := r.FormValue("path")
	name := r.FormValue("name")
	if name == "" {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": "Directory name is required",
		})
		return
	}

	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "")
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": "Invalid directory name",
		})
		return
	}

	targetDir, err := SafeJoin(s.cfg.SharedRoot, filepath.Join(rel, name))
	if err != nil {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}

	if err := os.MkdirAll(targetDir, 0775); err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok":    false,
			"error": "Failed to create directory: " + err.Error(),
		})
		return
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
	})
}

func (s *Server) HandleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		RespondJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":    false,
			"error": "Method not allowed",
		})
		return
	}

	rel := r.FormValue("path")
	rel = strings.ReplaceAll(rel, "\\", "/")
	rel = path.Clean("/" + rel)
	rel = strings.TrimPrefix(rel, "/")

	if rel == "" || rel == "/" || rel == "." {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": "Cannot delete shared root directory",
		})
		return
	}

	targetPath, err := SafeJoin(s.cfg.SharedRoot, rel)
	if err != nil {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}

	trashRel := ".Trash-FileLink"

	if rel == trashRel {
		entries, err := os.ReadDir(targetPath)
		if err == nil {
			for _, entry := range entries {
				_ = os.RemoveAll(filepath.Join(targetPath, entry.Name()))
			}
		}
		RespondJSON(w, http.StatusOK, map[string]interface{}{
			"ok": true,
		})
		return
	}

	trashPath := filepath.Join(s.cfg.SharedRoot, trashRel)
	if _, err := os.Stat(trashPath); os.IsNotExist(err) {
		_ = os.MkdirAll(trashPath, 0775)
	}

	if strings.HasPrefix(rel, trashRel+"/") {
		if err := os.RemoveAll(targetPath); err != nil {
			RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"ok":    false,
				"error": "Failed to permanently delete item: " + err.Error(),
			})
			return
		}
		RespondJSON(w, http.StatusOK, map[string]interface{}{
			"ok": true,
		})
		return
	}

	filename := filepath.Base(targetPath)
	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)
	timestamp := time.Now().Format("20060102_150405")

	destName := fmt.Sprintf("%s_%s%s", nameWithoutExt, timestamp, ext)
	destPath := filepath.Join(trashPath, destName)

	if err := os.Rename(targetPath, destPath); err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok":    false,
			"error": "Failed to move item to Recycle Bin: " + err.Error(),
		})
		return
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"ok":         true,
		"trash_path": trashRel + "/" + destName,
	})
}

func (s *Server) HandleMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		RespondJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":    false,
			"error": "Method not allowed",
		})
		return
	}

	src := r.FormValue("src")
	dest := r.FormValue("dest")

	src = strings.ReplaceAll(src, "\\", "/")
	src = path.Clean("/" + src)
	src = strings.TrimPrefix(src, "/")

	dest = strings.ReplaceAll(dest, "\\", "/")
	dest = path.Clean("/" + dest)
	dest = strings.TrimPrefix(dest, "/")

	if src == "" {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": "Source path cannot be empty",
		})
		return
	}

	srcPath, err := SafeJoin(s.cfg.SharedRoot, src)
	if err != nil {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": "Invalid source path: " + err.Error(),
		})
		return
	}

	destPath, err := SafeJoin(s.cfg.SharedRoot, dest)
	if err != nil {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": "Invalid destination path: " + err.Error(),
		})
		return
	}

	info, err := os.Stat(destPath)
	if err == nil && info.IsDir() {
		filename := filepath.Base(srcPath)
		destPath = filepath.Join(destPath, filename)
	}

	if _, err := os.Stat(destPath); err == nil {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": "Destination file already exists",
		})
		return
	}

	if err := os.Rename(srcPath, destPath); err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok":    false,
			"error": "Failed to move file: " + err.Error(),
		})
		return
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
	})
}

func (s *Server) HandleCopy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		RespondJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":    false,
			"error": "Method not allowed",
		})
		return
	}

	src := r.FormValue("src")
	dest := r.FormValue("dest")

	src = strings.ReplaceAll(src, "\\", "/")
	src = path.Clean("/" + src)
	src = strings.TrimPrefix(src, "/")

	dest = strings.ReplaceAll(dest, "\\", "/")
	dest = path.Clean("/" + dest)
	dest = strings.TrimPrefix(dest, "/")

	if src == "" {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": "Source path cannot be empty",
		})
		return
	}

	srcPath, err := SafeJoin(s.cfg.SharedRoot, src)
	if err != nil {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": "Invalid source path: " + err.Error(),
		})
		return
	}

	destPath, err := SafeJoin(s.cfg.SharedRoot, dest)
	if err != nil {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": "Invalid destination path: " + err.Error(),
		})
		return
	}

	info, err := os.Stat(destPath)
	if err == nil && info.IsDir() {
		filename := filepath.Base(srcPath)
		destPath = filepath.Join(destPath, filename)
	}

	if _, err := os.Stat(destPath); err == nil {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": "Destination file already exists",
		})
		return
	}

	if err := copyFileOrDir(srcPath, destPath); err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok":    false,
			"error": "Failed to copy file: " + err.Error(),
		})
		return
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
	})
}

func copyFileOrDir(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return copyDirHelper(src, dest)
	}
	return copyFileHelper(src, dest)
}

func copyDirHelper(src, dest string) error {
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		s := filepath.Join(src, entry.Name())
		d := filepath.Join(dest, entry.Name())
		if err := copyFileOrDir(s, d); err != nil {
			return err
		}
	}
	return nil
}

func copyFileHelper(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
