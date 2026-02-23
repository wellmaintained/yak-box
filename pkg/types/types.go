// Package types defines shared data structures for yak-box.
package types

import (
	"path/filepath"
	"strings"
	"time"
)

// WorkerNames is the pool of available worker names.
// These are simple identifiers used for display and home directory isolation.
var WorkerNames = []string{"Yakriel", "Yakueline", "Yakov", "Yakira"}

type Worker struct {
	Name          string
	WorkerName    string // Yak-shaver identity (e.g. "Yakriel")
	DisplayName   string
	ContainerName string
	Runtime       string
	CWD           string
	YakPath       string
	Tasks         []string
	SpawnedAt     time.Time
	SessionName   string
	WorktreePath  string // Path to git worktree (if using --auto-worktree)
	PidFile       string // Path to PID file for native workers
	Tool          string // Tool to use: "opencode", "claude", or "cursor"
	Model         string // Optional model name passed through to the selected tool
}

// SlugifyTaskPath converts a task display name path (e.g. "fixes/tab emoji")
// to the slugified directory path used under .yaks/ (e.g. "fixes/tab-emoji").
func SlugifyTaskPath(taskPath string) string {
	parts := strings.Split(filepath.ToSlash(taskPath), "/")
	for i, part := range parts {
		parts[i] = strings.ReplaceAll(part, " ", "-")
	}
	return filepath.Join(parts...)
}

type ResourceProfile struct {
	Name   string
	CPUs   string
	Memory string
	Swap   string
	PIDs   int
	Tmpfs  map[string]string
}
