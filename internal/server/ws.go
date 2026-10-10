package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"wozzle/internal/wslc"
)

// WS frame structs; see docs/API.md.

type backfillMsg struct {
	T     string         `json:"t"`
	Lines []wslc.LogLine `json:"lines"`
}

type lineMsg struct {
	T    string       `json:"t"`
	Line wslc.LogLine `json:"line"`
}

type noticeMsg struct {
	T       string `json:"t"`
	Message string `json:"message"`
}

type endMsg struct {
	T      string `json:"t"`
	Reason string `json:"reason"`
}

type pongMsg struct {
	T string `json:"t"`
}

type errorMsg struct {
	T       string `json:"t"`
	Message string `json:"message"`
}

type outputMsg struct {
	T    string `json:"t"`
	Data string `json:"data"`
}

type exitMsg struct {
	T    string `json:"t"`
	Code int    `json:"code"`
}

type eventMsg struct {
	T     string      `json:"t"`
	Event wslc.Event  `json:"event"`
}

// wsWriter serializes websocket writes (coder/websocket allows one writer).
type wsWriter struct {
	mu sync.Mutex
	c  *websocket.Conn
}

func (w *wsWriter) json(ctx context.Context, v any) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return wsjson.Write(ctx, w.c, v)
}

func acceptLocal(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	return websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // localhost dev tool; dev server proxies cross-origin
	})
}

// ---- logs ----

func (s *Server) handleWSLogs(w http.ResponseWriter, r *http.Request) {
	cont := s.store.Resolve(r.PathValue("id"))
	if cont == nil {
		httpError(w, http.StatusNotFound, "container not found")
		return
	}
	tail := clampInt(queryInt(r, "tail", 300), 1, 5000)
	c, err := acceptLocal(w, r)
	if err != nil {
		return
	}
	ctx := r.Context()
	writer := &wsWriter{c: c}

	// Exited containers cannot be followed. Send whatever history wslc still
	// has (note: wslc writes exited-container logs to stderr; CLI.Logs handles
	// that), then a terminal end frame, and close the socket cleanly.
	if cont.State != "running" {
		lines, err := s.provider.Logs(ctx, cont.ID, wslc.LogsOptions{Tail: tail})
		if err != nil {
			_ = writer.json(ctx, errorMsg{T: "error", Message: err.Error()})
		} else if len(lines) > 0 {
			_ = writer.json(ctx, backfillMsg{T: "backfill", Lines: lines})
		}
		_ = writer.json(ctx, endMsg{T: "end", Reason: "container-exited"})
		_ = c.Close(websocket.StatusNormalClosure, "container not running")
		return
	}

	sub, err := s.logs.Subscribe(cont.ID, tail)
	if err != nil {
		_ = writer.json(ctx, errorMsg{T: "error", Message: err.Error()})
		_ = c.Close(websocket.StatusInternalError, "subscribe failed")
		return
	}
	defer s.logs.Unsubscribe(cont.ID, sub)

	// control reader: pause/resume/tail/ping
	go func() {
		for {
			var msg struct {
				T string `json:"t"`
				N int    `json:"n"`
			}
			if err := wsjson.Read(ctx, c, &msg); err != nil {
				return
			}
			switch msg.T {
			case "pause":
				sub.pause.Store(true)
			case "resume":
				sub.pause.Store(false)
			case "ping":
				_ = writer.json(ctx, pongMsg{T: "pong"})
			case "tail":
				n := clampInt(msg.N, 1, 5000)
				_ = writer.json(ctx, backfillMsg{T: "backfill", Lines: s.logs.Ring(cont.ID, n)})
			}
		}
	}()

	if len(sub.Initial) > 0 {
		_ = writer.json(ctx, backfillMsg{T: "backfill", Lines: sub.Initial})
	}

	// Batch the initial burst (the `wslc logs --tail` history replay) into a
	// single backfill frame, then stream live lines individually. A gap of
	// 150ms without output (or a 1.5s cap) ends the burst window.
	var pending []wslc.LogLine
	burst := true
	gapTimer := time.NewTimer(150 * time.Millisecond)
	defer gapTimer.Stop()
	burstDeadline := time.After(1500 * time.Millisecond)
	flush := func() {
		if len(pending) > 0 {
			_ = writer.json(ctx, backfillMsg{T: "backfill", Lines: pending})
			pending = nil
		}
	}

	for {
		select {
		case line := <-sub.data:
			if burst {
				pending = append(pending, line)
				if !gapTimer.Stop() {
					select {
					case <-gapTimer.C:
					default:
					}
				}
				gapTimer.Reset(150 * time.Millisecond)
				if len(pending) >= 5000 {
					flush()
					burst = false
				}
			} else {
				if err := writer.json(ctx, lineMsg{T: "line", Line: line}); err != nil {
					return
				}
			}
		case <-gapTimer.C:
			if burst {
				flush()
				burst = false
			}
		case <-burstDeadline:
			if burst {
				flush()
				burst = false
			}
		case note := <-sub.notice:
			_ = writer.json(ctx, noticeMsg{T: "notice", Message: note})
		case reason := <-sub.end:
			_ = writer.json(ctx, endMsg{T: "end", Reason: reason})
			_ = c.Close(websocket.StatusNormalClosure, reason)
			return
		case <-ctx.Done():
			return
		case <-sub.quit:
			return
		}
	}
}

