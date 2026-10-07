package server

import (
	"bufio"
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"wozzle/internal/store"
	"wozzle/internal/wslc"
)

// Server wires the wslc provider, store and HTTP/WS endpoints together.
type Server struct {
	provider     *wslc.CLI
	store        *store.Store
	dist         fs.FS
	version      string
	eventHub     *hub
	statsHub     *hub
	logs         *LogSupervisor
	statsClients atomic.Int32
	lastStats    atomic.Value // statsFrame
	cancel       context.CancelFunc
}

// hub fans out typed values to websocket subscribers.
type hub struct {
	mu   sync.Mutex
	subs []chan any
}

func newHub() *hub { return &hub{} }

func (h *hub) subscribe() chan any {
	ch := make(chan any, 64)
	h.mu.Lock()
	h.subs = append(h.subs, ch)
	h.mu.Unlock()
	return ch
}

func (h *hub) unsubscribe(ch chan any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, s := range h.subs {
		if s == ch {
			h.subs = append(h.subs[:i], h.subs[i+1:]...)
			break
		}
	}
}

func (h *hub) publish(v any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.subs {
		select {
		case ch <- v:
		default:
		}
	}
}

type statsFrame struct {
	T     string      `json:"t"`
	TS    string      `json:"ts"`
	Stats []wslc.Stat `json:"stats"`
}

type statsError struct {
	T     string `json:"t"`
	Error string `json:"error"`
}

// New builds the server and starts its background loops.
func New(p *wslc.CLI, st *store.Store, dist fs.FS, version string) *Server {
	s := &Server{
		provider: p,
		store:    st,
		dist:     dist,
		version:  version,
		eventHub: newHub(),
		statsHub: newHub(),
	}
	s.logs = NewLogSupervisor(p, st)
	if err := s.refresh(); err != nil {
		log.Printf("initial container refresh: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go s.refreshLoop(ctx)
	go s.eventPump(ctx)
	go s.statsLoop(ctx)
	return s
}

// Close stops background loops and log streams.
func (s *Server) Close() {
	if s.cancel != nil {
		s.cancel()
	}
	s.logs.Close()
}

func (s *Server) refresh() error {
	list, err := s.provider.Containers(context.Background())
	if err != nil {
		return err
	}
	s.store.Replace(list)
	return nil
}

func (s *Server) refreshLoop(ctx context.Context) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := s.refresh(); err != nil {
				log.Printf("refresh: %v", err)
			}
		}
	}
}

func (s *Server) eventPump(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		cmd, stdout, err := s.provider.EventsStream(ctx)
		if err != nil {
			log.Printf("events: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" {
				continue
			}
			if ev, ok := wslc.ParseEvent(line); ok {
				s.eventHub.publish(ev)
				if strings.HasPrefix(ev.Type, "container") {
					go func() {
						if err := s.refresh(); err != nil {
							log.Printf("refresh: %v", err)
						}
					}()
				}
			}
		}
		_ = cmd.Wait()
		if ctx.Err() != nil {
			return
		}
		log.Printf("events stream ended, restarting in 2s")
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func (s *Server) statsLoop(ctx context.Context) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if s.statsClients.Load() == 0 {
				continue
			}
			stats, err := s.provider.Stats(ctx)
			if err != nil {
				s.statsHub.publish(statsError{T: "statsError", Error: err.Error()})
				continue
			}
			for i := range stats {
				if c := s.store.Resolve(stats[i].Name); c != nil {
					stats[i].ID = c.ID
				} else if stats[i].ID == "" {
					stats[i].ID = stats[i].FullID
				}
			}
			if stats == nil {
				stats = []wslc.Stat{}
			}
			fr := statsFrame{T: "stats", TS: time.Now().UTC().Format(time.RFC3339), Stats: stats}
			s.lastStats.Store(fr)
			s.statsHub.publish(fr)
		}
	}
}

// Routes builds the full HTTP handler.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/system/info", s.handleSystemInfo)
	mux.HandleFunc("GET /api/containers", s.handleContainers)
	mux.HandleFunc("GET /api/containers/{id}", s.handleContainer)
	mux.HandleFunc("GET /api/containers/{id}/logs", s.handleContainerLogsHistory)
	mux.HandleFunc("GET /api/images", s.handleImages)
	mux.HandleFunc("POST /api/containers/{id}/{action}", s.handleAction)
	mux.HandleFunc("DELETE /api/containers/{id}", s.handleDelete)
	mux.HandleFunc("GET /api/ws/logs/{id}", s.handleWSLogs)
	mux.HandleFunc("GET /api/ws/stats", s.handleWSStats)
	mux.HandleFunc("GET /api/ws/events", s.handleWSEvents)
	mux.HandleFunc("GET /api/ws/exec/{id}", s.handleWSExec)
	mux.Handle("/", s.spa())
	return mux
}

