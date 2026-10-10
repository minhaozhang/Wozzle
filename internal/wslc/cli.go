package wslc

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// CLI executes wslc.exe commands and parses their output.
type CLI struct{ exe string }

// New creates a CLI wrapper; exe defaults to "wslc" (resolved via PATH).
func New(exe string) *CLI {
	if exe == "" {
		exe = "wslc"
	}
	return &CLI{exe: exe}
}

func (c *CLI) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, c.exe, args...)
	cmd.Env = append(os.Environ(), "WSL_UTF8=1")
	hideWindow(cmd) // no console flash when running as a GUI-subsystem exe
	return cmd
}

func (c *CLI) run(ctx context.Context, args ...string) (string, error) {
	cmd := c.command(ctx, args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("wslc %s: %s", strings.Join(args, " "), msg)
	}
	return string(out), nil
}

// ---- containers ----

type rawContainer struct {
	Command      string
	CreatedAt    string
	HealthStatus string
	ID           string
	Image        string
	Labels       string
	Names        string
	Networks     string
	Ports        string
	RunningFor   string
	Size         string
	State        string
	Status       string
}

func (r rawContainer) toContainer() Container {
	name := strings.TrimPrefix(strings.TrimSpace(r.Names), "/")
	health := strings.TrimSpace(r.HealthStatus)
	if health == "" {
		health = "none"
	}
	return Container{
		ID:        r.ID,
		ShortID:   r.ID,
		Name:      name,
		Image:     r.Image,
		State:     r.State,
		Health:    health,
		StatusText: r.Status,
		CreatedAt: ParseCreatedAt(r.CreatedAt),
		Command:   strings.Trim(r.Command, "\""),
		Ports:     ParsePorts(r.Ports),
		Labels:    map[string]string{},
	}
}

// Containers lists all containers (`wslc list -a --format json`, JSONL).
func (c *CLI) Containers(ctx context.Context) ([]Container, error) {
	out, err := c.run(ctx, "list", "-a", "--format", "json")
	if err != nil {
		return nil, err
	}
	var res []Container
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var raw rawContainer
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue // tolerate schema drift
		}
		res = append(res, raw.toContainer())
	}
	return res, nil
}

// ---- stats ----

type rawStat struct {
	BlockIO  string
	CPUPerc  string
	ID       string
	MemPerc  string
	MemUsage string
	Name     string
	NetIO    string
	PIDs     int
}

// Stats snapshots resource usage of running containers (JSONL).
func (c *CLI) Stats(ctx context.Context) ([]Stat, error) {
	out, err := c.run(ctx, "stats", "--format", "json")
	if err != nil {
		return nil, err
	}
	var res []Stat
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var r rawStat
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue
		}
		mem, limit := ParseSizePair(r.MemUsage)
		rx, tx := ParseSizePair(r.NetIO)
		br, bw := ParseSizePair(r.BlockIO)
		res = append(res, Stat{
			Name:          r.Name,
			FullID:        r.ID,
			CPUPercent:    ParsePercent(r.CPUPerc),
			MemBytes:      mem,
			MemLimitBytes: limit,
			MemPercent:    ParsePercent(r.MemPerc),
			NetRx:         rx,
			NetTx:         tx,
			BlockRead:     br,
			BlockWrite:    bw,
			PIDs:          r.PIDs,
			TS:            time.Now().UTC().Format(time.RFC3339),
		})
	}
	return res, nil
}

// ---- inspect ----

// Inspect returns the raw inspect JSON plus the subset we consume.
func (c *CLI) Inspect(ctx context.Context, id string) (json.RawMessage, *InspectData, error) {
	out, err := c.run(ctx, "inspect", id)
	if err != nil {
		return nil, nil, err
	}
	var arr []json.RawMessage
	if err := json.Unmarshal([]byte(out), &arr); err != nil {
		return nil, nil, err
	}
	if len(arr) == 0 {
		return nil, nil, fmt.Errorf("wslc inspect %s: empty result", id)
	}
	var d InspectData
	if err := json.Unmarshal(arr[0], &d); err != nil {
		return nil, nil, err
	}
	return arr[0], &d, nil
}

// ---- images ----

type rawImage struct {
	Containers   string
	CreatedAt    string
	CreatedSince string
	Digest       string
	ID           string
	Repository   string
	Size         string
	Tag          string
}

