package wslc

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// HostStats is a live snapshot of the WSL utility VM — the machine all
// containers actually run in. Memory limits reported by container stats
// refer to this VM, not the Windows host.
type HostStats struct {
	CPUPercent    float64 `json:"cpuPercent"`
	MemBytes      uint64  `json:"memBytes"`
	MemTotalBytes uint64  `json:"memTotalBytes"`
	MemPercent    float64 `json:"memPercent"`
	UptimeSeconds float64 `json:"uptimeSeconds"`
}

// HostMonitor samples HostStats by reading /proc inside the WSL utility VM.
//
// All WSL2 distros share one utility VM, so any running distro sees the same
// /proc. We prefer the always-running system distro (docker-desktop) to avoid
// booting a user distro just for metrics, and fall back to the default distro.
// The first successful distro is cached. CPU percent is computed from
// consecutive /proc/stat samples, so the first Sample() reports 0%.
type HostMonitor struct {
	mu     sync.Mutex
	prevOK bool
	prevB  uint64 // busy ticks
	prevT  uint64 // total ticks
	distro string // "", "docker-desktop"
	tried  bool
}

// NewHostMonitor creates a monitor; pass the distro to prefer ("" = default).
func NewHostMonitor(preferDistro string) *HostMonitor {
	return &HostMonitor{distro: preferDistro}
}

func readProc(ctx context.Context, distro string) (string, error) {
	args := []string{}
	if distro != "" {
		args = append(args, "-d", distro)
	}
	args = append(args, "sh", "-c",
		"cat /proc/meminfo && echo ---WOZZLE--- && head -n 1 /proc/stat && echo ---WOZZLE--- && cat /proc/uptime")
	cmd := exec.CommandContext(ctx, "wsl.exe", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("wsl.exe (%s): %w", distroName(distro), err)
	}
	return string(out), nil
}

func distroName(d string) string {
	if d == "" {
		return "default distro"
	}
	return d
}

// Sample reads the VM's /proc and returns host metrics. Errors are non-fatal
// for callers mixing this with container stats.
func (m *HostMonitor) Sample(ctx context.Context) (*HostStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var (
		out string
		err error
	)
	if m.tried {
		out, err = readProc(ctx, m.distro)
	} else {
		// first call: prefer docker-desktop, fall back to the default distro
		for _, d := range []string{"docker-desktop", ""} {
			out, err = readProc(ctx, d)
			if err == nil {
				m.distro = d
				m.tried = true
				break
			}
		}
		if err != nil {
			return nil, err
		}
	}
	if err != nil {
		return nil, err
	}

	parts := strings.Split(out, "---WOZZLE---")
	if len(parts) != 3 {
		return nil, fmt.Errorf("host monitor: unexpected /proc payload (%d sections)", len(parts))
	}

	var st HostStats

	// ---- meminfo: MemTotal / MemAvailable (kB) ----
	var total, avail uint64
	for _, line := range strings.Split(parts[0], "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		v, perr := strconv.ParseUint(fields[1], 10, 64)
		if perr != nil {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			total = v * 1024
		case "MemAvailable:":
			avail = v * 1024
		}
	}
	if total == 0 {
		return nil, fmt.Errorf("host monitor: MemTotal missing")
	}
	st.MemTotalBytes = total
	if avail > total {
		avail = total
	}
	st.MemBytes = total - avail
	st.MemPercent = float64(st.MemBytes) / float64(total) * 100

	// ---- stat: "cpu  user nice system idle iowait irq softirq steal …" ----
	cpuFields := strings.Fields(strings.TrimSpace(parts[1]))
	if len(cpuFields) >= 5 && cpuFields[0] == "cpu" {
		var vals []uint64
		for _, f := range cpuFields[1:] {
			v, perr := strconv.ParseUint(f, 10, 64)
			if perr != nil {
				break
			}
			vals = append(vals, v)
		}
		if len(vals) >= 5 {
			var idleAll, sum uint64
			for i, v := range vals {
				sum += v
				if i == 3 || i == 4 { // idle + iowait
					idleAll += v
				}
			}
			busy := sum - idleAll
			if m.prevOK && sum > m.prevT {
				dt := sum - m.prevT
				db := busy - m.prevB
				if db > dt {
					db = dt
				}
				st.CPUPercent = float64(db) / float64(dt) * 100
			}
			m.prevB, m.prevT, m.prevOK = busy, sum, true
		}
	}

	// ---- uptime: "seconds idle_seconds" ----
	if f := strings.Fields(strings.TrimSpace(parts[2])); len(f) >= 1 {
		if v, perr := strconv.ParseFloat(f[0], 64); perr == nil {
			st.UptimeSeconds = v
		}
	}

	return &st, nil
}
