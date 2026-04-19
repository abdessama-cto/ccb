package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the ccb version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ccb %s · %s/%s\n", Version, runtime.GOOS, hostArch())
		fmt.Println("Repo: https://github.com/abdessama-cto/ccb")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
