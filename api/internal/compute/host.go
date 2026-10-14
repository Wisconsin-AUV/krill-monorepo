package compute

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

func ts(t time.Time) *timestamppb.Timestamp {
	return timestamppb.New(t)
}

// SampleAPI records the API's host every Interval until ctx is done.
func (r *Registry) SampleAPI(ctx context.Context, version string) {
	hostname, _ := os.Hostname()
	started := time.Now()
	platform := runtime.GOOS + "/" + runtime.GOARCH
	if info, err := host.InfoWithContext(ctx); err == nil && info.Platform != "" {
		platform = info.Platform + " " + info.PlatformVersion
	}
	// The first cpu.Percent call has nothing to compare against.
	_, _ = cpu.PercentWithContext(ctx, 0, false)
	ticker := time.NewTicker(Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		stats, err := hostStats(ctx)
		if err != nil {
			slog.Warn("sample api host", "err", err)
			continue
		}
		stats.Platform = platform
		r.RecordAPI(Report{Hostname: hostname, Version: version, StartedAt: started, Host: stats})
	}
}

func hostStats(ctx context.Context) (*krillv1.HostStats, error) {
	pct, err := cpu.PercentWithContext(ctx, 0, false)
	if err != nil {
		return nil, err
	}
	vm, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}
	out := &krillv1.HostStats{
		CpuCount:         uint32(runtime.NumCPU()), //nolint:gosec // core counts fit in uint32
		MemoryUsedBytes:  vm.Used,
		MemoryTotalBytes: vm.Total,
	}
	if len(pct) > 0 {
		out.CpuPercent = pct[0]
	}
	// In a container this is the filesystem Docker keeps its volumes on,
	// which is where frames and the database live. Capacity is used plus
	// free, leaving out the blocks the filesystem reserves, which df also
	// leaves out of what's available.
	if d, err := disk.UsageWithContext(ctx, "/"); err == nil {
		out.DiskUsedBytes = d.Used
		out.DiskTotalBytes = d.Used + d.Free
	}
	return out, nil
}
