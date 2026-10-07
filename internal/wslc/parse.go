package wslc

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	logLineRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2}))\s(.*)$`)
	eventRe   = regexp.MustCompile(`^(\S+)\s+(\S+)\s+(\S+)\s+(\S+)(?:\s+\((.*)\))?$`)
	portRe    = regexp.MustCompile(`(?:::|[\d.]+):(\d+)->(\d+)/(tcp|udp)`)
)

// ParseLogLine splits a `wslc logs -t` line into timestamp + text.
func ParseLogLine(s string) LogLine {
	if m := logLineRe.FindStringSubmatch(s); m != nil {
		ts := m[1]
		return LogLine{TS: &ts, Text: m[2]}
	}
	return LogLine{TS: nil, Text: s}
}

// ParseEvent parses a `wslc events` line like
// "2026-10-07T16:34:17.000000000+08:00 container start <id> (image=..., name=...)".
func ParseEvent(line string) (Event, bool) {
	m := eventRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return Event{}, false
	}
	ev := Event{TS: m[1], Type: m[2] + " " + m[3], ID: m[4], Raw: line}
	if m[5] != "" {
		for _, kv := range strings.Split(m[5], ", ") {
			if i := strings.Index(kv, "="); i > 0 {
				switch kv[:i] {
				case "name":
					ev.Name = kv[i+1:]
				case "image":
					ev.Image = kv[i+1:]
				}
			}
		}
	}
	return ev, true
}

// ParsePorts parses docker-style port strings, e.g. "0.0.0.0:8080->80/tcp, :::8080->80/tcp".
func ParsePorts(s string) []PortMapping {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []PortMapping
	for _, m := range portRe.FindAllStringSubmatch(s, -1) {
		h, _ := strconv.Atoi(m[1])
		c, _ := strconv.Atoi(m[2])
		out = append(out, PortMapping{Host: h, Container: c, Protocol: m[3]})
	}
	return out
}

// ParsePercent parses "0.06%" into 0.06.
func ParsePercent(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "%")), 64)
	return f
}

// ParseSize parses wslc human sizes: "3.543MiB", "1.04kB", "15.32GiB", "0B".
func ParseSize(s string) uint64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "<none>" || s == "N/A" {
		return 0
	}
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.' || s[i] == '-') {
		i++
	}
	num, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0
	}
	var mult float64 = 1
	switch strings.TrimSpace(s[i:]) {
	case "B":
		mult = 1
	case "kB", "KB":
		mult = 1000
	case "kiB", "KiB":
		mult = 1024
	case "MB":
		mult = 1000 * 1000
	case "MiB":
		mult = 1024 * 1024
	case "GB":
		mult = 1000 * 1000 * 1000
	case "GiB":
		mult = 1024 * 1024 * 1024
	case "TB":
		mult = 1000 * 1000 * 1000 * 1000
	case "TiB":
		mult = 1024 * 1024 * 1024 * 1024
	default:
		mult = 1
	}
	if num < 0 {
		return 0
	}
	return uint64(num * mult)
}

// ParseSizePair parses "3.543MiB / 15.32GiB" into (used, total).
func ParseSizePair(s string) (uint64, uint64) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return ParseSize(s), 0
	}
	return ParseSize(parts[0]), ParseSize(parts[1])
}

// ParseCreatedAt normalizes wslc list timestamps like
// "2026-10-07 16:33:45 +0800 GMT+8" into RFC3339 UTC.
func ParseCreatedAt(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	layouts := []string{
		"2006-01-02 15:04:05 -0700 GMT+8",
		"2006-01-02 15:04:05 -0700 GMT",
		"2006-01-02 15:04:05 -0700",
		time.RFC3339Nano,
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	if len(s) >= 19 {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", s[:19], time.Local); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return s
}
