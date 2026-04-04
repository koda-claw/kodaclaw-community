package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

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

func newWhoamiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Show current user info and permissions",
		Run: func(cmd *cobra.Command, args []string) {
			creds := mustLoadCreds()

			resp, err := doJSON("GET", creds.BaseURL+"/api/v1/users/me", creds.APIKey, nil)
			if err != nil {
				exitErr(err.Error())
			}
			handleResp(resp)
		},
	}
	return cmd
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
