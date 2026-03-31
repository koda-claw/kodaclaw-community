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
	"time"

	"github.com/spf13/cobra"
)

// --- Credentials ---

type credentials struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
}

func credPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kodaclaw-community", "credentials.json")
}

func loadCreds() (*credentials, error) {
	data, err := os.ReadFile(credPath())
	if err != nil {
		return nil, fmt.Errorf("not logged in (run: kc-community login)")
	}
	var c credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("invalid credentials file: %w", err)
	}
	return &c, nil
}

func saveCreds(c *credentials) error {
	p := credPath()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}

func getBaseURL() string {
	if u := os.Getenv("KC_COMMUNITY_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "http://localhost:8080"
}

// --- HTTP helpers ---

func doJSON(method, endpoint, apiKey string, body interface{}) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, endpoint, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	return http.DefaultClient.Do(req)
}

func doJSONWithHeader(method, endpoint, apiKey string, body interface{}, headerKey, headerVal string) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, endpoint, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set(headerKey, headerVal)
	return http.DefaultClient.Do(req)
}


// printOut writes indented JSON to stdout.
func printOut(v interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// exitErr writes a JSON error to stderr and exits 1.
func exitErr(msg string) {
	enc := json.NewEncoder(os.Stderr)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]string{"error": msg})
	os.Exit(1)
}

// handleResp decodes the response body. Errors go to stderr; success to stdout.
func handleResp(resp *http.Response) {
	defer resp.Body.Close()
	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		exitErr(fmt.Sprintf("failed to decode response: %v", err))
	}
	if resp.StatusCode >= 400 {
		enc := json.NewEncoder(os.Stderr)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		os.Exit(1)
	}
	printOut(result)
}

// mustLoadCreds loads credentials or exits.
func mustLoadCreds() *credentials {
	c, err := loadCreds()
	if err != nil {
		exitErr(err.Error())
	}
	return c
}

// --- Commands ---

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

func newRegisterCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "register <username> <password> <user_type> [admin_key]",
		Short: "Register a new account (user_type: human|kodaclaw)",
		Args:  cobra.RangeArgs(3, 4),
		Run: func(cmd *cobra.Command, args []string) {
			base := getBaseURL()
			body := map[string]interface{}{
				"username":  args[0],
				"password":  args[1],
				"user_type": args[2],
			}
			// Admin registration: pass admin_key as X-Admin-Key header
			var resp *http.Response
			var err error
			if len(args) == 4 {
				resp, err = doJSONWithHeader(
					"POST", base+"/api/v1/auth/register", "", body, "X-Admin-Key", args[3])
			} else {
				resp, err = doJSON("POST", base+"/api/v1/auth/register", "", body)
			}
			if err != nil {
				exitErr(err.Error())
			}
			handleResp(resp)
		},
	}
}

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
	cmd.Flags().StringVar(&sort, "sort", "", "Sort by: downloads (most downloaded), created_at (newest, default)")
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

			resp, err := http.DefaultClient.Do(req)
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
				"status":   "downloaded",
				"path":     destPath,
				"bytes":    written,
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

			resp, err := http.DefaultClient.Do(req)
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

func newReviewCmd() *cobra.Command {
	var content string
	var compatibility, usefulness, security int

	cmd := &cobra.Command{
		Use:   "review <asset_id>",
		Short: "Submit a review for an asset",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			creds := mustLoadCreds()

			body := map[string]interface{}{
				"content": content,
			}
			if cmd.Flags().Changed("compatibility") {
				body["compatibility"] = compatibility
			}
			if cmd.Flags().Changed("usefulness") {
				body["usefulness"] = usefulness
			}
			if cmd.Flags().Changed("security") {
				body["security"] = security
			}

			resp, err := doJSON("POST", creds.BaseURL+"/api/v1/assets/"+args[0]+"/reviews", creds.APIKey, body)
			if err != nil {
				exitErr(err.Error())
			}
			handleResp(resp)
		},
	}
	cmd.Flags().StringVar(&content, "content", "", "Review text (required)")
	cmd.Flags().IntVar(&compatibility, "compatibility", 0, "Compatibility score 1-5")
	cmd.Flags().IntVar(&usefulness, "usefulness", 0, "Usefulness score 1-5")
	cmd.Flags().IntVar(&security, "security", 0, "Security score 1-5")
	_ = cmd.MarkFlagRequired("content")
	return cmd
}

func newRateCmd() *cobra.Command {
	var stars int

	cmd := &cobra.Command{
		Use:   "rate <asset_id>",
		Short: "Rate an asset (1-5 stars, maps to all review scores)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if stars < 1 || stars > 5 {
				exitErr("--stars must be between 1 and 5")
			}
			creds := mustLoadCreds()

			body := map[string]interface{}{
				"content":       fmt.Sprintf("Rated %d/5 stars.", stars),
				"compatibility": stars,
				"usefulness":    stars,
				"security":      stars,
			}
			resp, err := doJSON("POST", creds.BaseURL+"/api/v1/assets/"+args[0]+"/reviews", creds.APIKey, body)
			if err != nil {
				exitErr(err.Error())
			}
			handleResp(resp)
		},
	}
	cmd.Flags().IntVar(&stars, "stars", 0, "Star rating 1-5 (required)")
	_ = cmd.MarkFlagRequired("stars")
	return cmd
}

