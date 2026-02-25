package cmd

import (
	"github.com/bantl23/yabba/run"
	"github.com/spf13/cobra"
)

var (
	listenAddr string
	listenSize int
)

func init() {
	listenCmd.Flags().StringVarP(&listenAddr, "addr", "a", ":5201", "bind address")
	listenCmd.Flags().IntVarP(&listenSize, "size", "s", 128*1024, "buffer size")
	rootCmd.AddCommand(listenCmd)
}

var listenCmd = &cobra.Command{
	Use:   "listen",
	Short: "Listens for clients",
	RunE: func(cmd *cobra.Command, args []string) error {
		server := run.Server{
			Address: listenAddr,
			Size:    listenSize,
		}
		return server.Run()
	},
}
