package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Token usage is aggregated from ~/.claude/projects/**/*.jsonl transcripts.
// Assistant entries carry message.usage; entries can be duplicated across
// files (resume/fork), so they are deduped by message.id + requestId.

type usageTotals struct {
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	CacheRead  int64 `json:"cacheRead"`
	CacheWrite int64 `json:"cacheWrite"`
}

func (t *usageTotals) add(e usageEntry) {
	t.Input += e.in
	t.Output += e.out
	t.CacheRead += e.cr
	t.CacheWrite += e.cw
}

type modelUsage struct {
	Model string `json:"model"`
	usageTotals
}

type usageReport struct {
	Today        usageTotals  `json:"today"`
	Week         usageTotals  `json:"week"`
	TodayByModel []modelUsage `json:"todayByModel"`
	UpdatedAt    time.Time    `json:"updatedAt"`
}

type usageEntry struct {
	t               time.Time
	model           string
	in, out, cr, cw int64
	key             string // dedupe key
}

type fileCache struct {
	size    int64
	mtime   time.Time
	entries []usageEntry
}

type usageScanner struct {
	mu       sync.Mutex
	files    map[string]*fileCache
	last     usageReport
	lastScan time.Time
}

var usage = &usageScanner{files: map[string]*fileCache{}}

// report returns the cached aggregate, rescanning at most every 20s.
func (u *usageScanner) report() usageReport {
	u.mu.Lock()
	defer u.mu.Unlock()
	if time.Since(u.lastScan) < 20*time.Second {
		return u.last
	}
	u.scanLocked()
	u.lastScan = time.Now()
	return u.last
}

func (u *usageScanner) scanLocked() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	now := time.Now()
	weekStart := now.Add(-7 * 24 * time.Hour)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	paths, _ := filepath.Glob(filepath.Join(home, ".claude", "projects", "*", "*.jsonl"))
	live := map[string]bool{}
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		// A file untouched since before the window holds nothing newer.
		if st.ModTime().Before(weekStart) {
			continue
		}
		live[p] = true
		c := u.files[p]
		if c != nil && c.size == st.Size() && c.mtime.Equal(st.ModTime()) {
			continue
		}
		u.files[p] = &fileCache{size: st.Size(), mtime: st.ModTime(), entries: parseUsageFile(p, weekStart)}
	}
	for p := range u.files {
		if !live[p] {
			delete(u.files, p)
		}
	}

	var r usageReport
	seen := map[string]bool{}
	byModel := map[string]*modelUsage{}
	for _, c := range u.files {
		for _, e := range c.entries {
			if e.key != "" && seen[e.key] {
				continue
			}
			if e.key != "" {
				seen[e.key] = true
			}
			if e.t.Before(weekStart) {
				continue
			}
			r.Week.add(e)
			if e.t.Before(dayStart) {
				continue
			}
			r.Today.add(e)
			m := byModel[e.model]
			if m == nil {
				m = &modelUsage{Model: e.model}
				byModel[e.model] = m
			}
			m.add(e)
		}
	}
	for _, m := range byModel {
		r.TodayByModel = append(r.TodayByModel, *m)
	}
	sort.Slice(r.TodayByModel, func(i, j int) bool {
		a, b := r.TodayByModel[i], r.TodayByModel[j]
		return a.Input+a.Output > b.Input+b.Output
	})
	r.UpdatedAt = now
	u.last = r
}

func parseUsageFile(path string, since time.Time) []usageEntry {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var out []usageEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if !bytes.Contains(line, []byte(`"assistant"`)) || !bytes.Contains(line, []byte(`"usage"`)) {
			continue
		}
		var e struct {
			Type      string    `json:"type"`
			Timestamp time.Time `json:"timestamp"`
			RequestID string    `json:"requestId"`
			Message   struct {
				ID    string `json:"id"`
				Model string `json:"model"`
				Usage struct {
					Input      int64 `json:"input_tokens"`
					Output     int64 `json:"output_tokens"`
					CacheWrite int64 `json:"cache_creation_input_tokens"`
					CacheRead  int64 `json:"cache_read_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if json.Unmarshal(line, &e) != nil || e.Type != "assistant" {
			continue
		}
		if e.Message.Model == "" || e.Message.Model == "<synthetic>" {
			continue
		}
		if e.Timestamp.Before(since) {
			continue
		}
		u := e.Message.Usage
		if u.Input+u.Output+u.CacheRead+u.CacheWrite == 0 {
			continue
		}
		key := ""
		if e.Message.ID != "" {
			key = e.Message.ID + ":" + e.RequestID
		}
		out = append(out, usageEntry{
			t: e.Timestamp.Local(), model: e.Message.Model,
			in: u.Input, out: u.Output, cr: u.CacheRead, cw: u.CacheWrite, key: key,
		})
	}
	return out
}
