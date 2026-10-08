// Command wozzle-mcp exposes WSL 3.0 native containers (wslc) to AI agents
// via the Model Context Protocol, using the stdio transport.
//
// It reuses Wozzle's wslc wrapper, so agents get clean JSON instead of the
// raw CLI's formatted strings, ID mismatches and text event lines.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"wozzle/internal/wslc"
)

const version = "0.2.1"

func main() {
	exe := flag.String("wslc", "wslc", "path to wslc.exe")
	flag.Parse()

	// stdout belongs to the MCP protocol; diagnostics go to stderr.
	log.SetFlags(0)
	log.SetPrefix("wozzle-mcp: ")
	log.SetOutput(log.Writer())

	s := mcp.NewServer(&mcp.Implementation{Name: "wozzle", Version: version}, nil)
	t := &tools{p: wslc.New(*exe)}
	t.register(s)

	if err := s.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

type tools struct{ p *wslc.CLI }

func (t *tools) register(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "containers_list",
		Description: "List all WSL native containers (running and stopped): id, name, image, state, health, ports, command.",
	}, t.containersList)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "container_inspect",
		Description: "Inspect one container: full raw JSON of its configuration and state (image, env, entrypoint, network...).",
	}, t.containerInspect)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "container_logs",
		Description: "Read the last N log lines of a container (default 100, max 2000), optionally since a timestamp.",
	}, t.containerLogs)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "container_stats",
		Description: "Snapshot resource usage: CPU %, memory (used/limit), network and block I/O counters, PID count. All containers, or one when id is given.",
	}, t.containerStats)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "exec_run",
		Description: "Run a shell command inside a running container (via sh -c) and return combined output plus exit code. One-shot, non-interactive. Default timeout 30s, max 300s.",
	}, t.execRun)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "container_action",
		Description: "Control a container lifecycle: start, stop, kill or restart. stop may take ~10s before the container is killed.",
	}, t.containerAction)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "images_list",
		Description: "List local container images: repository, tag, size, creation time.",
	}, t.imagesList)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "system_info",
		Description: "WSL / WSL container runtime versions, kernel, Windows build and container counts.",
	}, t.systemInfo)
}

// ---- helpers ----

func jsonResult(v any) (*mcp.CallToolResult, any, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return text(string(b)), nil, nil
}

func text(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}

// resolve maps a container reference (exact name, exact id, or id prefix of
// at least 2 chars) to its container, preferring names.
func (t *tools) resolve(ctx context.Context, ref string) (*wslc.Container, error) {
	cs, err := t.p.Containers(ctx)
	if err != nil {
		return nil, err
	}
	var prefix []wslc.Container
	for _, c := range cs {
		switch {
		case c.Name == ref, c.ID == ref:
			return &c, nil
		case len(ref) >= 2 && strings.HasPrefix(c.ID, ref):
			prefix = append(prefix, c)
		}
	}
	switch len(prefix) {
	case 1:
		return &prefix[0], nil
	case 0:
		return nil, fmt.Errorf("container %q not found", ref)
	default:
		return nil, fmt.Errorf("id prefix %q is ambiguous (%d containers match)", ref, len(prefix))
	}
}

// ---- tool implementations ----

type noArgs struct{}

type idArgs struct {
	ID string `json:"id" jsonschema:"container id or name (names are preferred; id prefix >= 2 chars also works)"`
}

func (t *tools) containersList(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
	cs, err := t.p.Containers(ctx)
	if err != nil {
		return nil, nil, err
	}
	if cs == nil {
		cs = []wslc.Container{}
	}
	return jsonResult(cs)
}

func (t *tools) containerInspect(ctx context.Context, _ *mcp.CallToolRequest, args idArgs) (*mcp.CallToolResult, any, error) {
	c, err := t.resolve(ctx, args.ID)
	if err != nil {
		return nil, nil, err
	}
	raw, _, err := t.p.Inspect(ctx, c.Name)
	if err != nil {
		return nil, nil, err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return text(string(raw)), nil, nil
	}
	return text(buf.String()), nil, nil
}

type logsArgs struct {
	ID    string `json:"id"`
	Tail  int    `json:"tail,omitempty" jsonschema:"number of last lines to return (default 100, max 2000)"`
	Since string `json:"since,omitempty" jsonschema:"optional: only lines since this timestamp (RFC3339 or wslc --since syntax)"`
}

