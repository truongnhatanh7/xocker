package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/truongnhatanh7/xocker/internal/container"
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

		if err := container.RunContainer(commandArgs); err != nil {
			os.Exit(1)
		}
	},
}

func init() {
	runCmd.Flags().StringVar(&rootfs, "rootfs", "", "Path to the root filesystem")
	runCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Keep STDIN open")
	runCmd.Flags().BoolVarP(&tty, "tty", "t", false, "Allocate a pseudo-TTY")

	rootCmd.AddCommand(runCmd)
}
