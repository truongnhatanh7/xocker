package container

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/truongnhatanh7/xocker/internal/cgroupv2"
	"github.com/truongnhatanh7/xocker/internal/common"
	"github.com/truongnhatanh7/xocker/internal/logger"
	"github.com/truongnhatanh7/xocker/internal/network"
	"github.com/truongnhatanh7/xocker/internal/sync"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

type Container struct {
	Cmd         string
	Args        []string
	RootFS      string
	Flags       []string
	Interactive bool
	CPUQuota    uint64
	Mem         uint64
}

func RunContainer(container *Container) error {
	if container == nil {
		panic("cotainer is nil")
	}

	if os.Getenv("_IN_CONTAINER") == "1" {
		if err := handleChild(container); err != nil {
			logger.Log.Error("handleChild failed", zap.Error(err))
			return err
		}
		return nil
	}

	// should be ran via hook or separated cmd, for learning purpose -> create bridge here
	network.CreateBridge()

	// Initialize IP state file with base IP
	common.Must(network.InitIPState("./ip.state"))

	// Create socketpair for parent-child synchronization
	parentConn, childConn, err := sync.CreateSocketPair()
	common.Must(err)
	defer parentConn.Close()

	// process rootfs dir, "." doesn't work in some cases -> resolve to full path
	absRootFS, err := filepath.Abs(container.RootFS)
	common.Must(err)
	container.RootFS = absRootFS

	self, err := os.Executable()
	common.Must(err)

	// ensure rootfs exists
	_, err = os.Stat(container.RootFS)
	common.Must(err)

	// check ps aux count before create ns
	checkPsAuxCount()

	unshareCmd := []string{
		"unshare",
		"--mount",
		"--uts",
		"--ipc",
		"--net",
		"--pid",
		"--fork",
		"--mount-proc",
	}

	cCmd := append(
		unshareCmd,
		"--",
		self, "run",
	)
	cCmd = append(cCmd, container.Flags...)
	cCmd = append(cCmd, "--")
	cCmd = append(cCmd, container.Cmd)
	cCmd = append(cCmd, container.Args...)

	// spawn new ns
	c := exec.Command(
		cCmd[0],
		cCmd[1:]...,
	)
	logger.Log.Debug("c command", zap.String("c", c.String()))

	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	// Pass child side of socketpair to child process via ExtraFiles
	// The child will access it as fd 3
	c.ExtraFiles = []*os.File{childConn}

	os.Setenv("_IN_CONTAINER", "1")
	c.Env = os.Environ()

	common.Must(c.Start())

	// Close child conn in parent (child has its own copy)
	childConn.Close()

	realPid, err := waitForChildPID(c.Process.Pid, 3*time.Second) // choose an appropriate timeout
	if err != nil {
		// best effort fallback: if we cannot find child, kill and return error
		c.Process.Kill()
		common.Must(fmt.Errorf("could not find forked child of unshare (%d): %w", c.Process.Pid, err))
	}
	logger.Log.Debug("realpid", zap.Int("pid", realPid))

	// Set up container networking from parent (host namespace)
	containerIP, vethName, err := network.CreateVethAndAttachToBridge(realPid)
	if err != nil {
		c.Process.Kill()
		common.Must(fmt.Errorf("failed to set up container network: %w", err))
	}
	logger.Log.Info("network configured for container",
		zap.String("ip", containerIP),
		zap.String("veth", vethName),
		zap.Int("pid", realPid))

	// Prepare network configuration to send to child
	networkConfig := fmt.Sprintf("%s\n%s\n%s", containerIP, vethName, "172.18.0.1")

	// Signal child that network is ready and send config
	if err := sync.SignalReady(parentConn, networkConfig); err != nil {
		c.Process.Kill()
		common.Must(fmt.Errorf("failed to signal child: %w", err))
	}
	logger.Log.Debug("signaled child that network is ready")

	// Set up cgroups
	cg := cgroupv2.NewCgroupV2("container_" + time.Now().Format(time.RFC3339Nano))
	cg.Limit(&cgroupv2.CgroupV2SetSpecs{
		ApplyToPid: realPid,
		CPUSpec: &cgroupv2.CPUSpec{
			Quota: container.CPUQuota,
		},
		MemSpec: &cgroupv2.MemSpec{
			Limit: container.Mem,
		},
	})
	defer cg.Destroy()

	if err := c.Wait(); err != nil {
		logger.Log.Error("container process exited with error", zap.Error(err))
		// Clean up IP before returning error
		justIP := strings.Split(containerIP, "/")[0]
		network.ReleaseIP("./ip.state", justIP)
		return fmt.Errorf("container exited with error: %w", err)
	}

	// Clean up IP allocation when container exits
	justIP := strings.Split(containerIP, "/")[0]
	if err := network.ReleaseIP("./ip.state", justIP); err != nil {
		logger.Log.Warn("failed to release IP", zap.String("ip", justIP), zap.Error(err))
	}

	return nil
}

