package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use: "version",
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("version 0.0.0")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