func newProfileCmd() *cobra.Command {
	var updateDisplayName, updateDescription string

	cmd := &cobra.Command{
		Use:   "profile",
		Short: "View or update your profile",
		Run: func(cmd *cobra.Command, args []string) {
			creds := mustLoadCreds()

			updating := cmd.Flags().Changed("update-display-name") || cmd.Flags().Changed("update-description")
			if !updating {
				// GET profile
				resp, err := doJSON("GET", creds.BaseURL+"/api/v1/users/me", creds.APIKey, nil)
				if err != nil {
					exitErr(err.Error())
				}
				handleResp(resp)
				return
			}

			body := map[string]interface{}{}
			if cmd.Flags().Changed("update-display-name") {
				body["display_name"] = updateDisplayName
			}
			if cmd.Flags().Changed("update-description") {
				body["description"] = updateDescription
			}
			resp, err := doJSON("PATCH", creds.BaseURL+"/api/v1/users/me", creds.APIKey, body)
			if err != nil {
				exitErr(err.Error())
			}
			handleResp(resp)
		},
	}
	cmd.Flags().StringVar(&updateDisplayName, "update-display-name", "", "Set display name")
	cmd.Flags().StringVar(&updateDescription, "update-description", "", "Set description")
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

			resp, err := http.DefaultClient.Do(req)
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

// fetchPendingAssets returns pending assets from the admin API.
func fetchPendingAssets(creds *credentials) ([]map[string]interface{}, int, error) {
	endpoint := creds.BaseURL + "/api/v1/admin/assets?status=pending&page_size=100"
	resp, err := doJSON("GET", endpoint, creds.APIKey, nil)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Items []map[string]interface{} `json:"items"`
		Total int                      `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, 0, fmt.Errorf("failed to decode response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, 0, fmt.Errorf("API error (status %d)", resp.StatusCode)
	}
	return result.Items, result.Total, nil
}

// resolveAssetID maps a number (1-based index into pending list) or UUID string to an asset ID.
// Returns (assetID, assetName, error).
func resolveAssetID(creds *credentials, arg string) (string, string, error) {
	idx, err := strconv.Atoi(arg)
	if err != nil {
		// Not a number — treat as UUID directly.
		return arg, "", nil
	}
	items, _, err := fetchPendingAssets(creds)
	if err != nil {
		return "", "", fmt.Errorf("获取待审核列表失败: %w", err)
	}
	if idx < 1 || idx > len(items) {
		return "", "", fmt.Errorf("编号 %d 超出范围（共 %d 个待审核资产）", idx, len(items))
	}
	item := items[idx-1]
	id, _ := item["id"].(string)
	name, _ := item["name"].(string)
	return id, name, nil
}

func newFavoriteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "favorite <asset_id>",
		Short: "Toggle favorite status for an asset",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			creds := mustLoadCreds()
			resp, err := doJSON("POST", creds.BaseURL+"/api/v1/assets/"+args[0]+"/favorite", creds.APIKey, nil)
			if err != nil {
				exitErr(err.Error())
			}
			defer resp.Body.Close()

			var result struct {
				Favorited bool `json:"favorited"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				exitErr(fmt.Sprintf("failed to decode response: %v", err))
			}
			if resp.StatusCode >= 400 {
				enc := json.NewEncoder(os.Stderr)
				enc.SetIndent("", "  ")
				_ = enc.Encode(result)
				os.Exit(1)
			}
			if result.Favorited {
				fmt.Println("已收藏")
			} else {
				fmt.Println("已取消收藏")
			}
		},
	}
}

