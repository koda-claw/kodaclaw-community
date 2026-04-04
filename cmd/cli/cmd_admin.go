package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

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

// fetchPendingVersions returns pending versions from the admin API.
func fetchPendingVersions(creds *credentials) ([]map[string]interface{}, int, error) {
	endpoint := creds.BaseURL + "/api/v1/admin/versions/pending?page_size=100"
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

// resolveVersionID maps a number (1-based index into pending versions list) or UUID string to a version ID.
// Returns (versionID, versionLabel, error).
func resolveVersionID(creds *credentials, arg string) (string, string, error) {
	idx, err := strconv.Atoi(arg)
	if err != nil {
		// Not a number — treat as UUID directly.
		return arg, "", nil
	}
	items, _, err := fetchPendingVersions(creds)
	if err != nil {
		return "", "", fmt.Errorf("获取待审核版本列表失败: %w", err)
	}
	if idx < 1 || idx > len(items) {
		return "", "", fmt.Errorf("编号 %d 超出范围（共 %d 个待审核版本）", idx, len(items))
	}
	item := items[idx-1]
	id, _ := item["id"].(string)
	assetID, _ := item["asset_id"].(string)
	version, _ := item["version"].(string)
	label := fmt.Sprintf("资产 %s v%s", assetID, version)
	return id, label, nil
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
			fmt.Println("\n💡 查看待审核版本: kc-community admin pending-versions")
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

	var showDetail bool
	pendingVersions := &cobra.Command{
		Use:   "pending-versions",
		Short: "List pending versions waiting for review",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds := mustLoadCreds()
			items, total, err := fetchPendingVersions(creds)
			if err != nil {
				return err
			}

			fmt.Printf("待审核版本 (共 %d 个):\n", total)
			for i, item := range items {
				assetID, _ := item["asset_id"].(string)
				version, _ := item["version"].(string)
				versionID, _ := item["id"].(string)
				changelog := ""
				if cl, ok := item["changelog"].(string); ok {
					changelog = cl
					if len([]rune(changelog)) > 80 {
						changelog = string([]rune(changelog)[:80]) + "..."
					}
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

				fmt.Printf("\n  [%d] 资产 %s v%s\n", i+1, assetID, version)
				fmt.Printf("      版本 ID: %s\n", versionID)
				if changelog != "" {
					fmt.Printf("      变更日志: %s\n", changelog)
				}
				if createdAt != "" {
					fmt.Printf("      提交时间: %s\n", createdAt)
				}
				if showDetail {
					skillContent := ""
					if sc, ok := item["skill_content"].(string); ok {
						skillContent = sc
					}
					if skillContent != "" {
						preview := skillContent
						if len([]rune(preview)) > 500 {
							preview = string([]rune(preview)[:500]) + "..."
						}
						fmt.Printf("      内容预览:\n%s\n", preview)
					}
				}
			}
			if total == 0 {
				fmt.Println("\n  （暂无待审核版本）")
			}
			return nil
		},
	}
	pendingVersions.Flags().BoolVar(&showDetail, "detail", false, "Show skill_content preview (first 500 chars)")

	approveVersion := &cobra.Command{
		Use:   "approve-version <version_id|number>",
		Short: "Approve a pending version (use number from 'admin pending-versions' or full UUID)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			creds := mustLoadCreds()
			versionID, label, err := resolveVersionID(creds, args[0])
			if err != nil {
				exitErr(err.Error())
			}
			resp, err := doJSON("POST", creds.BaseURL+"/api/v1/admin/versions/"+versionID+"/approve", creds.APIKey, nil)
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
			assetID, _ := result["asset_id"].(string)
			version, _ := result["version"].(string)
			if assetID != "" && version != "" {
				label = fmt.Sprintf("资产 %s v%s (ID: %s)", assetID, version, versionID)
			} else if label == "" {
				label = fmt.Sprintf("(ID: %s)", versionID)
			}
			fmt.Printf("已审核通过版本: %s\n", label)
		},
	}

	var rejectVersionReason string
	rejectVersion := &cobra.Command{
		Use:   "reject-version <version_id|number>",
		Short: "Reject a pending version (use number from 'admin pending-versions' or full UUID)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			creds := mustLoadCreds()
			versionID, label, err := resolveVersionID(creds, args[0])
			if err != nil {
				exitErr(err.Error())
			}
			body := map[string]string{"reason": rejectVersionReason}
			resp, err := doJSON("POST", creds.BaseURL+"/api/v1/admin/versions/"+versionID+"/reject", creds.APIKey, body)
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
			assetID, _ := result["asset_id"].(string)
			version, _ := result["version"].(string)
			if assetID != "" && version != "" {
				label = fmt.Sprintf("资产 %s v%s (ID: %s)", assetID, version, versionID)
			} else if label == "" {
				label = fmt.Sprintf("(ID: %s)", versionID)
			}
			fmt.Printf("已拒绝版本: %s\n原因: %s\n", label, rejectVersionReason)
		},
	}
	rejectVersion.Flags().StringVar(&rejectVersionReason, "reason", "", "Rejection reason (required)")
	_ = rejectVersion.MarkFlagRequired("reason")

	stats := &cobra.Command{
		Use:   "stats",
		Short: "Show platform statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds := mustLoadCreds()
			resp, err := doJSON("GET", creds.BaseURL+"/api/v1/admin/stats", creds.APIKey, nil)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return fmt.Errorf("failed to decode response: %w", err)
			}
			if resp.StatusCode >= 400 {
				enc := json.NewEncoder(os.Stderr)
				enc.SetIndent("", "  ")
				_ = enc.Encode(result)
				os.Exit(1)
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		},
	}

	var auditPage, auditPageSize int
	auditCmd := &cobra.Command{
		Use:   "audit",
		Short: "List admin audit logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds := mustLoadCreds()
			endpoint := fmt.Sprintf("%s/api/v1/admin/audit?page=%d&page_size=%d", creds.BaseURL, auditPage, auditPageSize)
			resp, err := doJSON("GET", endpoint, creds.APIKey, nil)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return fmt.Errorf("failed to decode response: %w", err)
			}
			if resp.StatusCode >= 400 {
				enc := json.NewEncoder(os.Stderr)
				enc.SetIndent("", "  ")
				_ = enc.Encode(result)
				os.Exit(1)
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		},
	}
	auditCmd.Flags().IntVar(&auditPage, "page", 1, "Page number")
	auditCmd.Flags().IntVar(&auditPageSize, "page-size", 20, "Items per page")

	adminCmd.AddCommand(pending, approve, reject, pendingVersions, approveVersion, rejectVersion, stats, auditCmd)
	return adminCmd
}