// ---- stats ----

func (s *Server) handleWSStats(w http.ResponseWriter, r *http.Request) {
	c, err := acceptLocal(w, r)
	if err != nil {
		return
	}
	ctx := c.CloseRead(r.Context())
	s.statsClients.Add(1)
	defer s.statsClients.Add(-1)
	ch := s.statsHub.subscribe()
	defer s.statsHub.unsubscribe(ch)

	if fr, ok := s.lastStats.Load().(statsFrame); ok {
		if err := wsjson.Write(ctx, c, fr); err != nil {
			return
		}
	}
	for {
		select {
		case v := <-ch:
			if err := wsjson.Write(ctx, c, v); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// ---- events ----

func (s *Server) handleWSEvents(w http.ResponseWriter, r *http.Request) {
	c, err := acceptLocal(w, r)
	if err != nil {
		return
	}
	ctx := c.CloseRead(r.Context())
	ch := s.eventHub.subscribe()
	defer s.eventHub.unsubscribe(ch)
	for {
		select {
		case v := <-ch:
			ev, ok := v.(wslc.Event)
			if !ok {
				continue
			}
			if err := wsjson.Write(ctx, c, eventMsg{T: "event", Event: ev}); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// ---- exec / terminal ----

func (s *Server) handleWSExec(w http.ResponseWriter, r *http.Request) {
	cont := s.store.Resolve(r.PathValue("id"))
	if cont == nil {
		httpError(w, http.StatusNotFound, "container not found")
		return
	}
	if cont.State != "running" {
		httpError(w, http.StatusConflict, "container not running")
		return
	}
	q := r.URL.Query()
	command := q.Get("cmd")
	if command == "" {
		command = "sh"
	}
	rows := clampInt(queryInt(r, "rows", 24), 2, 500)
	cols := clampInt(queryInt(r, "cols", 80), 2, 1000)

	c, err := acceptLocal(w, r)
	if err != nil {
		return
	}
	ctx := r.Context()
	writer := &wsWriter{c: c}

	cmd := s.provider.Exec(ctx, cont.ID, command, rows, cols)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		_ = writer.json(ctx, errorMsg{T: "error", Message: err.Error()})
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = writer.json(ctx, errorMsg{T: "error", Message: err.Error()})
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = writer.json(ctx, errorMsg{T: "error", Message: err.Error()})
		return
	}
	if err := cmd.Start(); err != nil {
		_ = writer.json(ctx, errorMsg{T: "error", Message: err.Error()})
		return
	}
	_ = writer.json(ctx, pongMsg{T: "ready"})

	pump := func(rc io.ReadCloser) {
		buf := make([]byte, 4096)
		for {
			n, err := rc.Read(buf)
			if n > 0 {
				if werr := writer.json(ctx, outputMsg{T: "output", Data: string(buf[:n])}); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}
	go pump(stdout)
	go pump(stderr)

	go func() {
		err := cmd.Wait()
		code := 0
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else if err != nil {
			_ = writer.json(ctx, errorMsg{T: "error", Message: err.Error()})
			return
		}
		_ = writer.json(ctx, exitMsg{T: "exit", Code: code})
	}()

	for {
		var msg struct {
			T    string `json:"t"`
			Data string `json:"data"`
			Rows int    `json:"rows"`
			Cols int    `json:"cols"`
		}
		if err := wsjson.Read(ctx, c, &msg); err != nil {
			return
		}
		switch msg.T {
		case "input":
			if _, err := io.WriteString(stdin, msg.Data); err != nil {
				return
			}
		case "ping":
			_ = writer.json(ctx, pongMsg{T: "pong"})
		case "resize":
			// wslc CLI cannot resize a running exec session; ignored on purpose.
		}
	}
}
