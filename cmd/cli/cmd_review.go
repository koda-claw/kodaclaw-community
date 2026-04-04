package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

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
