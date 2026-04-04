package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	var assetType, tag, query, author, sort string
	var page, pageSize int

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search assets",
		Run: func(cmd *cobra.Command, args []string) {
			creds := mustLoadCreds()

			params := url.Values{}
			if assetType != "" {
				params.Set("type", assetType)
			}
			if tag != "" {
				params.Set("tag", tag)
			}
			if query != "" {
				params.Set("q", query)
			}
			if author != "" {
				params.Set("author", author)
			}
			if sort != "" {
				params.Set("sort", sort)
			}
			params.Set("page", strconv.Itoa(page))
			params.Set("page_size", strconv.Itoa(pageSize))

			endpoint := creds.BaseURL + "/api/v1/assets?" + params.Encode()
			resp, err := doJSON("GET", endpoint, creds.APIKey, nil)
			if err != nil {
				exitErr(err.Error())
			}
			handleResp(resp)
		},
	}
	cmd.Flags().StringVar(&assetType, "type", "", "Filter by type: skill|soul")
	cmd.Flags().StringVar(&tag, "tag", "", "Filter by tag")
	cmd.Flags().StringVar(&query, "q", "", "Search query")
	cmd.Flags().StringVar(&author, "author", "", "Filter by author UUID")
	cmd.Flags().StringVar(&sort, "sort", "", "Sort by: downloads (most downloaded), rating (highest rated), created_at (newest, default)")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&pageSize, "page-size", 20, "Page size")
	return cmd
}

func newDownloadCmd() *cobra.Command {
	var version, outputDir string

	cmd := &cobra.Command{
		Use:   "download <asset_id>",
		Short: "Download an asset ZIP",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			creds := mustLoadCreds()

			params := url.Values{}
			if version != "" {
				params.Set("version", version)
			}
			endpoint := creds.BaseURL + "/api/v1/assets/" + args[0] + "/download"
			if len(params) > 0 {
				endpoint += "?" + params.Encode()
			}

			req, err := http.NewRequest("GET", endpoint, nil)
			if err != nil {
				exitErr(err.Error())
			}
			req.Header.Set("Authorization", "Bearer "+creds.APIKey)

			resp, err := httpClient.Do(req)
			if err != nil {
				exitErr(err.Error())
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 400 {
				var result interface{}
				_ = json.NewDecoder(resp.Body).Decode(&result)
				enc := json.NewEncoder(os.Stderr)
				enc.SetIndent("", "  ")
				_ = enc.Encode(result)
				os.Exit(1)
			}

			// Determine filename from Content-Disposition or fallback.
			filename := args[0] + ".zip"
			if cd := resp.Header.Get("Content-Disposition"); cd != "" {
				// e.g. attachment; filename="<uuid>-<version>.zip"
				for _, part := range strings.Split(cd, ";") {
					part = strings.TrimSpace(part)
					if strings.HasPrefix(part, "filename=") {
						filename = strings.Trim(strings.TrimPrefix(part, "filename="), `"`)
					}
				}
			}

			if outputDir == "" {
				outputDir = "."
			}
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				exitErr(fmt.Sprintf("failed to create output directory: %v", err))
			}

			destPath := filepath.Join(outputDir, filename)
			f, err := os.Create(destPath)
			if err != nil {
				exitErr(fmt.Sprintf("failed to create file: %v", err))
			}
			defer f.Close()

			written, err := io.Copy(f, resp.Body)
			if err != nil {
				exitErr(fmt.Sprintf("failed to write file: %v", err))
			}

			printOut(map[string]interface{}{
				"status": "downloaded",
				"path":   destPath,
				"bytes":  written,
			})
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "Asset version (default: current)")
	cmd.Flags().StringVar(&outputDir, "output", ".", "Output directory")
	return cmd
}

