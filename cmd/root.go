package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/truongnhatanh7/xocker/internal/logger"
)

var logLevel string

var rootCmd = &cobra.Command{
	Use: "xocker",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return logger.Init(logLevel)
	},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("this is xocker")
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Printf("error exec root cmd %v", err)
		os.Exit(1)
	}
}

func init() {
	// init flags here
	rootCmd.PersistentFlags().StringVar(
		&logLevel, "level", "dev", "log level",
	)
}
