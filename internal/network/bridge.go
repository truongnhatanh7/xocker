package network

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/truongnhatanh7/xocker/internal/logger"
	"go.uber.org/zap"
)

var (
	bridge    = "xocker0"
	networkIP = "172.18.0.0"
	bridgeIP  = "172.18.0.1/24"
)

func CreateBridge() {
	if err := exec.Command("ip", "link", "show", bridge).Run(); err == nil {
		logger.Log.Debug("bridge already exists, ensuring it's up", zap.String("bridge", bridge))
		if err := exec.Command("ip", "link", "set", bridge, "up").Run(); err != nil {
			logger.Log.Error("failed to bring existing bridge up", zap.Error(err))
		}
		return
	}

	logger.Log.Info("creating bridge", zap.String("bridge", bridge))
	if err := exec.Command("ip", "link", "add", "name", bridge, "type", "bridge").Run(); err != nil {
		logger.Log.Error("failed to create bridge", zap.Error(err))
		return
	}

	if err := exec.Command("ip", "addr", "add", bridgeIP, "dev", bridge).Run(); err != nil {
		logger.Log.Warn("warning: ip addr add may have failed (might already have this IP)", zap.Error(err))
	}

	if err := exec.Command("ip", "link", "set", bridge, "up").Run(); err != nil {
		logger.Log.Error("failed to bring bridge up", zap.Error(err))
		return
	}

	logger.Log.Info("bridge created and configured successfully", zap.String("bridge", bridge), zap.String("ip", bridgeIP))
}

func InitIPState(filename string) error {
	existingIPs, err := readIPs(filename)
	if err != nil {
		return fmt.Errorf("failed to read IP state file: %w", err)
	}

	if len(existingIPs) > 0 {
		logger.Log.Debug("IP state file already initialized", zap.Int("ipCount", len(existingIPs)))
		return nil
	}

	baseIP := "172.18.0.1"
	if err := appendIP(filename, baseIP); err != nil {
		return fmt.Errorf("failed to initialize IP state with base IP: %w", err)
	}

	logger.Log.Info("initialized IP state file", zap.String("baseIP", baseIP), zap.String("file", filename))
	return nil
}

func ReleaseIP(filename, ip string) error {
	logger.Log.Debug("releasing IP", zap.String("ip", ip))

	if err := removeIP(filename, ip); err != nil {
		return fmt.Errorf("failed to release IP %s: %w", ip, err)
	}

	logger.Log.Info("released IP", zap.String("ip", ip))
	return nil
}

func randomHex(n int) string {
	bytes := make([]byte, n/2)
	_, err := rand.Read(bytes)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}

func CreateVethAndAttachToBridge(pid int) (string, string, error) {
	hostVeth := fmt.Sprintf("vethh%s", randomHex(6))
	contVeth := fmt.Sprintf("vethc%s", randomHex(6))

	logger.Log.Debug("creating veth pair",
		zap.String("hostVeth", hostVeth),
		zap.String("contVeth", contVeth),
		zap.Int("pid", pid))

	existingIPs, err := readIPs("./ip.state")
	if err != nil {
		return "", "", fmt.Errorf("failed to read IP state: %w", err)
	}

	contIP, err := nextAvailableIP(existingIPs)
	if err != nil {
		return "", "", fmt.Errorf("failed to allocate IP: %w", err)
	}

	logger.Log.Debug("allocated IP for container", zap.String("ip", contIP))

	if err := exec.Command("ip", "link", "add", hostVeth, "type", "veth", "peer", "name", contVeth).Run(); err != nil {
		return "", "", fmt.Errorf("failed to create veth pair: %w", err)
	}

	if err := exec.Command("ip", "link", "set", hostVeth, "master", bridge).Run(); err != nil {
		return "", "", fmt.Errorf("failed to attach veth to bridge: %w", err)
	}

	if err := exec.Command("ip", "link", "set", hostVeth, "up").Run(); err != nil {
		return "", "", fmt.Errorf("failed to set host veth up: %w", err)
	}

	if err := exec.Command("ip", "link", "set", contVeth, "netns", strconv.Itoa(pid)).Run(); err != nil {
		return "", "", fmt.Errorf("failed to move veth to netns: %w", err)
	}

	if err := appendIP("./ip.state", contIP); err != nil {
		return "", "", fmt.Errorf("failed to save IP to state: %w", err)
	}

	logger.Log.Info("veth pair created and attached",
		zap.String("hostVeth", hostVeth),
		zap.String("contVeth", contVeth),
		zap.String("containerIP", contIP),
		zap.Int("pid", pid))

	return contIP + "/24", contVeth, nil
}

func readIPs(filename string) ([]string, error) {
	// Open file with create flag
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var ips []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			ips = append(ips, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return ips, nil
}

func appendIP(filename, ip string) error {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(ip + "\n")
	return err
}

func removeIP(filename, ipToRemove string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	var filtered []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && line != ipToRemove {
			filtered = append(filtered, line)
		}
	}

	// Rewrite file
	output := strings.Join(filtered, "\n")
	if len(filtered) > 0 {
		output += "\n"
	}

	return os.WriteFile(filename, []byte(output), 0644)
}

func nextAvailableIP(existing []string) (string, error) {
	if len(existing) == 0 {
		return "", fmt.Errorf("no base subnet found")
	}

	// Use first IP to determine subnet base
	firstIP := net.ParseIP(existing[0])
	if firstIP == nil {
		return "", fmt.Errorf("invalid IP in list")
	}

	base := firstIP.To4()
	if base == nil {
		return "", fmt.Errorf("not IPv4")
	}

	subnetPrefix := fmt.Sprintf("%d.%d.%d.", base[0], base[1], base[2])

	used := make(map[int]bool)

	for _, ipStr := range existing {
		ip := net.ParseIP(ipStr).To4()
		if ip == nil {
			continue
		}
		used[int(ip[3])] = true
	}

	// Search usable host range 1-254
	for i := 1; i <= 254; i++ {
		if !used[i] {
			return subnetPrefix + strconv.Itoa(i), nil
		}
	}

	return "", fmt.Errorf("no available IPs")
}