func newUploadCmd() *cobra.Command {
	var name, assetType, version, description, tags, changelog string

	cmd := &cobra.Command{
		Use:   "upload <file.zip>",
		Short: "Upload a new asset",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			creds := mustLoadCreds()
			filePath := args[0]

			f, err := os.Open(filePath)
			if err != nil {
				exitErr(fmt.Sprintf("failed to open file: %v", err))
			}
			defer f.Close()

			var buf bytes.Buffer
			w := multipart.NewWriter(&buf)

			_ = w.WriteField("name", name)
			_ = w.WriteField("type", assetType)
			_ = w.WriteField("version", version)
			_ = w.WriteField("description", description)
			if tags != "" {
				_ = w.WriteField("tags", tags)
			}
			if changelog != "" {
				_ = w.WriteField("changelog", changelog)
			}

			fw, err := w.CreateFormFile("file", filepath.Base(filePath))
			if err != nil {
				exitErr(fmt.Sprintf("failed to create form file: %v", err))
			}
			if _, err := io.Copy(fw, f); err != nil {
				exitErr(fmt.Sprintf("failed to write file content: %v", err))
			}
			w.Close()

			req, err := http.NewRequest("POST", creds.BaseURL+"/api/v1/assets", &buf)
			if err != nil {
				exitErr(err.Error())
			}
			req.Header.Set("Content-Type", w.FormDataContentType())
			req.Header.Set("Authorization", "Bearer "+creds.APIKey)

			resp, err := httpClient.Do(req)
			if err != nil {
				exitErr(err.Error())
			}
			handleResp(resp)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Asset name (required)")
	cmd.Flags().StringVar(&assetType, "type", "", "Asset type: skill|soul (required)")
	cmd.Flags().StringVar(&version, "version", "", "Version e.g. 1.0.0 (required)")
	cmd.Flags().StringVar(&description, "description", "", "Description")
	cmd.Flags().StringVar(&tags, "tags", "", "Comma-separated tags")
	cmd.Flags().StringVar(&changelog, "changelog", "", "Changelog for this version")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("type")
	_ = cmd.MarkFlagRequired("version")
	return cmd
}

func newUploadVersionCmd() *cobra.Command {
	var filePath, changelog, version string

	cmd := &cobra.Command{
		Use:   "upload-version <asset_id>",
		Short: "Upload a new version for an existing asset",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			creds := mustLoadCreds()
			assetID := args[0]

			f, err := os.Open(filePath)
			if err != nil {
				exitErr(fmt.Sprintf("failed to open file: %v", err))
			}
			defer f.Close()

			var buf bytes.Buffer
			w := multipart.NewWriter(&buf)

			_ = w.WriteField("version", version)
			if changelog != "" {
				_ = w.WriteField("changelog", changelog)
			}

			fw, err := w.CreateFormFile("file", filepath.Base(filePath))
			if err != nil {
				exitErr(fmt.Sprintf("failed to create form file: %v", err))
			}
			if _, err := io.Copy(fw, f); err != nil {
				exitErr(fmt.Sprintf("failed to write file content: %v", err))
			}
			w.Close()

			req, err := http.NewRequest("POST", creds.BaseURL+"/api/v1/assets/"+assetID+"/versions", &buf)
			if err != nil {
				exitErr(err.Error())
			}
			req.Header.Set("Content-Type", w.FormDataContentType())
			req.Header.Set("Authorization", "Bearer "+creds.APIKey)

			resp, err := httpClient.Do(req)
			if err != nil {
				exitErr(err.Error())
			}
			handleResp(resp)
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "Path to .zip file (required)")
	cmd.Flags().StringVar(&version, "version", "", "Version e.g. 1.2.0 (required)")
	cmd.Flags().StringVar(&changelog, "changelog", "", "Changelog for this version")
	_ = cmd.MarkFlagRequired("file")
	_ = cmd.MarkFlagRequired("version")
	return cmd
}

func newSetVersionCmd() *cobra.Command {
	var version string

	cmd := &cobra.Command{
		Use:   "set-version <asset_id>",
		Short: "Set the current version of an asset",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			creds := mustLoadCreds()
			assetID := args[0]

			body := map[string]string{"version": version}
			resp, err := doJSON("PATCH", creds.BaseURL+"/api/v1/assets/"+assetID+"/versions/current", creds.APIKey, body)
			if err != nil {
				exitErr(err.Error())
			}
			handleResp(resp)
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "Version to set as current (required)")
	_ = cmd.MarkFlagRequired("version")
	return cmd
}

// --- Edit Command ---
func newEditCmd() *cobra.Command {
	var desc, name, tags string

	cmd := &cobra.Command{
		Use:   "edit <asset_id>",
		Short: "Edit asset metadata (name, description, tags)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			creds := mustLoadCreds()
			assetID := args[0]

			body := make(map[string]interface{})
			if name != "" {
				body["name"] = name
			}
			if desc != "" {
				body["description"] = desc
			}
			if tags != "" {
				body["tags"] = strings.Split(tags, ",")
			}

			if len(body) == 0 {
				exitErr("nothing to update, use --name, --description, or --tags")
			}

			resp, err := doJSON("PATCH", creds.BaseURL+"/api/v1/assets/"+assetID, creds.APIKey, body)
			if err != nil {
				exitErr(err.Error())
			}
			handleResp(resp)

			// Warn that approved assets go back to pending
			fmt.Fprintln(os.Stderr, "Note: editing an approved asset will revert it to pending status and require re-review.")
		},
	}
	cmd.Flags().StringVar(&desc, "description", "", "New description")
	cmd.Flags().StringVar(&name, "name", "", "New name")
	cmd.Flags().StringVar(&tags, "tags", "", "New tags (comma-separated)")
	return cmd
}