// Images lists local images (JSONL).
func (c *CLI) Images(ctx context.Context) ([]Image, error) {
	out, err := c.run(ctx, "images", "--format", "json")
	if err != nil {
		return nil, err
	}
	var res []Image
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var r rawImage
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue
		}
		res = append(res, Image{
			ID:         r.ID,
			Repository: r.Repository,
			Tag:        r.Tag,
			SizeBytes:  ParseSize(r.Size),
			CreatedAt:  ParseCreatedAt(r.CreatedAt),
		})
	}
	return res, nil
}

// ---- system info ----

type rawInfo struct {
	Client struct {
		KernelVersion  string
		Version        string
		WindowsVersion string
	}
	Server struct {
		SessionManagerVersion string
	}
}

// SystemInfo reads `wslc info --format json`.
func (c *CLI) SystemInfo(ctx context.Context) (*SystemInfo, error) {
	out, err := c.run(ctx, "info", "--format", "json")
	if err != nil {
		return nil, err
	}
	var r rawInfo
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		return nil, err
	}
	return &SystemInfo{
		WSLVersion:  r.Client.Version,
		WSLcVersion: r.Client.Version,
		Kernel:      r.Client.KernelVersion,
		OS:          r.Client.WindowsVersion,
	}, nil
}

// ---- logs ----

// LogsOptions controls a `wslc logs` invocation.
type LogsOptions struct {
	Tail   int
	Since  string
	Until  string
	Follow bool
}

func (o LogsOptions) args() []string {
	args := []string{"logs", "-t"}
	if o.Tail > 0 {
		args = append(args, "--tail", strconv.Itoa(o.Tail))
	}
	if o.Since != "" {
		args = append(args, "--since", o.Since)
	}
	if o.Until != "" {
		args = append(args, "--until", o.Until)
	}
	if o.Follow {
		args = append(args, "-f")
	}
	return args
}

// LogsStream starts `wslc logs` and returns the command plus its stdout.
// The caller owns the process lifetime via ctx.
func (c *CLI) LogsStream(ctx context.Context, id string, opt LogsOptions) (*exec.Cmd, io.ReadCloser, error) {
	args := append(opt.args(), id)
	cmd := c.command(ctx, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("wslc logs: %w", err)
	}
	return cmd, stdout, nil
}

// Logs collects historical log lines (no follow).
func (c *CLI) Logs(ctx context.Context, id string, opt LogsOptions) ([]LogLine, error) {
	opt.Follow = false
	cmd, stdout, err := c.LogsStream(ctx, id, opt)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()
	var lines []LogLine
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		lines = append(lines, ParseLogLine(sc.Text()))
	}
	return lines, nil
}

// ---- events ----

// EventsStream starts `wslc events` and returns the command plus stdout.
func (c *CLI) EventsStream(ctx context.Context) (*exec.Cmd, io.ReadCloser, error) {
	cmd := c.command(ctx, "events")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("wslc events: %w", err)
	}
	return cmd, stdout, nil
}

// ---- exec ----

// Exec builds an interactive `wslc exec -i -t` command. The caller wires
// stdin/stdout/stderr pipes and calls Start.
func (c *CLI) Exec(ctx context.Context, id, command string, rows, cols int) *exec.Cmd {
	args := []string{"exec", "-i", "-t",
		"-e", "COLUMNS=" + strconv.Itoa(cols),
		"-e", "LINES=" + strconv.Itoa(rows),
		id,
	}
	args = append(args, strings.Fields(command)...)
	return c.command(ctx, args...)
}

// ExecOneShot runs a command inside a container without a TTY and returns
// its combined output and exit code (0 on success). The command runs via
// `sh -c`, so shell features are available.
//
// Note: no `-i` here on purpose. With `-i` wslc keeps the child's stdin open,
// and when the caller has no real console behind it (service/hidden window)
// any exec living longer than ~1s dies with ERROR_INVALID_HANDLE, wedging
// the container's exec channel until restart. Without `-i` long-running
// commands work fine.
func (c *CLI) ExecOneShot(ctx context.Context, id, command string) (string, int, error) {
	cmd := c.command(ctx, "exec", id, "sh", "-c", command)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return buf.String(), ee.ExitCode(), nil
		}
		return buf.String(), -1, fmt.Errorf("wslc exec %s: %w", id, err)
	}
	return buf.String(), 0, nil
}

// ---- lifecycle actions ----

// Action runs a lifecycle verb: start, stop, kill or restart.
func (c *CLI) Action(ctx context.Context, action, id string) error {
	switch action {
	case "start", "stop", "kill", "restart":
	default:
		return fmt.Errorf("unsupported action %q", action)
	}
	_, err := c.run(ctx, action, id)
	return err
}

// Remove deletes a container.
func (c *CLI) Remove(ctx context.Context, id string) error {
	_, err := c.run(ctx, "remove", id)
	return err
}
