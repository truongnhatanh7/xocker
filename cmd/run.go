package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	rootfs      string
	interactive bool
	tty         bool
)

var runCmd = &cobra.Command{
	Use: "run [flags] command [args...]",
	Run: func(cmd *cobra.Command, args []string) {
		command := args[0]
		commandArgs := args[1:]

		fmt.Printf("Rootfs: %s\n", rootfs)
		fmt.Printf("Interactive: %v\n", interactive)
		fmt.Printf("TTY: %v\n", tty)
		fmt.Printf("Command: %s\n", command)
		fmt.Printf("Args: %v\n", commandArgs)
	},
}

func init() {
	runCmd.Flags().StringVar(&rootfs, "rootfs", "", "Path to the root filesystem")
	runCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Keep STDIN open")
	runCmd.Flags().BoolVarP(&tty, "tty", "t", false, "Allocate a pseudo-TTY")

	rootCmd.AddCommand(runCmd)
}
