package main

import (
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	root := &cobra.Command{
		Use:   "kc-community",
		Short: "KodaClaw Community CLI",
		Long: "CLI tool for interacting with the KodaClaw Community API.\n\nBase URL can be overridden via KC_COMMUNITY_URL environment variable.",
		Version: version,
	}

	root.SetVersionTemplate("kc-community version {{.Version}}\n")

	root.PersistentFlags().BoolVarP(&jsonMode, "json", "j", false, "Output in JSON format")

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
