package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/truongnhatanh7/xocker/internal/cgroupv2"
	"github.com/truongnhatanh7/xocker/internal/common"
	"github.com/truongnhatanh7/xocker/internal/container"
	"github.com/truongnhatanh7/xocker/internal/logger"
	"go.uber.org/zap"
)

var (
	rootfs      string
	interactive bool
	cpu         uint64
	mem         uint64
)

var runCmd = &cobra.Command{
	Use:  "run",
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		command := args[0]
		commandArgs := args[1:]

		logger.Log.Debug("rootfs", zap.String("rootfs", rootfs))
		logger.Log.Debug("interactive", zap.Bool("interactive", interactive))
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
			Cmd:         command,
			Args:        commandArgs,
			RootFS:      rootfs,
			Flags:       flags,
			Interactive: interactive,
			CPUQuota:    cpu,
			Mem:         mem,
		}); err != nil {
			os.Exit(1)
		}
	},
}

func init() {
	runCmd.Flags().StringVar(&rootfs, "rootfs", "", "Path to the root filesystem")
	// for simplicity: handle both stdin and tty, instead of 2 flags -i and -t
	runCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode")
	runCmd.Flags().Uint64VarP(&cpu, "cpu", "c", cgroupv2.HALF_CPU_QUOTA, "CPU quota (CPUQuotaPerSecUSec)")
	runCmd.Flags().Uint64VarP(&mem, "mem", "m", 128, "Mem limit")

	rootCmd.AddCommand(runCmd)
}
