package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/truongnhatanh7/xocker/internal/common"
	"github.com/truongnhatanh7/xocker/internal/container"
	"github.com/truongnhatanh7/xocker/internal/logger"
	"go.uber.org/zap"
)

var (
	rootfs      string
	interactive bool
	tty         bool
)

var runCmd = &cobra.Command{
	Use:  "run",
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		command := args[0]
		commandArgs := args[1:]

		logger.Log.Debug("rootfs", zap.String("rootfs", rootfs))
		logger.Log.Debug("interactive", zap.Bool("interactive", interactive))
		logger.Log.Debug("tty", zap.Bool("tty", tty))
		logger.Log.Debug("command", zap.String("command", command))
		logger.Log.Debug("commandArgs", zap.Strings("commandArgs", commandArgs))

		// process rootfs dir, "." doesn't work in some cases -> resolve to full path
		var flags []string

		cmd.Flags().Visit(func(f *pflag.Flag) {
			if f.Name == "rootfs" {
				absRootFS, err := filepath.Abs(f.Value.String())
				common.Must(err)
				flags = append(flags, fmt.Sprintf("--%s=%s", f.Name, absRootFS))
				return
			}
			flags = append(flags, fmt.Sprintf("--%s=%s", f.Name, f.Value.String()))
		})

		if err := container.RunContainer(&container.Container{
			Cmd:    command,
			Args:   commandArgs,
			RootFS: rootfs,
			Flags:  flags,
		}); err != nil {
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
