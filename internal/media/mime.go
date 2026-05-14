package media

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var extMap = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
	".mp4":  "video/mp4",
	".mov":  "video/quicktime",
	".webm": "video/webm",
	".avi":  "video/x-msvideo",
	".mkv":  "video/x-matroska",
	".pdf":  "application/pdf",
	".zip":  "application/zip",
	".tar":  "application/x-tar",
	".gz":   "application/gzip",
	".go":   "text/x-go",
	".py":   "text/x-python",
	".js":   "text/javascript",
	".ts":   "text/typescript",
	".md":   "text/markdown",
	".json": "application/json",
	".yaml": "text/yaml",
	".yml":  "text/yaml",
	".html": "text/html",
	".css":  "text/css",
	".txt":  "text/plain",
	".sh":   "text/x-shellscript",
	".sql":  "text/x-sql",
	".xml":  "text/xml",
	".csv":  "text/csv",
	".rs":   "text/x-rust",
	".c":    "text/x-c",
	".cpp":  "text/x-c++",
	".h":    "text/x-c",
	".java": "text/x-java",
	".rb":   "text/x-ruby",
}

var videoExtensions = map[string]bool{
	".mp4": true, ".mov": true, ".webm": true,
	".avi": true, ".mkv": true, ".flv": true,
}

var sniffPool = sync.Pool{
	New: func() any { return new([512]byte) },
}

func SniffMIME(filePath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	if mime, ok := extMap[ext]; ok {
		return mime, nil
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	buf := sniffPool.Get().(*[512]byte)
	defer sniffPool.Put(buf)

	n, _ := f.Read(buf[:])
	detected := http.DetectContentType(buf[:n])

	if detected == "application/octet-stream" && videoExtensions[ext] {
		return "video/unknown", nil
	}
	return detected, nil
}

func IsImage(mime string) bool {
	return strings.HasPrefix(mime, "image/")
}

func IsVideo(mime string) bool {
	return strings.HasPrefix(mime, "video/")
}

func ItemTypeFromMIME(mime string) string {
	switch {
	case IsImage(mime):
		return "image"
	case IsVideo(mime):
		return "video"
	default:
		return "file"
	}
}
