package security

import (
	"archive/zip"
	"errors"
	"path/filepath"
	"strings"
)

var (
	ErrPathTraversal     = errors.New("path traversal detected")
	ErrTotalSizeExceeded = errors.New("total uncompressed size exceeds limit")
	ErrExecutableFile    = errors.New("executable files not allowed")
)

// BlockedExecutableExts are file extensions that are not allowed in uploads
var BlockedExecutableExts = map[string]bool{
	".exe": true, ".bat": true, ".cmd": true, ".ps1": true,
	".sh": true, ".bash": true, ".zsh": true,
	".com": true, ".msi": true, ".dll": true, ".so": true,
	".dylib": true, ".app": true, ".bin": true,
	".vbs": true, ".wsf": true, ".wsh": true,
}

const MaxUncompressedSize = 200 * 1024 * 1024 // 200MB

func ScanZip(zipPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	var totalSize int64
	for _, f := range r.File {
		// Check path traversal
		clean := filepath.Clean(f.Name)
		if strings.Contains(clean, "..") || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "\\") {
			return ErrPathTraversal
		}

		// Check executable extensions
		ext := strings.ToLower(filepath.Ext(clean))
		if BlockedExecutableExts[ext] {
			return ErrExecutableFile
		}

		// Check total uncompressed size
		totalSize += int64(f.UncompressedSize64)
		if totalSize > MaxUncompressedSize {
			return ErrTotalSizeExceeded
		}
	}

	return nil
}
