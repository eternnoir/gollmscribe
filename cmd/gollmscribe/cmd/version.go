package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	version   = "0.2.0"
	gitCommit = "dev"
	buildDate = "unknown"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display version information for gollmscribe`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("gollmscribe version %s\n", version)
		fmt.Printf("Git commit: %s\n", gitCommit)
		fmt.Printf("Build date: %s\n", buildDate)
		fmt.Printf("Go version: %s\n", runtime.Version())
		fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