// ---- REST handlers ----

func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	info, err := s.provider.SystemInfo(r.Context())
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	running, total := s.store.Count()
	info.WozzleVersion = s.version
	info.Running, info.Total = running, total
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleContainers(w http.ResponseWriter, r *http.Request) {
	list := s.store.All()
	writeJSON(w, http.StatusOK, list) // marshals as [] when empty
}

type containerInspectResp struct {
	Container wslc.Container  `json:"container"`
	Inspect   json.RawMessage `json:"inspect"`
}

func (s *Server) handleContainer(w http.ResponseWriter, r *http.Request) {
	cont := s.store.Resolve(r.PathValue("id"))
	if cont == nil {
		httpError(w, http.StatusNotFound, "container not found")
		return
	}
	raw, d, err := s.provider.Inspect(r.Context(), cont.ID)
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	merged := *cont
	if d != nil {
		if d.State.StartedAt != "" && d.State.StartedAt != "0001-01-01T00:00:00Z" {
			merged.StartedAt = d.State.StartedAt
		}
		if d.State.Health != nil && d.State.Health.Status != "" {
			merged.Health = d.State.Health.Status
		}
		merged.FullID = d.ID
	}
	writeJSON(w, http.StatusOK, containerInspectResp{Container: merged, Inspect: raw})
}

type logsResp struct {
	Lines []wslc.LogLine `json:"lines"`
}

func (s *Server) handleContainerLogsHistory(w http.ResponseWriter, r *http.Request) {
	cont := s.store.Resolve(r.PathValue("id"))
	if cont == nil {
		httpError(w, http.StatusNotFound, "container not found")
		return
	}
	q := r.URL.Query()
	tail := clampInt(queryInt(r, "tail", 300), 1, 5000)
	lines, err := s.provider.Logs(r.Context(), cont.ID, wslc.LogsOptions{
		Tail:  tail,
		Since: q.Get("since"),
		Until: q.Get("until"),
	})
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if lines == nil {
		lines = []wslc.LogLine{}
	}
	writeJSON(w, http.StatusOK, logsResp{Lines: lines})
}

func (s *Server) handleImages(w http.ResponseWriter, r *http.Request) {
	images, err := s.provider.Images(r.Context())
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, images)
}

type okResp struct {
	OK bool `json:"ok"`
}

func (s *Server) handleAction(w http.ResponseWriter, r *http.Request) {
	action := r.PathValue("action")
	switch action {
	case "start", "stop", "kill", "restart":
	default:
		httpError(w, http.StatusBadRequest, "unsupported action")
		return
	}
	if r.URL.Query().Get("confirm") != "1" {
		httpError(w, http.StatusBadRequest, "missing confirm=1")
		return
	}
	cont := s.store.Resolve(r.PathValue("id"))
	if cont == nil {
		httpError(w, http.StatusNotFound, "container not found")
		return
	}
	if err := s.provider.Action(r.Context(), action, cont.ID); err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	go func() { _ = s.refresh() }()
	writeJSON(w, http.StatusOK, okResp{OK: true})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("confirm") != "1" {
		httpError(w, http.StatusBadRequest, "missing confirm=1")
		return
	}
	cont := s.store.Resolve(r.PathValue("id"))
	if cont == nil {
		httpError(w, http.StatusNotFound, "container not found")
		return
	}
	if err := s.provider.Remove(r.Context(), cont.ID); err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	go func() { _ = s.refresh() }()
	writeJSON(w, http.StatusOK, okResp{OK: true})
}

// ---- static SPA ----

func (s *Server) spa() http.Handler {
	fileServer := http.FileServer(http.FS(s.dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(s.dist, p); err != nil {
			b, err := fs.ReadFile(s.dist, "index.html")
			if err != nil {
				http.Error(w, "no frontend build", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(b)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

// ---- helpers ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func queryInt(r *http.Request, key string, def int) int {
	if v := r.URL.Query().Get(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func clampInt(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}
