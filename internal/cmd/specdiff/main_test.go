package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestIssueTitle(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Bot API 10.4", "Support Telegram Bot API 10.4"},
		{"Telegram Bot API 10.4", "Support Telegram Bot API 10.4"},
		{"10.4", "Support Telegram Bot API 10.4"},
		{"Support Telegram Bot API 10.4", "Support Telegram Bot API 10.4"},
		{"", "Support new Telegram Bot API version"},
		{"   ", "Support new Telegram Bot API version"},
	}

	for _, tt := range tests {
		got := issueTitle(tt.input)
		if got != tt.want {
			t.Errorf("issueTitle(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestChangelogLink(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "[Official changelog](https://core.telegram.org/bots/api-changelog)"},
		{"https://core.telegram.org/bots/api-changelog", "[Official changelog](https://core.telegram.org/bots/api-changelog)"},
		{
			"https://core.telegram.org/bots/api#august-24-2026",
			"[Official changelog](https://core.telegram.org/bots/api-changelog) · [Release notes](https://core.telegram.org/bots/api#august-24-2026)",
		},
	}

	for _, tt := range tests {
		got := changelogLink(tt.input)
		if got != tt.want {
			t.Errorf("changelogLink(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatMarkdown(t *testing.T) {
	s := &spec{
		Version:     "Bot API 10.4",
		ReleaseDate: "October 12, 2026",
		Changelog:   "https://core.telegram.org/bots/api#october-12-2026",
	}

	// Case 1: Nothing missing
	mdEmpty := formatMarkdown(s, nil, nil, false)
	if !strings.Contains(mdEmpty, "Nothing is missing.") {
		t.Errorf("expected 'Nothing is missing.', got:\n%s", mdEmpty)
	}
	if !strings.Contains(mdEmpty, "[Official changelog](https://core.telegram.org/bots/api-changelog)") {
		t.Errorf("expected official changelog link, got:\n%s", mdEmpty)
	}

	// Case 2: Missing items
	missing := []string{"type NewType", "method newMethod"}
	extra := []string{"field Old.Field"}
	md := formatMarkdown(s, missing, extra, true)

	if !strings.Contains(md, "Telegram Bot API 10.4 (October 12, 2026)") {
		t.Errorf("missing title in markdown:\n%s", md)
	}
	if !strings.Contains(md, "### Missing (2)") {
		t.Errorf("missing section header in markdown:\n%s", md)
	}
	if !strings.Contains(md, "- type NewType") || !strings.Contains(md, "- method newMethod") {
		t.Errorf("missing bullet points in markdown:\n%s", md)
	}
	if !strings.Contains(md, "### Not in the spec (1)") {
		t.Errorf("missing extra section header in markdown:\n%s", md)
	}
	if !strings.Contains(md, "- field Old.Field") {
		t.Errorf("missing extra bullet point in markdown:\n%s", md)
	}
}

func TestFormatJSON(t *testing.T) {
	s := &spec{
		Version:     "Bot API 10.4",
		ReleaseDate: "October 12, 2026",
		Changelog:   "https://core.telegram.org/bots/api#october-12-2026",
	}
	missing := []string{"type NewType"}

	data, err := formatJSON(s, missing, nil, false)
	if err != nil {
		t.Fatalf("formatJSON failed: %v", err)
	}

	var out jsonOutput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if out.Version != "Bot API 10.4" {
		t.Errorf("unexpected version: %q", out.Version)
	}
	if out.Title != "Support Telegram Bot API 10.4" {
		t.Errorf("unexpected title: %q", out.Title)
	}
	if len(out.Missing) != 1 || out.Missing[0] != "type NewType" {
		t.Errorf("unexpected missing: %v", out.Missing)
	}
	if !strings.Contains(out.Markdown, "### Missing (1)") {
		t.Errorf("unexpected markdown in JSON: %q", out.Markdown)
	}
}

func TestFormatText(t *testing.T) {
	s := &spec{
		Version:     "Bot API 10.4",
		ReleaseDate: "October 12, 2026",
	}
	missing := []string{"type NewType"}

	text := formatText(s, missing, nil, false)
	expected := "Telegram Bot API 10.4 (October 12, 2026)\n\nMissing (1):\n  - type NewType\n"
	if text != expected {
		t.Errorf("got:\n%q\nwant:\n%q", text, expected)
	}
}
