package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	var showVersion bool

	root := &cobra.Command{
		Use:   "kc-community",
		Short: "KodaClaw Community CLI",
		Long:  "CLI tool for interacting with the KodaClaw Community API.\n\nBase URL can be overridden via KC_COMMUNITY_URL environment variable.",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if showVersion {
				fmt.Println("kc-community version " + version)
				os.Exit(0)
			}
		},
	}

	root.PersistentFlags().BoolVar(&showVersion, "version", false, "Print version and exit")

	root.AddCommand(
		newLoginCmd(),
		newLogoutCmd(),
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
		newInstalledCmd(),
		newUpdateCmd(),
		newUninstallCmd(),
		newMyAssetsCmd(),
		newDeleteCmd(),
		newEditCmd(),
		newVersionsCmd(),
		newWhoamiCmd(),
		newAdminCmd(),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
