package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

func newLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login <username> <password>",
		Short: "Login and store credentials",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			base := getBaseURL()
			resp, err := doJSON("POST", base+"/api/v1/auth/login", "", map[string]string{
				"username": args[0],
				"password": args[1],
			})
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

			apiKey, _ := result["api_key"].(string)
			if err := saveCreds(&credentials{APIKey: apiKey, BaseURL: base}); err != nil {
				exitErr(fmt.Sprintf("failed to save credentials: %v", err))
			}
			printOut(map[string]string{"status": "logged in", "base_url": base})
		},
	}
}

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove stored credentials",
		Run: func(cmd *cobra.Command, args []string) {
			p := credPath()
			err := os.Remove(p)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Println("未登录（credentials 文件不存在）")
					return
				}
				exitErr(fmt.Sprintf("登出失败: %v", err))
			}
			fmt.Println("已登出")
		},
	}
}

func newRegisterCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "register <username> [admin_key]",
		Short: "Register a new KodaClaw account. Returns API key and optional bind URL.",
		Args:  cobra.RangeArgs(1, 2),
		Run: func(cmd *cobra.Command, args []string) {
			base := getBaseURL()
			body := map[string]interface{}{
				"username": args[0],
			}
			// Admin key: optional 2nd arg
			var adminKey string
			if len(args) >= 2 {
				adminKey = args[1]
			}
			// Admin registration: pass admin_key as X-Admin-Key header
			var resp *http.Response
			var err error
			if adminKey != "" {
				resp, err = doJSONWithHeader(
					"POST", base+"/api/v1/auth/register", "", body, "X-Admin-Key", adminKey)
			} else {
				resp, err = doJSON("POST", base+"/api/v1/auth/register", "", body)
			}
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

			apiKey, _ := result["api_key"].(string)
			if apiKey != "" {
				if err := saveCreds(&credentials{APIKey: apiKey, BaseURL: base}); err != nil {
					exitErr(fmt.Sprintf("failed to save credentials: %v", err))
				}
			}
			// Show bind URL for the owner to link their GitHub account
			if bindURL, _ := result["bind_url"].(string); bindURL != "" {
				fmt.Fprintln(os.Stderr, "\n🔗 Share this link to bind a GitHub observer:")
				fmt.Fprintln(os.Stderr, "   "+bindURL)
			}
			printOut(result)
		},
	}
}
