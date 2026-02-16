package cgroupv2

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/godbus/dbus/v5"
	"github.com/truongnhatanh7/xocker/internal/common"
	"github.com/truongnhatanh7/xocker/internal/logger"
	"go.uber.org/zap"
)

var (
	HALF_CPU_QUOTA = uint64(500000)
	ONE_CPU_QUOTA  = uint64(1000000)
)

type CgroupV2 struct {
	path     string
	dbusConn *dbus.Conn
}

func NewCgroupV2(containerId string) *CgroupV2 {
	path := filepath.Join("/sys/fs/cgroup/xocker", containerId)
	common.Must(os.MkdirAll(path, 0755))
	return &CgroupV2{
		path: path,
	}
}

type CgroupV2SetSpecs struct {
	ApplyToPid int
	CPUSpec    *CPUSpec
	MemSpec    *MemSpec
}

type CPUSpec struct {
	Quota uint64
}

type MemSpec struct {
	Limit uint64
}

func (c *CgroupV2) Limit(s *CgroupV2SetSpecs) {
	if s == nil {
		panic("spec cannot be nil")
	}

	logger.Log.Debug("limit pid", zap.Int("pid", s.ApplyToPid))
	logger.Log.Debug("cpu", zap.Uint64("quota", s.CPUSpec.Quota))
	logger.Log.Debug("mem", zap.Uint64("lim", s.MemSpec.Limit))

	conn, err := dbus.SystemBus()
	common.Must(err)
	c.dbusConn = conn

	unitName := fmt.Sprintf("xocker-%d.scope", s.ApplyToPid)
	systemd := conn.Object(
		"org.freedesktop.systemd1",
		"/org/freedesktop/systemd1",
	)

	props := []struct {
		Name  string
		Value dbus.Variant
	}{
		{
			Name:  "PIDs",
			Value: dbus.MakeVariant([]uint32{uint32(s.ApplyToPid)}),
		},
		{
			Name:  "MemoryMax",
			Value: dbus.MakeVariant(uint64(s.MemSpec.Limit * 1024 * 1024)),
		},
		{
			Name:  "CPUQuotaPerSecUSec",
			Value: dbus.MakeVariant(s.CPUSpec.Quota),
		},
	}

	aux := []struct {
		Name  string
		Value []struct {
			Name  string
			Value dbus.Variant
		}
	}{}

	call := systemd.Call(
		"org.freedesktop.systemd1.Manager.StartTransientUnit",
		0,
		unitName,
		"replace",
		props,
		aux,
	)
	common.Must(call.Err)
}

func (c *CgroupV2) Destroy() {
	c.dbusConn.Close()
}
