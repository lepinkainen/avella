package template

import (
	"testing"
	"time"
)

func TestNewFileData(t *testing.T) {
	ts := time.Date(2025, 9, 15, 10, 0, 0, 0, time.UTC)
	data := newFileData("/downloads/movie.mkv", ts)

	if data.Year != "2025" {
		t.Errorf("Year = %q, want %q", data.Year, "2025")
	}
	if data.Month != "09" {
		t.Errorf("Month = %q, want %q", data.Month, "09")
	}
	if data.Day != "15" {
		t.Errorf("Day = %q, want %q", data.Day, "15")
	}
	if data.Ext != "mkv" {
		t.Errorf("Ext = %q, want %q", data.Ext, "mkv")
	}
	if data.Type != "Video" {
		t.Errorf("Type = %q, want %q", data.Type, "Video")
	}
	if data.Filename != "movie.mkv" {
		t.Errorf("Filename = %q, want %q", data.Filename, "movie.mkv")
	}
}

func TestExpand(t *testing.T) {
	data := FileData{
		Year:  "2025",
		Month: "09",
		Day:   "15",
		Ext:   "mkv",
		Type:  "Video",
	}

	tests := []struct {
		name string
		tmpl string
		want string
	}{
		{"year-month type", "~/Media/{{.Year}}-{{.Month}} {{.Type}}", "~/Media/2025-09 Video"},
		{"all fields", "{{.Year}}/{{.Month}}/{{.Day}}/{{.Type}}/{{.Ext}}", "2025/09/15/Video/mkv"},
		{"no placeholders", "~/Media/Unsorted", "~/Media/Unsorted"},
		{"just type", "~/{{.Type}}", "~/Video"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Expand(tt.tmpl, data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Expand(%q) = %q, want %q", tt.tmpl, got, tt.want)
			}
		})
	}
}

func TestExpandInvalidTemplate(t *testing.T) {
	data := FileData{}
	_, err := Expand("{{.Invalid", data)
	if err == nil {
		t.Error("expected error for invalid template")
	}
}

func TestHasPlaceholders(t *testing.T) {
	if !HasPlaceholders("~/Media/{{.Year}}") {
		t.Error("should detect placeholders")
	}
	if HasPlaceholders("~/Media/Unsorted") {
		t.Error("should not detect placeholders in plain path")
	}
}

func TestClassifyExt(t *testing.T) {
	tests := []struct {
		ext  string
		want string
	}{
		{"mkv", "Video"},
		{"MKV", "Video"},
		{"jpg", "Image"},
		{"mp3", "Audio"},
		{"pdf", "Document"},
		{"zip", "Archive"},
		{"xyz", "Other"},
		{"", "Other"},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := ClassifyExt(tt.ext)
			if got != tt.want {
				t.Errorf("ClassifyExt(%q) = %q, want %q", tt.ext, got, tt.want)
			}
		})
	}
}

func TestNewFileDataNoExtension(t *testing.T) {
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	data := newFileData("/downloads/Makefile", ts)

	if data.Ext != "" {
		t.Errorf("Ext = %q, want empty", data.Ext)
	}
	if data.Type != "Other" {
		t.Errorf("Type = %q, want %q", data.Type, "Other")
	}
}
