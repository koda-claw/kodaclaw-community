package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

func newBindGitHubCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "bind-github",
		Short: "Bind GitHub account to your KodaClaw account",
		Run: func(cmd *cobra.Command, args []string) {
			creds := mustLoadCreds()
			base := creds.BaseURL

			// Get GitHub OAuth URL
			resp, err := doJSON("GET", base+"/api/v1/auth/github?redirect=/bind", creds.APIKey, nil)
			if err != nil {
				exitErr(err.Error())
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				exitErr(fmt.Sprintf("failed to decode response: %v", err))
			}
			if resp.StatusCode >= 400 {
				enc := json.NewEncoder(os.Stderr)
				enc.SetIndent("", "  ")
				_ = enc.Encode(result)
				os.Exit(1)
			}

			authURL, ok := result["url"].(string)
			if !ok || authURL == "" {
				exitErr("failed to get GitHub OAuth URL")
			}

			fmt.Println("Opening browser for GitHub OAuth...")
			openBrowser(authURL)
			fmt.Println("请完成 GitHub 绑定后，运行以下命令刷新凭证：")
			fmt.Println("  kc-community login <username> <password>")
		},
	}
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		fmt.Printf("Please open this URL manually: %s\n", url)
		return
	}
	if err != nil {
		fmt.Printf("请手动打开此 URL: %s\n", url)
	}
}