func (t *tools) containerLogs(ctx context.Context, _ *mcp.CallToolRequest, args logsArgs) (*mcp.CallToolResult, any, error) {
	c, err := t.resolve(ctx, args.ID)
	if err != nil {
		return nil, nil, err
	}
	tail := args.Tail
	if tail <= 0 {
		tail = 100
	}
	if tail > 2000 {
		tail = 2000
	}
	lines, err := t.p.Logs(ctx, c.Name, wslc.LogsOptions{Tail: tail, Since: args.Since})
	if err != nil {
		return nil, nil, err
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "container %s: %d log line(s)\n", c.Name, len(lines))
	for _, l := range lines {
		if l.TS != nil && *l.TS != "" {
			sb.WriteString(*l.TS)
			sb.WriteByte(' ')
		}
		sb.WriteString(l.Text)
		sb.WriteByte('\n')
	}
	return text(sb.String()), nil, nil
}

type statsArgs struct {
	ID string `json:"id,omitempty" jsonschema:"optional: restrict to one container id/name"`
}

type statOut struct {
	Name          string  `json:"name"`
	ID            string  `json:"id"`
	CPUPercent    float64 `json:"cpuPercent"`
	MemBytes      uint64  `json:"memBytes"`
	MemLimitBytes uint64  `json:"memLimitBytes"`
	MemPercent    float64 `json:"memPercent"`
	NetRx         uint64  `json:"netRx"`
	NetTx         uint64  `json:"netTx"`
	BlockRead     uint64  `json:"blockRead"`
	BlockWrite    uint64  `json:"blockWrite"`
	PIDs          int     `json:"pids"`
	TS            string  `json:"ts"`
}

func (t *tools) containerStats(ctx context.Context, _ *mcp.CallToolRequest, args statsArgs) (*mcp.CallToolResult, any, error) {
	want := ""
	if args.ID != "" {
		c, err := t.resolve(ctx, args.ID)
		if err != nil {
			return nil, nil, err
		}
		want = c.Name
	}
	stats, err := t.p.Stats(ctx)
	if err != nil {
		return nil, nil, err
	}
	out := []statOut{}
	for _, s := range stats {
		if want != "" && s.Name != want {
			continue
		}
		id := s.FullID
		if len(id) > 12 {
			id = id[:12]
		}
		out = append(out, statOut{
			Name: s.Name, ID: id,
			CPUPercent: s.CPUPercent, MemBytes: s.MemBytes, MemLimitBytes: s.MemLimitBytes,
			MemPercent: s.MemPercent, NetRx: s.NetRx, NetTx: s.NetTx,
			BlockRead: s.BlockRead, BlockWrite: s.BlockWrite, PIDs: s.PIDs, TS: s.TS,
		})
	}
	if want != "" && len(out) == 0 {
		return nil, nil, fmt.Errorf("container %q is not running (no stats)", want)
	}
	return jsonResult(out)
}

type execArgs struct {
	ID             string `json:"id"`
	Command        string `json:"command" jsonschema:"shell command, executed via sh -c inside the container"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty" jsonschema:"default 30, max 300"`
}

func (t *tools) execRun(ctx context.Context, _ *mcp.CallToolRequest, args execArgs) (*mcp.CallToolResult, any, error) {
	c, err := t.resolve(ctx, args.ID)
	if err != nil {
		return nil, nil, err
	}
	timeout := args.TimeoutSeconds
	if timeout <= 0 {
		timeout = 30
	}
	if timeout > 300 {
		timeout = 300
	}
	ctx2, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	out, code, err := t.p.ExecOneShot(ctx2, c.Name, args.Command)
	if err != nil {
		return nil, nil, err
	}
	return jsonResult(struct {
		Output   string `json:"output"`
		ExitCode int    `json:"exitCode"`
	}{
		Output:   strings.TrimRight(out, "\n"),
		ExitCode: code,
	})
}

type actionArgs struct {
	ID     string `json:"id"`
	Action string `json:"action" jsonschema:"one of: start, stop, kill, restart"`
}

func (t *tools) containerAction(ctx context.Context, _ *mcp.CallToolRequest, args actionArgs) (*mcp.CallToolResult, any, error) {
	c, err := t.resolve(ctx, args.ID)
	if err != nil {
		return nil, nil, err
	}
	if err := t.p.Action(ctx, args.Action, c.Name); err != nil {
		return nil, nil, err
	}
	return jsonResult(struct {
		OK        bool   `json:"ok"`
		Action    string `json:"action"`
		Container string `json:"container"`
	}{true, args.Action, c.Name})
}

func (t *tools) imagesList(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
	imgs, err := t.p.Images(ctx)
	if err != nil {
		return nil, nil, err
	}
	if imgs == nil {
		imgs = []wslc.Image{}
	}
	return jsonResult(imgs)
}

func (t *tools) systemInfo(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
	info, err := t.p.SystemInfo(ctx)
	if err != nil {
		return nil, nil, err
	}
	cs, _ := t.p.Containers(ctx)
	running := 0
	for _, c := range cs {
		if c.State == "running" {
			running++
		}
	}
	info.Running = running
	info.Total = len(cs)
	info.WozzleVersion = version
	return jsonResult(info)
}