func handleChild(container *Container) error {
	childConn := os.NewFile(uintptr(3), "sync-pipe")
	if childConn == nil {
		return fmt.Errorf("failed to get sync connection from fd 3")
	}
	defer childConn.Close()

	myPid := os.Getpid()
	logger.Log.Debug("child process started", zap.Int("pid", myPid))

	time.Sleep(100 * time.Millisecond)

	mergedRootFS := container.RootFS + "/../merged"

	common.Must(os.MkdirAll(container.RootFS+"/../merged", 0755))
	common.Must(os.MkdirAll(container.RootFS+"/../overlay/upper", 0755))
	common.Must(os.MkdirAll(container.RootFS+"/../overlay/work", 0755))
	lower := container.RootFS
	upper := container.RootFS + "/../overlay/upper"
	work := container.RootFS + "/../overlay/work"
	opts := "lowerdir=" + lower + ",upperdir=" + upper + ",workdir=" + work
	common.Must(syscall.Mount("overlay", mergedRootFS, "overlay", 0, opts))

	common.Must(os.MkdirAll(mergedRootFS+"/proc", 0755))
	common.Must(syscall.Mount("proc", mergedRootFS+"/proc", "proc", 0, ""))

	common.Must(os.MkdirAll(mergedRootFS+"/dev", 0755))
	common.Must(syscall.Mount("tmpfs", mergedRootFS+"/dev", "tmpfs", 0, ""))

	common.Must(os.MkdirAll(mergedRootFS+"/dev/pts", 0755))
	common.Must(
		syscall.Mount(
			"devpts",
			mergedRootFS+"/dev/pts",
			"devpts",
			syscall.MS_NOSUID|syscall.MS_NOEXEC,
			"newinstance,ptmxmode=0666,mode=620,gid=5",
		),
	)
	common.Must(mknodChar(mergedRootFS+"/dev/tty", 0o666, 5, 0))
	common.Must(mknodChar(mergedRootFS+"/dev/ptmx", 0o666, 5, 2))

	common.Must(mknodChar(mergedRootFS+"/dev/null", 0o666, 1, 3))
	common.Must(mknodChar(mergedRootFS+"/dev/zero", 0o666, 1, 5))
	common.Must(mknodChar(mergedRootFS+"/dev/random", 0o666, 1, 8))
	common.Must(mknodChar(mergedRootFS+"/dev/urandom", 0o666, 1, 9))

	logger.Log.Debug("done mounting")

	logger.Log.Debug("child waiting for network setup signal from parent")
	networkConfig, err := sync.WaitForReady(childConn, 10*time.Second)
	if err != nil {
		return fmt.Errorf("timeout waiting for network setup: %w", err)
	}
	logger.Log.Debug("received network ready signal from parent")

	configLines := strings.Split(strings.TrimSpace(networkConfig), "\n")
	if len(configLines) != 3 {
		return fmt.Errorf("invalid network config format, expected 3 lines, got %d", len(configLines))
	}

	containerIP := configLines[0]
	vethName := configLines[1]
	gatewayIP := configLines[2]

	logger.Log.Debug("network config received",
		zap.String("ip", containerIP),
		zap.String("veth", vethName),
		zap.String("gateway", gatewayIP))

	// Configure network inside container namespace
	if err := network.ConfigureContainerNetwork(vethName, containerIP, gatewayIP); err != nil {
		return fmt.Errorf("failed to configure container network: %w", err)
	}

	// pivot root
	// make newroot a mountpoint
	//
	_, err = os.Stat(container.RootFS)
	common.Must(err)
	unix.Mount(mergedRootFS, mergedRootFS, "", unix.MS_BIND|unix.MS_REC, "")
	common.Must(os.MkdirAll(container.RootFS+"/../merged/old_root", 0o777))
	common.Must(unix.PivotRoot(container.RootFS+"/../merged", container.RootFS+"/../merged/old_root"))
	common.Must(os.Chdir("/"))
	common.Must(unix.Unmount("./old_root", syscall.MNT_DETACH))
	logger.Log.Debug("done pivot root")

	// actually exec input command
	argv := []string{container.Cmd}
	argv = append(argv, container.Args...)
	logger.Log.Debug("argv", zap.Strings("argv", argv))

	// rename hostname of container
	chc := exec.Command("hostname", "container_"+time.Now().Format(time.RFC3339Nano))
	common.Must(chc.Start())

	if container.Interactive {
		// use exec command insteaqd of syscall.Exec to maintain connection
		// with go runtime, cuz we're setting up pty master-slave, ...
		cmd := exec.Command(container.Cmd)

		// start pty
		ptmx, err := pty.Start(cmd)
		common.Must(err)
		defer ptmx.Close()

		// turn off canonical mode
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		common.Must(err)
		defer term.Restore(int(os.Stdin.Fd()), oldState)

		// handle resize -> propagate resize events
		go func() {
			ch := make(chan os.Signal, 1)
			signal.Notify(ch, syscall.SIGWINCH)

			go func() {
				for range ch {
					pty.InheritSize(os.Stdin, ptmx)
				}
			}()
			ch <- syscall.SIGWINCH
		}()

		// copy io
		go func() {
			//user input -> master
			io.Copy(ptmx, os.Stdin)
		}()
		// master output -> stdout
		io.Copy(os.Stdout, ptmx)

		// call wait to properly clean up
		common.Must(cmd.Wait())

		return nil
	}
	common.Must(syscall.Exec(container.Cmd, argv, []string{}))

	return nil
}

