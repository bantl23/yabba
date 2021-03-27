package cmd

import (
	"time"

	"github.com/bantl23/yabba/run"
	"github.com/spf13/cobra"
)

var (
	connectAddrs    []string
	connections     int
	connectSize     int
	connectDuration time.Duration
)

func init() {
	connectCmd.Flags().StringSliceVarP(&connectAddrs, "addrs", "a", []string{"localhost:5201"}, "connect address(es)")
	connectCmd.Flags().IntVarP(&connections, "connections", "c", 1, "parallel connections")
	connectCmd.Flags().DurationVarP(&connectDuration, "duration", "d", 10*time.Second, "duration")
	connectCmd.Flags().IntVarP(&connectSize, "size", "s", 128*1024, "buffer size")
	rootCmd.AddCommand(connectCmd)
}

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connects to listeners",
	Run: func(cmd *cobra.Command, args []string) {
		client := run.Client{
			Addresses:   connectAddrs,
			Connections: connections,
			Duration:    connectDuration,
			Size:        connectSize,
		}
		client.Run()
	},
}
