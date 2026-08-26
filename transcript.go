package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"unicode"
)

// transcriptEntry covers the JSONL line shapes we care about: "summary"
// lines carry the auto-generated session title; "assistant" lines carry
// Claude's messages, whose last text block is its own report of the work.
type transcriptEntry struct {
	Type    string `json:"type"`
	Summary string `json:"summary"`
	Message struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
}

// transcriptInfo extracts the session title and the last assistant text
// (used as the work summary) from a Claude Code transcript.
func transcriptInfo(path string) (title, summary string) {
	if path == "" {
		return "", ""
	}
	const tailSize = 512 * 1024

	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return "", ""
	}
	offset := int64(0)
	if st.Size() > tailSize {
		offset = st.Size() - tailSize
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return "", ""
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return "", ""
	}

	lines := bytes.Split(data, []byte("\n"))
	if offset > 0 && len(lines) > 0 {
		lines = lines[1:] // first line is partial after seeking
	}
	title, summary = scanLines(lines)

	// Summary (title) lines often sit at the head of the file; if the tail
	// didn't have one, check the head too.
	if title == "" && offset > 0 {
		if _, err := f.Seek(0, io.SeekStart); err == nil {
			head := make([]byte, 64*1024)
			n, _ := io.ReadFull(f, head)
			headLines := bytes.Split(head[:n], []byte("\n"))
			if len(headLines) > 1 {
				headLines = headLines[:len(headLines)-1] // last line may be partial
			}
			t, _ := scanLines(headLines)
			title = t
		}
	}
	return title, summary
}

func scanLines(lines [][]byte) (title, lastAssistantText string) {
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var e transcriptEntry
		if json.Unmarshal(line, &e) != nil {
			continue
		}
		switch e.Type {
		case "summary":
			if e.Summary != "" {
				title = e.Summary
			}
		case "assistant":
			var parts []string
			for _, c := range e.Message.Content {
				if c.Type == "text" && strings.TrimSpace(c.Text) != "" {
					parts = append(parts, c.Text)
				}
			}
			if len(parts) > 0 {
				lastAssistantText = condense(strings.Join(parts, " "), 180)
			}
		}
	}
	return title, lastAssistantText
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
