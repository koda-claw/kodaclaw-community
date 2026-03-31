package tests

import (
	"archive/zip"
	"bytes"
	"os"
)

func init() {
	os.Setenv("ADMIN_API_KEY", "dev-admin-secret")
}

// makeMinimalZip creates a valid empty ZIP archive for use in tests.
func makeMinimalZip() []byte {
	buf := &bytes.Buffer{}
	w := zip.NewWriter(buf)
	w.Close()
	return buf.Bytes()
}
