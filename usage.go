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

// statPoint is one bucket of the usage time series.
type statPoint struct {
	T   string `json:"t"` // "2006-01-02T15" | "2006-01-02" | "2006-01"
	In  int64  `json:"in"`
	Out int64  `json:"out"`
}

type statsReport struct {
	Hourly  []statPoint `json:"hourly"`  // last 48h
	Daily   []statPoint `json:"daily"`   // last 30d (live + persisted history)
	Monthly []statPoint `json:"monthly"` // everything known
}

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
	mu          sync.Mutex
	files       map[string]*fileCache
	last        usageReport
	lastScan    time.Time
	lastPersist time.Time
}

var usage = &usageScanner{files: map[string]*fileCache{}}

func statsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude-notify", "stats.json")
}

type dailyTotals map[string]*statPoint // "2006-01-02" → totals

func loadStatsHistory() dailyTotals {
	h := dailyTotals{}
	if p := statsPath(); p != "" {
		if data, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(data, &h)
		}
	}
	return h
}

// stats buckets the cached entries and merges the persisted daily history
// so days whose transcripts Claude has cleaned up survive. Live data wins
// for days it still covers; the merged history is written back (throttled
// by the caller's report() cadence).
func (u *usageScanner) stats() statsReport {
	u.mu.Lock()
	defer u.mu.Unlock()
	if time.Since(u.lastScan) >= 20*time.Second {
		u.scanLocked()
		u.lastScan = time.Now()
	}

	now := time.Now()
	hourCut := now.Add(-48 * time.Hour)
	seen := map[string]bool{}
	hourly := map[string]*statPoint{}
	liveDaily := dailyTotals{}
	for _, c := range u.files {
		for _, e := range c.entries {
			if e.key != "" {
				if seen[e.key] {
					continue
				}
				seen[e.key] = true
			}
			day := e.t.Format("2006-01-02")
			dp := liveDaily[day]
			if dp == nil {
				dp = &statPoint{T: day}
				liveDaily[day] = dp
			}
			dp.In += e.in
			dp.Out += e.out
			if e.t.After(hourCut) {
				hk := e.t.Format("2006-01-02T15")
				hp := hourly[hk]
				if hp == nil {
					hp = &statPoint{T: hk}
					hourly[hk] = hp
				}
				hp.In += e.in
				hp.Out += e.out
			}
		}
	}

	hist := loadStatsHistory()
	for d, p := range liveDaily {
		hist[d] = p // live scan is authoritative while transcripts exist
	}
	if p := statsPath(); p != "" && time.Since(u.lastPersist) > 10*time.Minute {
		u.lastPersist = now
		if data, err := json.MarshalIndent(hist, "", " "); err == nil {
			_ = os.WriteFile(p, data, 0o644)
		}
	}

	var r statsReport
	for i := 47; i >= 0; i-- {
		hk := now.Add(-time.Duration(i) * time.Hour).Format("2006-01-02T15")
		p := hourly[hk]
		if p == nil {
			p = &statPoint{T: hk}
		}
		r.Hourly = append(r.Hourly, *p)
	}
	for i := 29; i >= 0; i-- {
		day := now.AddDate(0, 0, -i).Format("2006-01-02")
		p := hist[day]
		if p == nil {
			p = &statPoint{T: day}
		}
		q := *p
		q.T = day
		r.Daily = append(r.Daily, q)
	}
	monthly := map[string]*statPoint{}
	for d, p := range hist {
		mk := d[:7]
		mp := monthly[mk]
		if mp == nil {
			mp = &statPoint{T: mk}
			monthly[mk] = mp
		}
		mp.In += p.In
		mp.Out += p.Out
	}
	for _, p := range monthly {
		r.Monthly = append(r.Monthly, *p)
	}
	sort.Slice(r.Monthly, func(i, j int) bool { return r.Monthly[i].T < r.Monthly[j].T })
	if len(r.Monthly) > 24 {
		r.Monthly = r.Monthly[len(r.Monthly)-24:]
	}
	return r
}

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
