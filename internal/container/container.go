package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/truongnhatanh7/xocker/internal/common"
	"golang.org/x/sys/unix"
)

type Container struct {
	Cmd    string
	Args   []string
	RootFS string
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
	fmt.Println("self", self)

	// ensure rootfs exists
	if _, err := os.Stat(container.RootFS); err != nil {
		fmt.Println(err)
		return err
	}

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
		unshareCmd, "--",
		self, "run", "--rootfs="+fmt.Sprintf("%s", container.RootFS), container.Cmd, "--",
	)
	cCmd = append(cCmd, container.Args...)

	// spawn new ns
	c := exec.Command(
		cCmd[0],
		cCmd[1:]...,
	)
	fmt.Printf("c %s\n", c.String())

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

	fmt.Println("done mounting")

	// pivot root
	// make newroot a mountpoint
	//
	fmt.Println("rootfs", container.RootFS)
	_, err := os.Stat(container.RootFS)
	common.Must(err)
	unix.Mount(container.RootFS, container.RootFS, "", unix.MS_BIND|unix.MS_REC, "")
	common.Must(os.MkdirAll(container.RootFS+"/old_root", 0o777))
	common.Must(unix.PivotRoot(container.RootFS, container.RootFS+"/old_root"))
	common.Must(os.Chdir("/"))
	common.Must(unix.Unmount("./old_root", syscall.MNT_DETACH))
	fmt.Println("done pivot root")

	// actually exec input command
	argv := []string{container.Cmd}
	argv = append(argv, container.Args...)
	fmt.Printf("argv %#v\n===========\n\n", argv)
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

	fmt.Println("Process count:", string(output))
}

func checkPsAuxCountFull() {
	ps := exec.Command("ps", "aux")

	out, err := ps.Output()
	if err != nil {
		panic(err)
	}

	fmt.Println(out)
}

func mknodChar(path string, perm uint32, major, minor uint32) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	} else {
		fmt.Println("mknodChar err", err)
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
