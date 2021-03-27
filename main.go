package main

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/bantl23/yabba/cmd"
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
}

func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	return cmd.Execute()
}
