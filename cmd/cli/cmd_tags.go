package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

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
