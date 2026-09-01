package tui

import (
	"strings"
	"testing"
)

func TestParseConv(t *testing.T) {
	line := `conv id=abc-123 kind=direct title=alice`
	c, err := ParseConv(line)
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != "abc-123" {
		t.Errorf("id: %q", c.ID)
	}
	if c.Kind != "direct" {
		t.Errorf("kind: %q", c.Kind)
	}
	if c.Title != "alice" {
		t.Errorf("title: %q", c.Title)
	}
}

func TestParseMessage(t *testing.T) {
	line := `msg id=m1 sender=u1 ts=1735732800000 content="hello world"`
	m, err := ParseMessage(line)
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "m1" || m.Sender != "u1" || m.Content != "hello world" {
		t.Errorf("bad: %+v", m)
	}
	if m.CreatedAt.UnixMilli() != 1735732800000 {
		t.Errorf("ts: %d", m.CreatedAt.UnixMilli())
	}
}

func TestParseMessageWithSpaces(t *testing.T) {
	line := `msg id=m1 sender=u1 ts=100 content="multiple  spaces  here"`
	m, err := ParseMessage(line)
	if err != nil {
		t.Fatal(err)
	}
	if m.Content != "multiple  spaces  here" {
		t.Errorf("content: %q", m.Content)
	}
}

func TestIsEndLine(t *testing.T) {
	cases := map[string]bool{
		"end (3)":     true,
		"(3)":         true,
		"msg id=x":    false,
		"end other":   false,
		"unrelated":   false,
	}
	for line, want := range cases {
		if got := IsEndLine(line); got != want {
			t.Errorf("%q: want %v, got %v", line, want, got)
		}
	}
}

func TestExtractCount(t *testing.T) {
	if n := ExtractCount("end (5)"); n != 5 {
		t.Errorf("end: %d", n)
	}
	if n := ExtractCount("(12)"); n != 12 {
		t.Errorf("paren: %d", n)
	}
}

func TestParseMessageMissingContent(t *testing.T) {
	_, err := ParseMessage("msg id=m1 sender=u1 ts=100")
	if err == nil || !strings.Contains(err.Error(), "content") {
		t.Fatalf("want content error, got %v", err)
	}
}