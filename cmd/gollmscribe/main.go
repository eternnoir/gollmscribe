package main

import (
	"os"

	"github.com/eternnoir/gollmscribe/cmd/gollmscribe/cmd"
	"github.com/eternnoir/gollmscribe/pkg/logger"
)

func main() {
	if err := cmd.Execute(); err != nil {
		logger.Error().Err(err).Msg("Application execution failed")
		os.Exit(1)
	}
}
