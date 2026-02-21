package network

import (
	"fmt"
	"os/exec"

	"github.com/truongnhatanh7/xocker/internal/logger"
	"go.uber.org/zap"
)

func ConfigureContainerNetwork(vethName, ipWithCIDR, gatewayIP string) error {
	logger.Log.Debug("configuring container network",
		zap.String("veth", vethName),
		zap.String("ip", ipWithCIDR),
		zap.String("gateway", gatewayIP))

	cmd := exec.Command("ip", "addr", "add", ipWithCIDR, "dev", vethName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to assign IP %s to %s: %w, output: %s", ipWithCIDR, vethName, err, string(output))
	}
	logger.Log.Debug("assigned IP to veth", zap.String("veth", vethName), zap.String("ip", ipWithCIDR))

	cmd = exec.Command("ip", "link", "set", vethName, "up")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring up %s: %w, output: %s", vethName, err, string(output))
	}
	logger.Log.Debug("brought up veth interface", zap.String("veth", vethName))

	cmd = exec.Command("ip", "link", "set", "lo", "up")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring up loopback interface: %w, output: %s", err, string(output))
	}
	logger.Log.Debug("brought up loopback interface")

	cmd = exec.Command("ip", "route", "add", "default", "via", gatewayIP)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add default route via %s: %w, output: %s", gatewayIP, err, string(output))
	}
	logger.Log.Debug("added default route", zap.String("gateway", gatewayIP))

	logger.Log.Info("container network configured successfully",
		zap.String("veth", vethName),
		zap.String("ip", ipWithCIDR),
		zap.String("gateway", gatewayIP))

	return nil
}
