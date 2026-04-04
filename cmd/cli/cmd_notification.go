package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/spf13/cobra"
)

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
					ID        string `json:"id"`
					Type      string `json:"type"`
					Title     string `json:"title"`
					IsRead    bool   `json:"is_read"`
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
