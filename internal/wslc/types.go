// Package wslc wraps the wslc.exe CLI (WSL 3.0+ native containers) and
// normalizes its JSONL output into Wozzle's API models.
package wslc

// Container is the normalized container model served to the frontend.
type Container struct {
	ID         string            `json:"id"`
	ShortID    string            `json:"shortId"`
	Name       string            `json:"name"`
	Image      string            `json:"image,omitempty"`
	State      string            `json:"state"`
	Health     string            `json:"health,omitempty"`
	StatusText string            `json:"statusText,omitempty"`
	CreatedAt  string            `json:"createdAt,omitempty"`
	StartedAt  string            `json:"startedAt,omitempty"`
	Command    string            `json:"command,omitempty"`
	Ports      []PortMapping     `json:"ports,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	FullID     string            `json:"-"`
}

// PortMapping is a host->container port publication.
type PortMapping struct {
	Host      int    `json:"host"`
	Container int    `json:"container"`
	Protocol  string `json:"protocol"`
}

// Stat is a one-shot resource usage snapshot for one container.
type Stat struct {
	ID            string  `json:"id"`
	Name          string  `json:"-"`
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
	FullID        string  `json:"-"`
}

// LogLine is one container log line; TS is the parsed -t prefix if present.
type LogLine struct {
	TS   *string `json:"ts"`
	Text string  `json:"text"`
}

// Event is one wslc events stream record, e.g. "container start".
type Event struct {
	Type  string `json:"type"` // "container start", "network connect", ...
	ID    string `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image"`
	TS    string `json:"ts"`
	Raw   string `json:"raw,omitempty"`
}

// Image is one local image from `wslc images --format json`.
type Image struct {
	ID         string `json:"id"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	SizeBytes  uint64 `json:"sizeBytes"`
	CreatedAt  string `json:"createdAt"`
}

// SystemInfo answers GET /api/system/info.
type SystemInfo struct {
	WSLVersion    string `json:"wslVersion"`
	WSLcVersion   string `json:"wslcVersion"`
	Kernel        string `json:"kernel"`
	OS            string `json:"os"`
	WozzleVersion string `json:"wozzleVersion"`
	Running       int    `json:"running"`
	Total         int    `json:"total"`
}

// InspectData is the small subset of `wslc inspect` we consume.
type InspectData struct {
	ID    string `json:"Id"`
	Name  string `json:"Name"`
	State struct {
		ExitCode   int    `json:"ExitCode"`
		FinishedAt string `json:"FinishedAt"`
		StartedAt  string `json:"StartedAt"`
		Running    bool   `json:"Running"`
		Status     string `json:"Status"`
		Health     *struct {
			Status string `json:"Status"`
		} `json:"Health"`
	} `json:"State"`
}
