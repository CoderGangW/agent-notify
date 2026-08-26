package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// rawEntry covers the JSONL line shapes we care about. "summary" lines
// carry a session title (only present after compaction/resume);
// "assistant" lines carry Claude's messages; "user" lines give us the
// first prompt (title fallback, same thing the resume picker shows) and
// the last request (context for the AI summary). Content is either a
// plain string or an array of typed blocks.
type rawEntry struct {
	Type    string `json:"type"`
	IsMeta  bool   `json:"isMeta"`
	Summary string `json:"summary"`
	Message struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

func contentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		t := strings.TrimSpace(b.Text)
		// Drop injected non-prompt text (system reminders, command XML).
		if b.Type != "text" || t == "" || strings.HasPrefix(t, "<") {
			continue
		}
		parts = append(parts, t)
	}
	return strings.Join(parts, " ")
}

// transcriptDetail is what a transcript gives us for one notification.
type transcriptDetail struct {
	Title         string // session title (summary entry, else first prompt)
	LastUser      string // most recent user request, for summary context
	LastAssistant string // Claude's final message = its report of the work
}

func transcriptInfo(path string) transcriptDetail {
	var d transcriptDetail
	if path == "" {
		return d
	}
	const tailSize = 512 * 1024

	f, err := os.Open(path)
	if err != nil {
		return d
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return d
	}
	offset := int64(0)
	if st.Size() > tailSize {
		offset = st.Size() - tailSize
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return d
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return d
	}

	lines := bytes.Split(data, []byte("\n"))
	if offset > 0 && len(lines) > 0 {
		lines = lines[1:] // first line is partial after seeking
	}
	tail := scanLines(lines)
	d.Title = tail.title
	d.LastUser = tail.lastUser
	d.LastAssistant = tail.lastAssistant

	firstUser := tail.firstUser
	if offset > 0 {
		// Titles and the first prompt live at the head of the file.
		firstUser = ""
		if _, err := f.Seek(0, io.SeekStart); err == nil {
			head := make([]byte, 64*1024)
			n, _ := io.ReadFull(f, head)
			headLines := bytes.Split(head[:n], []byte("\n"))
			if len(headLines) > 1 {
				headLines = headLines[:len(headLines)-1] // last line may be partial
			}
			hs := scanLines(headLines)
			if d.Title == "" {
				d.Title = hs.title
			}
			firstUser = hs.firstUser
		}
	}
	if d.Title == "" {
		d.Title = condense(firstUser, 60)
	}
	return d
}

type scanResult struct {
	title         string
	firstUser     string
	lastUser      string
	lastAssistant string
}

func scanLines(lines [][]byte) scanResult {
	var r scanResult
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var e rawEntry
		if json.Unmarshal(line, &e) != nil || e.IsMeta {
			continue
		}
		switch e.Type {
		case "summary":
			if e.Summary != "" {
				r.title = e.Summary
			}
		case "user":
			if t := contentText(e.Message.Content); t != "" {
				if r.firstUser == "" {
					r.firstUser = t
				}
				r.lastUser = clip(t, 1000)
			}
		case "assistant":
			if t := contentText(e.Message.Content); t != "" {
				r.lastAssistant = clip(t, 4000)
			}
		}
	}
	return r
}

// sessionName returns the session's name from ~/.claude/sessions when the
// user set it explicitly (nameSource != "derived"; derived names are just
// directory-based like "personal-53").
func sessionName(sessionID string) string {
	if sessionID == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	entries, err := os.ReadDir(filepath.Join(home, ".claude", "sessions"))
	if err != nil {
		return ""
	}
	for _, ent := range entries {
		if !strings.HasSuffix(ent.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(home, ".claude", "sessions", ent.Name()))
		if err != nil || len(data) > 64*1024 {
			continue
		}
		var s struct {
			SessionID  string `json:"sessionId"`
			Name       string `json:"name"`
			NameSource string `json:"nameSource"`
		}
		if json.Unmarshal(data, &s) != nil {
			continue
		}
		if s.SessionID == sessionID && s.Name != "" && s.NameSource != "derived" {
			return s.Name
		}
	}
	return ""
}

func clip(s string, max int) string {
	r := []rune(s)
	if len(r) > max {
		return string(r[:max])
	}
	return s
}

// condense collapses whitespace/markdown noise into a single notification-
// sized line.
func condense(s string, max int) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			space = true
			continue
		}
		if r == '`' || r == '*' || r == '#' {
			continue
		}
		if space && b.Len() > 0 {
			b.WriteRune(' ')
		}
		space = false
		b.WriteRune(r)
	}
	out := []rune(b.String())
	if len(out) > max {
		return string(out[:max-1]) + "…"
	}
	return string(out)
}
