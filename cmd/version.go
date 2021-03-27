package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	Version = "v0.0.0"
	Hash    = "n/a"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s (%s)\n", Version, Hash)
	},
}
