package record

import (
	"strings"
	"testing"
	"time"
)

func TestEncodeParseRoundTrip(t *testing.T) {
	createdAt := time.Date(2026, 8, 4, 14, 30, 12, 0, time.FixedZone("EDT", -4*60*60))
	common := Common{
		ID: "20260804T143012-broken-doc-link-a7c2", CreatedAt: createdAt,
		Summary: "Broken doc link", Tags: []string{"paper/cut", "docs"},
		Project:   Project{Name: "paper", Key: "github.com/gabe/paper", Root: "/tmp/paper", Remote: "git@github.com:gabe/paper.git"},
		Directory: "internal/record", Git: &Git{Branch: "main", Commit: "abc123"}, Agent: "opencode",
	}
	open, err := NewOpenCut(common, CutFields{Kind: CutKindBrokenLink, Description: "The link is gone.", Workaround: "Read source."})
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := Resolve(open, createdAt.Add(time.Hour), "Updated the link.")
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := Encode(resolved)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(encoded)
	if err != nil {
		t.Fatal(err)
	}
	cut, ok := parsed.(ResolvedCut)
	if !ok {
		t.Fatalf("got %T, want ResolvedCut", parsed)
	}
	if cut.Resolution != "Updated the link." || cut.Workaround != "Read source." || cut.Project.Key != common.Project.Key {
		t.Fatalf("unexpected parsed cut: %#v", cut)
	}
}

func TestParseRejectsUnknownFrontmatter(t *testing.T) {
	document := `---
id: 20260804T143012-work-a7c2
type: work
created_at: 2026-08-04T14:30:12-04:00
summary: Work
surprise: true
project:
  name: paper
  key: paper
  root: /tmp/paper
---
`
	_, err := Parse([]byte(document))
	if err == nil || !strings.Contains(err.Error(), "field surprise not found") {
		t.Fatalf("expected unknown-field error, got %v", err)
	}
}

func TestParseRejectsIncompleteResolvedCut(t *testing.T) {
	document := `---
id: 20260804T143012-cut-a7c2
type: cut
created_at: 2026-08-04T14:30:12-04:00
status: resolved
resolved_at: 2026-08-04T15:30:12-04:00
summary: Cut
kind: other
project:
  name: paper
  key: paper
  root: /tmp/paper
---

## Description

Something broke.
`
	_, err := Parse([]byte(document))
	if err == nil || !strings.Contains(err.Error(), "resolution must not be empty") {
		t.Fatalf("expected resolution error, got %v", err)
	}
}

func TestNarrativeHeadingsRoundTrip(t *testing.T) {
	common := Common{
		ID: "20260804T143012-work-a7c2", CreatedAt: time.Date(2026, 8, 4, 14, 30, 12, 0, time.UTC),
		Summary: "Work", Project: Project{Name: "paper", Key: "paper", Root: "/tmp/paper"},
	}
	details := "Implemented parser.\n\n## Verification\n\nTests pass.\n\n## Description\n\nThis is narrative, not cut metadata."
	work, err := NewWork(common, details)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := Encode(work)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.(Work).Details != details {
		t.Fatalf("details changed during round trip:\n%s", parsed.(Work).Details)
	}
}

func TestLiteralEscapedHeadingRoundTrip(t *testing.T) {
	common := Common{
		ID: "20260804T143012-work-a7c2", CreatedAt: time.Date(2026, 8, 4, 14, 30, 12, 0, time.UTC),
		Summary: "Work", Project: Project{Name: "paper", Key: "paper", Root: "/tmp/paper"},
	}
	details := `Keep this literal: \## Description`
	work, err := NewWork(common, details)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := Encode(work)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.(Work).Details != details {
		t.Fatalf("got %q, want %q", parsed.(Work).Details, details)
	}
}

func TestParseIDRejectsImpossibleTimestamp(t *testing.T) {
	if _, err := ParseID("20261399T296099-work-a7c2"); err == nil {
		t.Fatal("expected impossible timestamp error")
	}
}
