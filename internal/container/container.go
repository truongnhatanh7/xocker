package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/truongnhatanh7/xocker/internal/common"
	"golang.org/x/sys/unix"
)

var HARD_CODED_ROOTFS = "./rootfs"

func RunContainer(cmdArgs []string) error {
	if os.Getenv("_IN_CONTAINER") == "1" {
		return handleChild(cmdArgs)
	}

	self, err := os.Executable()
	common.Must(err)
	fmt.Println("self", self)

	// ensure rootfs exists
	if _, err := os.Stat(HARD_CODED_ROOTFS); err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Println("rootfs exists")

	fmt.Printf("cmdArgs %#v\n", cmdArgs)

	// check ps aux count before create ns
	checkPsAuxCount()

	// spawn new ns
	c := exec.Command(
		"unshare",
		"--mount",
		"--uts",
		"--ipc",
		"--net",
		"--pid",
		"--fork",
		"--mount-proc",
		"--",
		self, "run", "/bin/sh",
		cmdArgs[0],
	)
	fmt.Println(c.String())

	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	os.Setenv("_IN_CONTAINER", "1")
	c.Env = os.Environ()
	common.Must(c.Run())

	return nil
}

func handleChild(cmdArgs []string) error {
	// checkPsAuxCount()

	// mount nescessay stuff
	//
	// rootfs must be a mount point
	// mount proc - pseudo filesystem
	// mount tmpfs - dev

	common.Must(syscall.Mount("proc", HARD_CODED_ROOTFS+"/proc", "proc", 0, ""))
	common.Must(syscall.Mount("tmpfs", HARD_CODED_ROOTFS+"/dev", "tmpfs", 0, ""))
	// Best-effort minimal device nodes (requires CAP_MKNOD; may fail rootless)
	common.Must(mknodChar(HARD_CODED_ROOTFS+"/dev/null", 0o666, 1, 3))
	common.Must(mknodChar(HARD_CODED_ROOTFS+"/dev/zero", 0o666, 1, 5))
	common.Must(mknodChar(HARD_CODED_ROOTFS+"/dev/random", 0o666, 1, 8))
	common.Must(mknodChar(HARD_CODED_ROOTFS+"/dev/urandom", 0o666, 1, 9))

	fmt.Println("done mounting")

	// checkPsAuxCount()

	// pivot root
	// make newroot a mountpoint
	unix.Mount(HARD_CODED_ROOTFS, HARD_CODED_ROOTFS, "", unix.MS_BIND|unix.MS_REC, "")
	common.Must(os.MkdirAll(HARD_CODED_ROOTFS+"/old_root", 0o777))
	common.Must(unix.PivotRoot(HARD_CODED_ROOTFS, HARD_CODED_ROOTFS+"/old_root"))
	common.Must(os.Chdir("/"))
	common.Must(unix.Unmount("./old_root", syscall.MNT_DETACH))
	fmt.Println("done pivot root")

	// actually exec input command
	fmt.Println(cmdArgs)

	_, err := os.Stat("/bin/sh")
	common.Must(err)

	realCmd := strings.Join(cmdArgs, " ")
	common.Must(syscall.Exec("/bin/sh", []string{"sh", "-c", realCmd}, []string{}))

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
