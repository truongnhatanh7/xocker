package container

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/truongnhatanh7/xocker/internal/common"
	"github.com/truongnhatanh7/xocker/internal/logger"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

type Container struct {
	Cmd    string
	Args   []string
	RootFS string
	Flags  []string
}

func RunContainer(container *Container) error {
	if container == nil {
		panic("cotainer is nil")
	}

	if os.Getenv("_IN_CONTAINER") == "1" {
		return handleChild(container)
	}

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

	os.Setenv("_IN_CONTAINER", "1")
	c.Env = os.Environ()
	common.Must(c.Run())

	return nil
}

func handleChild(container *Container) error {
	// mount nescessay stuff
	//
	// rootfs must be a mount point
	// mount proc - pseudo filesystem
	// mount tmpfs - dev
	common.Must(syscall.Mount("proc", container.RootFS+"/proc", "proc", 0, ""))
	common.Must(syscall.Mount("tmpfs", container.RootFS+"/dev", "tmpfs", 0, ""))
	// Best-effort minimal device nodes (requires CAP_MKNOD; may fail rootless)
	common.Must(mknodChar(container.RootFS+"/dev/null", 0o666, 1, 3))
	common.Must(mknodChar(container.RootFS+"/dev/zero", 0o666, 1, 5))
	common.Must(mknodChar(container.RootFS+"/dev/random", 0o666, 1, 8))
	common.Must(mknodChar(container.RootFS+"/dev/urandom", 0o666, 1, 9))
	logger.Log.Debug("done mounting")

	// pivot root
	// make newroot a mountpoint
	//
	_, err := os.Stat(container.RootFS)
	common.Must(err)
	unix.Mount(container.RootFS, container.RootFS, "", unix.MS_BIND|unix.MS_REC, "")
	common.Must(os.MkdirAll(container.RootFS+"/old_root", 0o777))
	common.Must(unix.PivotRoot(container.RootFS, container.RootFS+"/old_root"))
	common.Must(os.Chdir("/"))
	common.Must(unix.Unmount("./old_root", syscall.MNT_DETACH))
	logger.Log.Debug("done pivot root")

	// actually exec input command
	argv := []string{container.Cmd}
	argv = append(argv, container.Args...)
	logger.Log.Debug("argv", zap.Strings("argv", argv))
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