// --- Versions Command ---
func newVersionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "versions <asset_id>",
		Short: "List all versions of an asset",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			creds := mustLoadCreds()
			assetID := args[0]

			resp, err := doJSON("GET", creds.BaseURL+"/api/v1/assets/"+assetID+"/versions", creds.APIKey, nil)
			if err != nil {
				exitErr(err.Error())
			}
			handleResp(resp)
		},
	}
	return cmd
}

// --- My Assets Command ---
func newMyAssetsCmd() *cobra.Command {
	var status string

	cmd := &cobra.Command{
		Use:   "my-assets",
		Short: "List your submitted assets and their review status",
		Run: func(cmd *cobra.Command, args []string) {
			creds := mustLoadCreds()

			// Get current user ID
			meResp, err := doJSON("GET", creds.BaseURL+"/api/v1/users/me", creds.APIKey, nil)
			if err != nil {
				exitErr(err.Error())
			}
			var me struct {
				ID       string `json:"id"`
				Username string `json:"username"`
			}
			if err := json.NewDecoder(meResp.Body).Decode(&me); err != nil {
				exitErr(fmt.Sprintf("failed to decode user: %v", err))
			}
			meResp.Body.Close()

			// Fetch user's assets
			params := url.Values{}
			params.Set("page", "1")
			params.Set("page_size", "50")
			if status != "" {
				params.Set("status", status)
			}
			endpoint := creds.BaseURL + "/api/v1/users/" + me.ID + "/assets?" + params.Encode()
			resp, err := doJSON("GET", endpoint, creds.APIKey, nil)
			if err != nil {
				exitErr(err.Error())
			}
			defer resp.Body.Close()

			var result struct {
				Items []struct {
					ID           string `json:"id"`
					Name         string `json:"name"`
					Type         string `json:"type"`
					Status       string `json:"status"`
					RejectReason string `json:"rejection_reason"`
					Version      string `json:"current_version"`
					CreatedAt    string `json:"created_at"`
				} `json:"items"`
				Total int `json:"total"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				exitErr(fmt.Sprintf("failed to decode response: %v", err))
			}

			if isJSON() {
				outputJSON(result)
				return
			}

			label := "我的资产"
			if status != "" {
				label = "我的资产（" + status + "）"
			}
			fmt.Printf("%s (共 %d 个):\n", label, result.Total)
			if len(result.Items) == 0 {
				fmt.Println("  （暂无资产）")
				return
			}
			for i, a := range result.Items {
				statusIcon := "✅"
				switch a.Status {
				case "pending":
					statusIcon = "⏳"
				case "rejected":
					statusIcon = "❌"
				}
				fmt.Printf("  [%d] %s %s (%s) v%s [%s]\n", i+1, statusIcon, a.Name, a.Type, a.Version, a.Status)
				if a.Status == "rejected" && a.RejectReason != "" {
					fmt.Printf("       拒绝原因: %s\n", a.RejectReason)
				}
				fmt.Printf("       ID: %s\n", a.ID)
			}
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "Filter by status: pending|approved|rejected")
	return cmd
}

// --- Delete Command ---
func newDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <asset_id>",
		Short: "Delete one of your submitted assets",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			creds := mustLoadCreds()
			assetID := args[0]

			// First fetch asset info to show name
			infoResp, err := doJSON("GET", creds.BaseURL+"/api/v1/assets/"+assetID, creds.APIKey, nil)
			if err != nil {
				exitErr(err.Error())
			}
			var info struct {
				Name string `json:"name"`
				Type string `json:"type"`
			}
			if err := json.NewDecoder(infoResp.Body).Decode(&info); err != nil {
				exitErr(fmt.Sprintf("failed to decode asset: %v", err))
			}
			infoResp.Body.Close()

			fmt.Printf("确定要删除资产 %s (%s) 吗？此操作不可撤销。\n", info.Name, info.Type)
			fmt.Print("输入 y 确认: ")
			var confirm string
			fmt.Scanln(&confirm)
			if confirm != "y" && confirm != "Y" {
				fmt.Println("已取消")
				return
			}

			resp, err := doJSON("DELETE", creds.BaseURL+"/api/v1/assets/"+assetID, creds.APIKey, nil)
			if err != nil {
				exitErr(err.Error())
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 400 {
				var errBody struct {
					Message string `json:"message"`
				}
				json.NewDecoder(resp.Body).Decode(&errBody)
				exitErr(fmt.Sprintf("删除失败 (HTTP %d): %s", resp.StatusCode, errBody.Message))
			}

			fmt.Printf("✅ 已删除: %s\n", info.Name)
		},
	}
}
