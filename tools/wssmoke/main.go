// wssmoke is an end-to-end smoke test for Wozzle's WebSocket API against a
// live server and the `wozzle-test` container.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const base = "127.0.0.1:8080"

func dial(ctx context.Context, path string) (*websocket.Conn, error) {
	c, _, err := websocket.Dial(ctx, "ws://"+base+path, nil)
	return c, err
}

func rest(method, path string) int {
	req, _ := http.NewRequest(method, "http://"+base+path, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("    REST %s %s -> ERR %v\n", method, path, err)
		return -1
	}
	defer resp.Body.Close()
	fmt.Printf("    REST %s %s -> %d\n", method, path, resp.StatusCode)
	return resp.StatusCode
}

type frame struct {
	ch   chan json.RawMessage
	done chan error
}

func startReader(ctx context.Context, c *websocket.Conn) *frame {
	f := &frame{ch: make(chan json.RawMessage, 128), done: make(chan error, 1)}
	go func() {
		for {
			var raw json.RawMessage
			if err := wsjson.Read(ctx, c, &raw); err != nil {
				f.done <- err
				return
			}
			select {
			case f.ch <- raw:
			default:
			}
		}
	}()
	return f
}

func kind(raw json.RawMessage) string {
	var m struct {
		T string `json:"t"`
	}
	_ = json.Unmarshal(raw, &m)
	return m.T
}

var failed int

func pass(name string, ok bool, detail string) {
	status := "PASS"
	if !ok {
		status = "FAIL"
		failed++
	}
	fmt.Printf("[%s] %s — %s\n", status, name, detail)
}

func main() {
	testLogs()
	testStats()
	testEventsAndRestart()
	testExec()
	if failed > 0 {
		fmt.Printf("\n%d test(s) FAILED\n", failed)
		os.Exit(1)
	}
	fmt.Println("\nall smoke tests passed")
}

func testLogs() {
	fmt.Println("== logs ==")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c, err := dial(ctx, "/api/ws/logs/wozzle-test?tail=3")
	if err != nil {
		pass("logs", false, "dial: "+err.Error())
		return
	}
	defer c.CloseNow()
	f := startReader(ctx, c)

	go func() {
		time.Sleep(1500 * time.Millisecond)
		_ = wsjson.Write(ctx, c, map[string]any{"t": "ping"})
		time.Sleep(300 * time.Millisecond)
		_ = wsjson.Write(ctx, c, map[string]any{"t": "pause"})
		time.Sleep(300 * time.Millisecond)
		_ = wsjson.Write(ctx, c, map[string]any{"t": "resume"})
	}()

	gotBackfill, gotLine, gotPong := false, false, false
	var sample string
	deadline := time.After(10 * time.Second)
	for !(gotBackfill && gotLine && gotPong) {
		select {
		case raw := <-f.ch:
			switch kind(raw) {
			case "backfill":
				gotBackfill = true
				var m struct {
					Lines []struct {
						TS   *string `json:"ts"`
						Text string  `json:"text"`
					} `json:"lines"`
				}
				_ = json.Unmarshal(raw, &m)
				sample = fmt.Sprintf("backfill=%d lines, ts=%v", len(m.Lines), m.Lines[0].TS != nil)
			case "line":
				gotLine = true
			case "pong":
				gotPong = true
			}
		case err := <-f.done:
			pass("logs", false, "reader: "+err.Error())
			return
		case <-deadline:
			pass("logs", false, fmt.Sprintf("timeout (backfill=%v line=%v pong=%v)", gotBackfill, gotLine, gotPong))
			return
		}
	}
	pass("logs", gotBackfill && gotLine && gotPong, sample)
}

func testStats() {
	fmt.Println("== stats ==")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := dial(ctx, "/api/ws/stats")
	if err != nil {
		pass("stats", false, "dial: "+err.Error())
		return
	}
	defer c.CloseNow()
	f := startReader(ctx, c)

	frames := 0
	detail := ""
	deadline := time.After(8 * time.Second)
	for frames < 2 {
		select {
		case raw := <-f.ch:
			if kind(raw) == "stats" {
				frames++
				var m struct {
					Stats []struct {
						ID         string  `json:"id"`
						CPUPercent float64 `json:"cpuPercent"`
						MemBytes   uint64  `json:"memBytes"`
						PIDs       int     `json:"pids"`
					} `json:"stats"`
				}
				_ = json.Unmarshal(raw, &m)
				if len(m.Stats) > 0 {
					s := m.Stats[0]
					detail = fmt.Sprintf("%d containers, cpu=%.2f%% mem=%dB pids=%d", len(m.Stats), s.CPUPercent, s.MemBytes, s.PIDs)
				}
			}
		case err := <-f.done:
			pass("stats", false, "reader: "+err.Error())
			return
		case <-deadline:
			pass("stats", false, fmt.Sprintf("only %d frames", frames))
			return
		}
	}
	pass("stats", frames >= 2, detail)
}

