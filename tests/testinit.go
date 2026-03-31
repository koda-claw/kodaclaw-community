package tests

import "os"

func init() {
	os.Setenv("ADMIN_API_KEY", "dev-admin-secret")
}
