package main

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/user/doris-export-go/internal/cli"
)

func main() {
	// Initialize logger
	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Create and execute root command
	rootCmd := cli.NewRootCommand()

	if err := rootCmd.Execute(); err != nil {
		log.Errorf("Error: %v", err)
		os.Exit(1)
	}
}