func testEventsAndRestart() {
	fmt.Println("== events + kill/start cycle ==")
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	evConn, err := dial(ctx, "/api/ws/events")
	if err != nil {
		pass("events", false, "dial: "+err.Error())
		return
	}
	defer evConn.CloseNow()
	evFrames := startReader(ctx, evConn)

	logConn, err := dial(ctx, "/api/ws/logs/wozzle-test?tail=2")
	if err != nil {
		pass("events", false, "logs dial: "+err.Error())
		return
	}
	logFrames := startReader(ctx, logConn)

	// Drain initial log frames until we see a line.
	sawLine := false
	for !sawLine {
		select {
		case raw := <-logFrames.ch:
			if kind(raw) == "line" {
				sawLine = true
			}
		case <-time.After(5 * time.Second):
			pass("events", false, "no initial log lines")
			return
		}
	}

	rest("POST", "/api/containers/wozzle-test/kill?confirm=1")

	// Expect an end frame on the logs connection once the container dies.
	gotEnd := false
	deadline := time.After(20 * time.Second)
	for !gotEnd {
		select {
		case raw := <-logFrames.ch:
			if kind(raw) == "end" {
				gotEnd = true
			}
		case err := <-logFrames.done:
			_ = err
			gotEnd = true // connection closed is also acceptable
		case <-deadline:
			pass("events", false, "no end frame after kill")
			return
		}
	}
	logConn.CloseNow()

	rest("POST", "/api/containers/wozzle-test/start?confirm=1")
	time.Sleep(2 * time.Second)

	// Reconnect logs; expect fresh lines.
	logConn2, err := dial(ctx, "/api/ws/logs/wozzle-test?tail=5")
	if err != nil {
		pass("events", false, "reconnect dial: "+err.Error())
		return
	}
	logFrames2 := startReader(ctx, logConn2)
	gotNewLine := false
	deadline = time.After(15 * time.Second)
	for !gotNewLine {
		select {
		case raw := <-logFrames2.ch:
			if kind(raw) == "line" {
				gotNewLine = true
			}
		case err := <-logFrames2.done:
			pass("events", false, "reconnect reader: "+err.Error())
			return
		case <-deadline:
			pass("events", false, "no fresh lines after start")
			return
		}
	}
	logConn2.CloseNow()

	// Expect kill + start events.
	gotKill, gotStart := false, false
	deadline = time.After(5 * time.Second)
	for !(gotKill && gotStart) {
		select {
		case raw := <-evFrames.ch:
			if kind(raw) == "event" {
				var m struct {
					Event struct {
						Type string `json:"type"`
						Name string `json:"name"`
					} `json:"event"`
				}
				_ = json.Unmarshal(raw, &m)
				if m.Event.Name == "wozzle-test" {
					if strings.Contains(m.Event.Type, "kill") {
						gotKill = true
					}
					if strings.Contains(m.Event.Type, "start") {
						gotStart = true
					}
				}
			}
		case err := <-evFrames.done:
			pass("events", false, "events reader: "+err.Error())
			return
		case <-deadline:
			pass("events", false, fmt.Sprintf("events: kill=%v start=%v", gotKill, gotStart))
			return
		}
	}
	pass("events", gotKill && gotStart, fmt.Sprintf("kill=%v start=%v, logs end+reconnect OK", gotKill, gotStart))
}

func testExec() {
	fmt.Println("== exec ==")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c, err := dial(ctx, "/api/ws/exec/wozzle-test?cmd=sh&rows=24&cols=80")
	if err != nil {
		pass("exec", false, "dial: "+err.Error())
		return
	}
	defer c.CloseNow()
	f := startReader(ctx, c)

	gotReady, sawEcho, gotExit := false, false, false
	output := strings.Builder{}
	sentInput := false
	deadline := time.After(15 * time.Second)
	for !(sawEcho && gotExit) {
		select {
		case raw := <-f.ch:
			switch kind(raw) {
			case "ready":
				gotReady = true
				_ = wsjson.Write(ctx, c, map[string]any{"t": "input", "data": "echo wozzle-exec-ok\r"})
			case "output":
				var m struct {
					Data string `json:"data"`
				}
				_ = json.Unmarshal(raw, &m)
				output.WriteString(m.Data)
				if strings.Contains(output.String(), "wozzle-exec-ok") {
					sawEcho = true
					if !sentInput {
						// nothing; exit sent below
					}
				}
				if sawEcho && !sentInput {
					sentInput = true
					_ = wsjson.Write(ctx, c, map[string]any{"t": "input", "data": "exit\r"})
				}
			case "exit":
				gotExit = true
			case "error":
				var m struct {
					Message string `json:"message"`
				}
				_ = json.Unmarshal(raw, &m)
				pass("exec", false, "error frame: "+m.Message)
				return
			}
		case err := <-f.done:
			pass("exec", false, "reader: "+err.Error())
			return
		case <-deadline:
			pass("exec", false, fmt.Sprintf("timeout ready=%v echo=%v exit=%v", gotReady, sawEcho, gotExit))
			return
		}
	}
	clean := strings.ReplaceAll(strings.ReplaceAll(output.String(), "\r", ""), "\n", " ")
	pass("exec", gotReady && sawEcho && gotExit, "output: "+truncate(clean, 120))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