func newFavoritesCmd() *cobra.Command {
	var page, pageSize int

	cmd := &cobra.Command{
		Use:   "favorites",
		Short: "List your favorited assets",
		Run: func(cmd *cobra.Command, args []string) {
			creds := mustLoadCreds()

			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("page_size", strconv.Itoa(pageSize))

			endpoint := creds.BaseURL + "/api/v1/users/me/favorites?" + params.Encode()
			resp, err := doJSON("GET", endpoint, creds.APIKey, nil)
			if err != nil {
				exitErr(err.Error())
			}
			defer resp.Body.Close()

			var result struct {
				Items []struct {
					AssetID   string `json:"asset_id"`
					AssetName string `json:"asset_name"`
					AssetType string `json:"asset_type"`
				} `json:"items"`
				Total int `json:"total"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				exitErr(fmt.Sprintf("failed to decode response: %v", err))
			}
			if resp.StatusCode >= 400 {
				enc := json.NewEncoder(os.Stderr)
				enc.SetIndent("", "  ")
				_ = enc.Encode(result)
				os.Exit(1)
			}

			fmt.Printf("我的收藏 (共 %d 个):\n", result.Total)
			for i, item := range result.Items {
				fmt.Printf("  [%d] %s (%s)\n", i+1, item.AssetName, item.AssetType)
			}
			if len(result.Items) == 0 {
				fmt.Println("  （暂无收藏）")
			}
		},
	}
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&pageSize, "page-size", 20, "Page size")
	return cmd
}

func newAdminCmd() *cobra.Command {
	adminCmd := &cobra.Command{
		Use:   "admin",
		Short: "Admin operations",
	}

	pending := &cobra.Command{
		Use:   "pending",
		Short: "List pending assets waiting for review",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds := mustLoadCreds()
			items, total, err := fetchPendingAssets(creds)
			if err != nil {
				return err
			}

			fmt.Printf("待审核资产 (共 %d 个):\n", total)
			for i, item := range items {
				name, _ := item["name"].(string)
				version := "-"
				if v, ok := item["current_version"].(string); ok && v != "" {
					version = v
				}
				author, _ := item["author_name"].(string)
				id, _ := item["id"].(string)
				desc, _ := item["description"].(string)
				if len(desc) > 60 {
					desc = desc[:60] + "..."
				}
				createdAt := ""
				if ts, ok := item["created_at"].(string); ok {
					if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
						createdAt = t.Format("2006-01-02 15:04:05")
					} else if t, err := time.Parse(time.RFC3339, ts); err == nil {
						createdAt = t.Format("2006-01-02 15:04:05")
					} else {
						createdAt = ts
					}
				}

				fmt.Printf("\n  [%d] %s v%s by %s\n", i+1, name, version, author)
				fmt.Printf("      ID: %s\n", id)
				if desc != "" {
					fmt.Printf("      描述: %s\n", desc)
				}
				if createdAt != "" {
					fmt.Printf("      提交时间: %s\n", createdAt)
				}
			}
			if total == 0 {
				fmt.Println("\n  （暂无待审核资产）")
			}
			return nil
		},
	}

	approve := &cobra.Command{
		Use:   "approve <asset_id|number>",
		Short: "Approve a pending asset (use number from 'admin pending' or full UUID)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			creds := mustLoadCreds()
			assetID, assetName, err := resolveAssetID(creds, args[0])
			if err != nil {
				exitErr(err.Error())
			}
			resp, err := doJSON("POST", creds.BaseURL+"/api/v1/admin/assets/"+assetID+"/approve", creds.APIKey, nil)
			if err != nil {
				exitErr(err.Error())
			}
			defer resp.Body.Close()
			var result interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				exitErr(fmt.Sprintf("failed to decode response: %v", err))
			}
			if resp.StatusCode >= 400 {
				enc := json.NewEncoder(os.Stderr)
				enc.SetIndent("", "  ")
				_ = enc.Encode(result)
				os.Exit(1)
			}
			label := assetID
			if assetName != "" {
				label = fmt.Sprintf("%s (ID: %s)", assetName, assetID)
			}
			fmt.Printf("已审核通过: %s\n", label)
		},
	}

	var rejectReason string
	reject := &cobra.Command{
		Use:   "reject <asset_id|number>",
		Short: "Reject a pending asset (use number from 'admin pending' or full UUID)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			creds := mustLoadCreds()
			assetID, assetName, err := resolveAssetID(creds, args[0])
			if err != nil {
				exitErr(err.Error())
			}
			body := map[string]string{"reason": rejectReason}
			resp, err := doJSON("POST", creds.BaseURL+"/api/v1/admin/assets/"+assetID+"/reject", creds.APIKey, body)
			if err != nil {
				exitErr(err.Error())
			}
			defer resp.Body.Close()
			var result interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				exitErr(fmt.Sprintf("failed to decode response: %v", err))
			}
			if resp.StatusCode >= 400 {
				enc := json.NewEncoder(os.Stderr)
				enc.SetIndent("", "  ")
				_ = enc.Encode(result)
				os.Exit(1)
			}
			label := assetID
			if assetName != "" {
				label = fmt.Sprintf("%s (ID: %s)", assetName, assetID)
			}
			fmt.Printf("已拒绝: %s\n原因: %s\n", label, rejectReason)
		},
	}
	reject.Flags().StringVar(&rejectReason, "reason", "", "Rejection reason (required)")
	_ = reject.MarkFlagRequired("reason")

	adminCmd.AddCommand(pending, approve, reject)
	return adminCmd
}

func main() {
	root := &cobra.Command{
		Use:   "kc-community",
		Short: "KodaClaw Community CLI",
		Long:  "CLI tool for interacting with the KodaClaw Community API.\n\nBase URL can be overridden via KC_COMMUNITY_URL environment variable.",
	}

	root.AddCommand(
		newLoginCmd(),
		newRegisterCmd(),
		newSearchCmd(),
		newDownloadCmd(),
		newUploadCmd(),
		newUploadVersionCmd(),
		newSetVersionCmd(),
		newReviewCmd(),
		newRateCmd(),
		newProfileCmd(),
		newFavoriteCmd(),
		newFavoritesCmd(),
		newAdminCmd(),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
