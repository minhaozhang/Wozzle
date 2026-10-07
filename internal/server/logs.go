package server

import (
	"bufio"
	"context"
	"sync"
	"sync/atomic"
	"time"

	"wozzle/internal/wslc"
)

const (
	ringCap    = 2000 // log lines kept per container for backfill
	subChanCap = 1024
)

type resolver interface {
	Resolve(string) *wslc.Container
}

// logSub is one browser connection's view of a log stream.
type logSub struct {
	data    chan wslc.LogLine
	notice  chan string
	end     chan string
	quit    chan struct{}
	pause   atomic.Bool
	Initial []wslc.LogLine // ring backfill captured at subscribe time
}

// logStream owns one `wslc logs -f` child process per container.
type logStream struct {
	sup    *LogSupervisor
	key    string
	cancel context.CancelFunc
	done   chan struct{}

	ringMu sync.Mutex
	ring   []wslc.LogLine

	subMu sync.Mutex
	subs  map[*logSub]struct{}
}

// LogSupervisor manages log streams by container key (store ID).
type LogSupervisor struct {
	provider *wslc.CLI
	store    resolver
	mu       sync.Mutex
	streams  map[string]*logStream
}

// NewLogSupervisor creates the supervisor.
func NewLogSupervisor(p *wslc.CLI, st resolver) *LogSupervisor {
	return &LogSupervisor{provider: p, store: st, streams: map[string]*logStream{}}
}

// Subscribe returns a subscription with ring backfill; it starts the stream on
// first use and respawns it across container restarts.
func (sup *LogSupervisor) Subscribe(key string, tail int) (*logSub, error) {
	sup.mu.Lock()
	st := sup.streams[key]
	if st == nil {
		ctx, cancel := context.WithCancel(context.Background())
		st = &logStream{sup: sup, key: key, cancel: cancel, done: make(chan struct{}), subs: map[*logSub]struct{}{}}
		sup.streams[key] = st
		go st.run(ctx, tail)
	}
	sup.mu.Unlock()

	st.subMu.Lock()
	sub := &logSub{
		data:   make(chan wslc.LogLine, subChanCap),
		notice: make(chan string, 8),
		end:    make(chan string, 1),
		quit:   make(chan struct{}),
	}
	st.subs[sub] = struct{}{}
	st.ringMu.Lock()
	if n := len(st.ring); n > 0 {
		lo := n - tail
		if lo < 0 {
			lo = 0
		}
		sub.Initial = append([]wslc.LogLine(nil), st.ring[lo:]...)
	}
	st.ringMu.Unlock()
	st.subMu.Unlock()

	// The stream may have ended between the map lookup and the subscribe;
	// hand the fresh subscriber its end frame instead of hanging.
	select {
	case <-st.done:
		select {
		case sub.end <- "stream-closed":
		default:
		}
	default:
	}
	return sub, nil
}

// Unsubscribe drops a subscription and tears the stream down when idle.
func (sup *LogSupervisor) Unsubscribe(key string, sub *logSub) {
	sup.mu.Lock()
	st := sup.streams[key]
	sup.mu.Unlock()
	if st == nil {
		return
	}
	st.subMu.Lock()
	delete(st.subs, sub)
	empty := len(st.subs) == 0
	st.subMu.Unlock()
	select {
	case <-sub.quit:
	default:
		close(sub.quit)
	}
	if empty {
		sup.remove(st)
	}
}

// Ring returns the last n buffered lines for a container.
func (sup *LogSupervisor) Ring(key string, n int) []wslc.LogLine {
	sup.mu.Lock()
	st := sup.streams[key]
	sup.mu.Unlock()
	if st == nil {
		return nil
	}
	st.ringMu.Lock()
	defer st.ringMu.Unlock()
	if l := len(st.ring); l > 0 {
		lo := l - n
		if lo < 0 {
			lo = 0
		}
		return append([]wslc.LogLine(nil), st.ring[lo:]...)
	}
	return nil
}

// Close tears down every stream (server shutdown).
func (sup *LogSupervisor) Close() {
	sup.mu.Lock()
	streams := make([]*logStream, 0, len(sup.streams))
	for _, st := range sup.streams {
		streams = append(streams, st)
	}
	sup.streams = map[string]*logStream{}
	sup.mu.Unlock()
	for _, st := range streams {
		st.cancel()
	}
}

func (sup *LogSupervisor) remove(st *logStream) {
	sup.mu.Lock()
	defer sup.mu.Unlock()
	if cur := sup.streams[st.key]; cur == st {
		delete(sup.streams, st.key)
		st.cancel()
	}
}

func (st *logStream) run(ctx context.Context, tail int) {
	defer close(st.done)
	t := tail
	for {
		if ctx.Err() != nil {
			return
		}
		cmd, stdout, err := st.sup.provider.LogsStream(ctx, st.key, wslc.LogsOptions{Tail: t, Follow: true})
		if err != nil {
			st.notifyAll("log stream error: " + err.Error())
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			t = 50
			continue
		}
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			line := wslc.ParseLogLine(sc.Text())
			st.appendRing(line)
			st.fanout(line)
		}
		_ = cmd.Wait()
		if ctx.Err() != nil {
			return
		}
		if c := st.sup.store.Resolve(st.key); c == nil || c.State != "running" {
			st.endAll("container-exited")
			st.sup.remove(st)
			return
		}
		st.notifyAll("log stream ended, reconnecting...")
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
		t = 50
	}
}

func (st *logStream) appendRing(line wslc.LogLine) {
	st.ringMu.Lock()
	defer st.ringMu.Unlock()
	if len(st.ring) >= ringCap {
		copy(st.ring, st.ring[len(st.ring)-ringCap+1:])
		st.ring = st.ring[:ringCap-1]
	}
	st.ring = append(st.ring, line)
}

func (st *logStream) fanout(line wslc.LogLine) {
	st.subMu.Lock()
	defer st.subMu.Unlock()
	for sub := range st.subs {
		if sub.pause.Load() {
			continue
		}
		select {
		case sub.data <- line:
		default: // slow consumer: drop rather than stall the pump
		}
	}
}

func (st *logStream) notifyAll(msg string) {
	st.subMu.Lock()
	defer st.subMu.Unlock()
	for sub := range st.subs {
		select {
		case sub.notice <- msg:
		default:
		}
	}
}

func (st *logStream) endAll(reason string) {
	st.subMu.Lock()
	defer st.subMu.Unlock()
	for sub := range st.subs {
		select {
		case sub.end <- reason:
		default:
		}
	}
}
