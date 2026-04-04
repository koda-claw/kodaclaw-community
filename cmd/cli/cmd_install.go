package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type installedSkill struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	InstalledAt string `json:"installed_at"`
}

type installedRecords struct {
	Skills []installedSkill `json:"skills"`
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

			// Fetch current_version from API
			version := "unknown"
			vURL := baseURL + "/api/v1/public/skills/" + url.PathEscape(name)
			if vResp, vErr := httpClient.Get(vURL); vErr == nil {
				var info struct {
					CurrentVersion *string `json:"current_version"`
				}
				if json.NewDecoder(vResp.Body).Decode(&info) == nil && info.CurrentVersion != nil {
					version = *info.CurrentVersion
				}
				vResp.Body.Close()
			}

			// 4. Write installed.json record
			installedFile := func() string {
				h, _ := os.UserHomeDir()
				return filepath.Join(h, ".kodaclaw-community", "installed.json")
			}()
			var installed installedRecords
			if idata, err := os.ReadFile(installedFile); err == nil {
				json.Unmarshal(idata, &installed)
			}
			// Update or add
			found := false
			for i, s := range installed.Skills {
				if s.Name == name {
					installed.Skills[i].Version = version
					installed.Skills[i].InstalledAt = time.Now().Format(time.RFC3339)
					found = true
					break
				}
			}
			if !found {
				installed.Skills = append(installed.Skills, installedSkill{
					Name:        name,
					Version:     version,
					InstalledAt: time.Now().Format(time.RFC3339),
				})
			}
			if idata, err := json.MarshalIndent(installed, "", "  "); err == nil {
				os.MkdirAll(filepath.Dir(installedFile), 0700)
				os.WriteFile(installedFile, idata, 0644)
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

func newInstalledCmd() *cobra.Command {
	var workspaceDir string

	cmd := &cobra.Command{
		Use:   "installed",
		Short: "List skills installed from the community",
		Run: func(cmd *cobra.Command, args []string) {
			if workspaceDir == "" {
				workspaceDir = os.Getenv("KC_WORKSPACE")
				if workspaceDir == "" {
					home, _ := os.UserHomeDir()
					workspaceDir = filepath.Join(home, ".kodaclaw")
				}
			}

			// Read installed.json
			installFile := func() string {
				h, _ := os.UserHomeDir()
				return filepath.Join(h, ".kodaclaw-community", "installed.json")
			}()
			data, err := os.ReadFile(installFile)
			if err != nil {
				if isJSON() {
					outputJSON(installedRecords{Skills: []installedSkill{}})
					return
				}
				fmt.Println("尚未安装任何社区 skill")
				return
			}

			var records installedRecords
			if err := json.Unmarshal(data, &records); err != nil {
				exitErr(fmt.Sprintf("failed to parse installed.json: %v", err))
			}

			if isJSON() {
				if records.Skills == nil {
					records.Skills = []installedSkill{}
				}
				outputJSON(records)
				return
			}

			if len(records.Skills) == 0 {
				fmt.Println("尚未安装任何社区 skill")
				return
			}

			fmt.Printf("已安装的社区 skill (共 %d 个):\n", len(records.Skills))
			for i, s := range records.Skills {
				skillDir := filepath.Join(workspaceDir, "skills", s.Name, "SKILL.md")
				exists := "✅"
				if _, err := os.Stat(skillDir); os.IsNotExist(err) {
					exists = "⚠ 文件缺失"
				}
				fmt.Printf("  [%d] %s v%s — %s\n", i+1, s.Name, s.Version, exists)
				if s.InstalledAt != "" {
					fmt.Printf("       安装时间: %s\n", s.InstalledAt)
				}
			}
		},
	}
	cmd.Flags().StringVar(&workspaceDir, "workspace", "", "KodaClaw workspace directory")
	return cmd
}

// --- Update Command ---
func newUpdateCmd() *cobra.Command {
	var updateAll bool
	var workspaceDir string

	cmd := &cobra.Command{
		Use:   "update [skill-name]",
		Short: "Update installed skills to latest version",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if workspaceDir == "" {
				workspaceDir = os.Getenv("KC_WORKSPACE")
				if workspaceDir == "" {
					home, _ := os.UserHomeDir()
					workspaceDir = filepath.Join(home, ".kodaclaw")
				}
			}

			installFile := func() string {
				h, _ := os.UserHomeDir()
				return filepath.Join(h, ".kodaclaw-community", "installed.json")
			}()
			data, err := os.ReadFile(installFile)
			if err != nil {
				exitErr("尚未安装任何社区 skill，请先使用 install 命令安装")
			}

			var records installedRecords
			if err := json.Unmarshal(data, &records); err != nil {
				exitErr(fmt.Sprintf("failed to parse installed.json: %v", err))
			}

			baseURL := getBaseURL()

			// Filter which skills to update
			var toUpdate []struct {
				Name    string
				Version string
			}
			if updateAll {
				toUpdate = make([]struct{ Name, Version string }, len(records.Skills))
				for i, s := range records.Skills {
					toUpdate[i].Name = s.Name
					toUpdate[i].Version = s.Version
				}
			} else if len(args) > 0 {
				name := args[0]
				found := false
				for _, s := range records.Skills {
					if s.Name == name {
						toUpdate = append(toUpdate, struct{ Name, Version string }{Name: name, Version: s.Version})
						found = true
						break
					}
				}
				if !found {
					exitErr(fmt.Sprintf("skill '%s' 未安装", name))
				}
			} else {
				exitErr("请指定 skill 名称或使用 --all 更新全部")
			}

			updated := 0
			for _, s := range toUpdate {
				// Fetch latest skill info
				skillURL := baseURL + "/api/v1/public/skills/" + url.PathEscape(s.Name)
				resp, err := httpClient.Get(skillURL)
				if err != nil {
					fmt.Printf("  ⚠ %s: 获取失败 (%v)\n", s.Name, err)
					continue
				}

				var skillInfo struct {
					CurrentVersion string `json:"current_version"`
					Status         string `json:"status"`
				}
				if json.NewDecoder(resp.Body).Decode(&skillInfo) != nil {
					resp.Body.Close()
					fmt.Printf("  ⚠ %s: 解析失败\n", s.Name)
					continue
				}
				resp.Body.Close()

				if skillInfo.Status != "approved" {
					fmt.Printf("  ⏭ %s: 资产未通过审核\n", s.Name)
					continue
				}

				if skillInfo.CurrentVersion == s.Version {
					fmt.Printf("  ✅ %s: 已是最新 (v%s)\n", s.Name, s.Version)
					continue
				}

				// Download new SKILL.md
				contentURL := baseURL + "/api/v1/public/skills/" + url.PathEscape(s.Name) + "/SKILL.md"
				cResp, err := httpClient.Get(contentURL)
				if err != nil {
					fmt.Printf("  ⚠ %s: 下载失败 (%v)\n", s.Name, err)
					continue
				}
				newData, err := io.ReadAll(cResp.Body)
				cResp.Body.Close()
				if err != nil || len(newData) == 0 {
					fmt.Printf("  ⚠ %s: 内容为空\n", s.Name)
					continue
				}

				skillDir := filepath.Join(workspaceDir, "skills", s.Name)
				if err := os.MkdirAll(skillDir, 0755); err != nil {
					fmt.Printf("  ⚠ %s: 创建目录失败 (%v)\n", s.Name, err)
					continue
				}
				if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), newData, 0644); err != nil {
					fmt.Printf("  ⚠ %s: 写入失败 (%v)\n", s.Name, err)
					continue
				}

				// Update installed.json
				for i, rec := range records.Skills {
					if rec.Name == s.Name {
						records.Skills[i].Version = skillInfo.CurrentVersion
						records.Skills[i].InstalledAt = time.Now().Format(time.RFC3339)
						break
					}
				}

				fmt.Printf("  %s: v%s → v%s\n", s.Name, s.Version, skillInfo.CurrentVersion)
				updated++
			}

			if updated > 0 {
				// Save updated installed.json
				newData, _ := json.MarshalIndent(records, "", "  ")
				os.WriteFile(installFile, newData, 0644)
			}

			fmt.Printf("\n更新完成: %d 个 skill 已更新\n", updated)
		},
	}
	cmd.Flags().BoolVar(&updateAll, "all", false, "Update all installed skills")
	cmd.Flags().StringVar(&workspaceDir, "workspace", "", "KodaClaw workspace directory")
	return cmd
}

