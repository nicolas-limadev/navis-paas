package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print NavisPaaS CLI version",
	Long:  `Print the version of NavisPaaS CLI.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("NavisPaaS CLI v%s\n", version)
		fmt.Printf("Git commit: %s\n", "latest")
		fmt.Printf("Go version: %s\n", "1.24")
	},
}
