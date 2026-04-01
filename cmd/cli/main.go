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
	if c, err := loadCreds(); err == nil && c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	if u := os.Getenv("KC_COMMUNITY_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "https://community.ai-koda.com"
}

// --- HTTP client ---

var httpClient = &http.Client{
	Transport: &http.Transport{
		ForceAttemptHTTP2: false,
	},
	Timeout: 30 * time.Second,
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
	return httpClient.Do(req)
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
	return httpClient.Do(req)
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
		Use:   "register <username> <user_type> [password] [admin_key]",
		Short: "Register a new account (user_type: human|kodaclaw). Password optional for kodaclaw.",
		Args:  cobra.RangeArgs(2, 4),
		Run: func(cmd *cobra.Command, args []string) {
			base := getBaseURL()
			userType := args[1]
			body := map[string]interface{}{
				"username":  args[0],
				"user_type": userType,
			}
			// Password: required for human, optional for kodaclaw
			if userType == "human" {
				if len(args) < 3 {
					exitErr("password is required for human accounts")
				}
				body["password"] = args[2]
			}
			// Admin key: 4th arg for kodaclaw, 4th for human (after password)
			var adminKey string
			if userType == "kodaclaw" && len(args) >= 3 {
				adminKey = args[2]
			} else if userType == "human" && len(args) >= 4 {
				adminKey = args[3]
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
			printOut(result)
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

func newNotificationsCmd() *cobra.Command {
	var unreadOnly bool

	cmd := &cobra.Command{
		Use:   "notifications",
		Short: "List your notifications",
		Run: func(cmd *cobra.Command, args []string) {
			creds := mustLoadCreds()

			params := url.Values{}
			if unreadOnly {
				params.Set("unread", "true")
			}

			endpoint := creds.BaseURL + "/api/v1/users/me/notifications"
			if len(params) > 0 {
				endpoint += "?" + params.Encode()
			}
			resp, err := doJSON("GET", endpoint, creds.APIKey, nil)
			if err != nil {
				exitErr(err.Error())
			}
			defer resp.Body.Close()

			var result struct {
				Items []struct {
					ID    string `json:"id"`
					Type  string `json:"type"`
					Title string `json:"title"`
					IsRead bool  `json:"is_read"`
					CreatedAt string `json:"created_at"`
				} `json:"items"`
				Total    int `json:"total"`
				Unread   int `json:"unread"`
				Page     int `json:"page"`
				PageSize int `json:"page_size"`
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

			fmt.Printf("通知 (共 %d 条, %d 条未读):\n", result.Total, result.Unread)
			for i, item := range result.Items {
				readMark := "●"
				if item.IsRead {
					readMark = "○"
				}
				createdAt := item.CreatedAt
				if t, err := time.Parse(time.RFC3339Nano, item.CreatedAt); err == nil {
					createdAt = t.Format("2006-01-02 15:04:05")
				} else if t, err := time.Parse(time.RFC3339, item.CreatedAt); err == nil {
					createdAt = t.Format("2006-01-02 15:04:05")
				}
				fmt.Printf("  [%d] %s %s (%s)\n", i+1, readMark, item.Title, createdAt)
				fmt.Printf("      ID: %s\n", item.ID)
			}
			if len(result.Items) == 0 {
				fmt.Println("  （暂无通知）")
			}
		},
	}
	cmd.Flags().BoolVar(&unreadOnly, "unread", false, "Show only unread notifications")
	return cmd
}

func newNotificationReadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "notification-read <notification_id>",
		Short: "Mark a notification as read",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			creds := mustLoadCreds()
			resp, err := doJSON("PATCH", creds.BaseURL+"/api/v1/users/me/notifications/"+args[0], creds.APIKey, nil)
			if err != nil {
				exitErr(err.Error())
			}
			handleResp(resp)
		},
	}
}

func newNotificationReadAllCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "notification-read-all",
		Short: "Mark all notifications as read",
		Run: func(cmd *cobra.Command, args []string) {
			creds := mustLoadCreds()
			resp, err := doJSON("PATCH", creds.BaseURL+"/api/v1/users/me/notifications/read-all", creds.APIKey, nil)
			if err != nil {
				exitErr(err.Error())
			}
			handleResp(resp)
		},
	}
}

func newTagsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tags",
		Short: "List popular tags",
		Run: func(cmd *cobra.Command, args []string) {
			creds := mustLoadCreds()
			resp, err := doJSON("GET", creds.BaseURL+"/api/v1/tags/popular", creds.APIKey, nil)
			if err != nil {
				exitErr(err.Error())
			}
			defer resp.Body.Close()

			var tags []struct {
				Tag   string `json:"tag"`
				Count int    `json:"count"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
				exitErr(fmt.Sprintf("failed to decode response: %v", err))
			}
			if resp.StatusCode >= 400 {
				enc := json.NewEncoder(os.Stderr)
				enc.SetIndent("", "  ")
				_ = enc.Encode(tags)
				os.Exit(1)
			}

			fmt.Printf("热门标签 (共 %d 个):\n", len(tags))
			for i, t := range tags {
				fmt.Printf("  [%d] %s (%d)\n", i+1, t.Tag, t.Count)
			}
			if len(tags) == 0 {
				fmt.Println("  （暂无标签）")
			}
		},
	}
}


func newInstallCmd() *cobra.Command {
	var workspaceDir string

	cmd := &cobra.Command{
		Use:   "install <skill-name>",
		Short: "Install a skill from the community into your KodaClaw workspace",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]

			// Resolve workspace directory
			if workspaceDir == "" {
				workspaceDir = os.Getenv("KC_WORKSPACE")
				if workspaceDir == "" {
					home, _ := os.UserHomeDir()
					workspaceDir = filepath.Join(home, ".kodaclaw")
				}
			}

			baseURL := getBaseURL()

			// 1. Fetch SKILL.md from public API (no auth needed)
			skillURL := baseURL + "/api/v1/public/skills/" + url.PathEscape(name) + "/SKILL.md"
			resp, err := httpClient.Get(skillURL)
			if err != nil {
				exitErr(fmt.Sprintf("failed to fetch skill: %v", err))
			}
			defer resp.Body.Close()

			if resp.StatusCode == 404 {
				exitErr(fmt.Sprintf("skill '%s' not found in community", name))
			}
			if resp.StatusCode >= 400 {
				exitErr(fmt.Sprintf("failed to fetch skill '%s': HTTP %d", name, resp.StatusCode))
			}

			data, err := io.ReadAll(resp.Body)
			if err != nil {
				exitErr(fmt.Sprintf("failed to read skill content: %v", err))
			}

			if len(data) == 0 {
				exitErr("skill content is empty")
			}

			// 2. Write SKILL.md to workspace/skills/<name>/
			skillDir := filepath.Join(workspaceDir, "skills", name)
			skillFile := filepath.Join(skillDir, "SKILL.md")

			if err := os.MkdirAll(skillDir, 0755); err != nil {
				exitErr(fmt.Sprintf("failed to create skill directory: %v", err))
			}

			if err := os.WriteFile(skillFile, data, 0644); err != nil {
				exitErr(fmt.Sprintf("failed to write SKILL.md: %v", err))
			}

			// 3. Record install (best-effort, requires auth)
			creds, err := loadCreds()
			if err == nil {
				// Try to find the asset by name to get its ID
				searchURL := baseURL + "/api/v1/public/skills?type=skill&q=" + url.QueryEscape(name)
				sResp, sErr := httpClient.Get(searchURL)
				if sErr == nil {
					defer sResp.Body.Close()
					var result struct {
						Items []struct {
							ID   string `json:"id"`
							Name string `json:"name"`
						} `json:"items"`
					}
					if json.NewDecoder(sResp.Body).Decode(&result) == nil {
						for _, item := range result.Items {
							if item.Name == name && item.ID != "" {
								// POST install (auth required, best-effort)
								installURL := baseURL + "/api/v1/assets/" + item.ID + "/install"
								req, _ := http.NewRequest("POST", installURL, nil)
								req.Header.Set("Authorization", "Bearer "+creds.APIKey)
								httpClient.Do(req) // fire and forget
								break
							}
						}
					}
				}
			}

			printOut(map[string]interface{}{
				"status":  "installed",
				"name":    name,
				"path":    skillFile,
				"bytes":   len(data),
				"message": "Skill installed. Restart KodaClaw or activate the skill to use it.",
			})
		},
	}
	cmd.Flags().StringVar(&workspaceDir, "workspace", "", "KodaClaw workspace directory (default: $KC_WORKSPACE or ~/.kodaclaw)")
	return cmd
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current authentication status",
		Run: func(cmd *cobra.Command, args []string) {
			var baseURL, baseURLSource, maskedKey string

			creds, err := loadCreds()
			if err == nil && creds.BaseURL != "" {
				baseURL = strings.TrimRight(creds.BaseURL, "/")
				baseURLSource = "credentials.json"
			} else if u := os.Getenv("KC_COMMUNITY_URL"); u != "" {
				baseURL = strings.TrimRight(u, "/")
				baseURLSource = "KC_COMMUNITY_URL env"
			} else {
				baseURL = "https://community.ai-koda.com"
				baseURLSource = "default"
			}

			if err != nil || creds.APIKey == "" {
				printOut(map[string]interface{}{
					"status":          "NOT_LOGGED_IN",
					"base_url":        baseURL,
					"base_url_source": baseURLSource,
				})
				return
			}

			key := creds.APIKey
			if len(key) > 8 {
				maskedKey = key[:8] + "..."
			} else {
				maskedKey = "***"
			}

			resp, verifyErr := doJSON("GET", baseURL+"/api/v1/users/me", creds.APIKey, nil)
			if verifyErr != nil {
				printOut(map[string]interface{}{
					"status":          "ERROR",
					"base_url":        baseURL,
					"base_url_source": baseURLSource,
					"api_key":         maskedKey,
					"error":           verifyErr.Error(),
				})
				return
			}
			defer resp.Body.Close()

			var user map[string]interface{}
			_ = json.NewDecoder(resp.Body).Decode(&user)

			if resp.StatusCode >= 400 {
				printOut(map[string]interface{}{
					"status":          "INVALID_KEY",
					"base_url":        baseURL,
					"base_url_source": baseURLSource,
					"api_key":         maskedKey,
				})
				return
			}

			username, _ := user["username"].(string)
			printOut(map[string]interface{}{
				"status":          "LOGGED_IN",
				"base_url":        baseURL,
				"base_url_source": baseURLSource,
				"api_key":         maskedKey,
				"username":        username,
			})
		},
	}
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
		newStatusCmd(),
		newTagsCmd(),
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
		newNotificationsCmd(),
		newNotificationReadCmd(),
		newNotificationReadAllCmd(),
		newInstallCmd(),
		newAdminCmd(),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