// --- Uninstall Command ---
func newUninstallCmd() *cobra.Command {
	var workspaceDir string

	cmd := &cobra.Command{
		Use:   "uninstall <skill-name>",
		Short: "Uninstall a community skill",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]

			if workspaceDir == "" {
				workspaceDir = os.Getenv("KC_WORKSPACE")
				if workspaceDir == "" {
					home, _ := os.UserHomeDir()
					workspaceDir = filepath.Join(home, ".kodaclaw")
				}
			}

			fmt.Printf("确定要卸载 %s 吗？将删除 skill 目录及其所有文件。\n", name)
			fmt.Print("输入 y 确认: ")
			var confirm string
			fmt.Scanln(&confirm)
			if confirm != "y" && confirm != "Y" {
				fmt.Println("已取消")
				return
			}

			skillDir := filepath.Join(workspaceDir, "skills", name)
			if err := os.RemoveAll(skillDir); err != nil {
				exitErr(fmt.Sprintf("删除失败: %v", err))
			}

			// Remove from installed.json
			installFile := func() string {
				h, _ := os.UserHomeDir()
				return filepath.Join(h, ".kodaclaw-community", "installed.json")
			}()
			data, err := os.ReadFile(installFile)
			if err == nil {
				var records installedRecords
				if json.Unmarshal(data, &records) == nil {
					filtered := records.Skills[:0]
					for _, s := range records.Skills {
						if s.Name != name {
							filtered = append(filtered, s)
						}
					}
					records.Skills = filtered
					newData, _ := json.MarshalIndent(records, "", "  ")
					os.WriteFile(installFile, newData, 0644)
				}
			}

			fmt.Printf("✅ 已卸载: %s\n", name)
		},
	}
	cmd.Flags().StringVar(&workspaceDir, "workspace", "", "KodaClaw workspace directory")
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