func checkPsAuxCount() {
	ps := exec.Command("ps", "aux")
	wc := exec.Command("wc", "-l")

	// Pipe ps output into wc input
	pipe, err := ps.StdoutPipe()
	common.Must(err)
	wc.Stdin = pipe

	common.Must(ps.Start())

	output, err := wc.Output()
	common.Must(err)

	common.Must(ps.Wait())

	logger.Log.Debug("ps aux", zap.String("count", string(output)))
}

func checkPsAuxCountFull() {
	ps := exec.Command("ps", "aux")

	out, err := ps.Output()
	if err != nil {
		panic(err)
	}

	logger.Log.Debug("px aux full", zap.String("out", string(out)))
}

func mknodChar(path string, perm uint32, major, minor uint32) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	} else {
		logger.Log.Debug("mknodChar", zap.Error(err))
	}

	logger.Log.Debug("creating dir", zap.String("path", path))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	mode := uint32(unix.S_IFCHR) | perm
	dev := int(unix.Mkdev(major, minor))
	if err := unix.Mknod(path, mode, dev); err != nil {
		return err
	}

	return os.Chmod(path, os.FileMode(perm))
}

func waitForChildPID(unsharePid int, timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	for {
		pid, err := getChildPID(unsharePid)
		if err == nil {
			return pid, nil
		}
		// if children file exists but is empty, retry until timeout
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("timed out waiting for child of %d: last error: %w", unsharePid, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
func getChildPID(unsharePid int) (int, error) {
	// path like: /proc/1234/task/1234/children
	path := fmt.Sprintf("/proc/%d/task/%d/children", unsharePid, unsharePid)
	b, err := os.ReadFile(path)
	common.Must(err)

	s := strings.TrimSpace(string(b))
	if s == "" {
		return 0, fmt.Errorf("no children for %d", unsharePid)
	}
	parts := strings.Fields(s)
	pid, err := strconv.Atoi(parts[0])
	common.Must(err)

	return pid, nil
}
