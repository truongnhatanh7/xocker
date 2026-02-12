package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "",
	Short: "",
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
}
