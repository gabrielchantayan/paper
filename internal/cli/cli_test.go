package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gabrielchantayan/paper/internal/config"
	"github.com/gabrielchantayan/paper/internal/record"
	"github.com/gabrielchantayan/paper/internal/store"
)

func TestWorkAndCutLifecycle(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("PAPER_VAULT", vault)

	workOutput := run(t, nil, "log", "--summary", "Implemented lifecycle", "--details", "Tests pass.", "--format", "json")
	var work outputRecord
	if err := json.Unmarshal([]byte(workOutput), &work); err != nil {
		t.Fatal(err)
	}
	if work.Type != "work" || work.Summary != "Implemented lifecycle" {
		t.Fatalf("unexpected work output: %#v", work)
	}

	cutOutput := run(t, strings.NewReader("The documentation URL returns 404."), "cut", "--summary", "Broken documentation", "--description", "-", "--kind", "broken-link", "--format", "json")
	var cut outputRecord
	if err := json.Unmarshal([]byte(cutOutput), &cut); err != nil {
		t.Fatal(err)
	}
	if cut.Status != "open" || cut.Description == "" {
		t.Fatalf("unexpected cut output: %#v", cut)
	}

	cuts := run(t, nil, "cuts", "--format", "json")
	var open []outputRecord
	if err := json.Unmarshal([]byte(cuts), &open); err != nil {
		t.Fatal(err)
	}
	if len(open) != 1 {
		t.Fatalf("got %d open cuts, want 1", len(open))
	}

	run(t, nil, "resolve", cut.ID, "--resolution", "Updated the URL.", "--format", "json")
	cuts = run(t, nil, "cuts", "--format", "json")
	if err := json.Unmarshal([]byte(cuts), &open); err != nil {
		t.Fatal(err)
	}
	if len(open) != 0 {
		t.Fatalf("got %d open cuts after resolution, want 0", len(open))
	}

	history := run(t, nil, "history", "--format", "json")
	var works []outputRecord
	if err := json.Unmarshal([]byte(history), &works); err != nil {
		t.Fatal(err)
	}
	if len(works) != 1 {
		t.Fatalf("got %d work records, want 1", len(works))
	}
}

func TestRequiredFieldFailureDoesNotWrite(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("PAPER_VAULT", vault)
	var stdout, stderr bytes.Buffer
	err := Run(t.Context(), []string{"cut", "--summary", "Missing description"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "--description is required") {
		t.Fatalf("expected required description error, got %v", err)
	}
	entries, readErr := os.ReadDir(vault)
	if readErr == nil && len(entries) != 0 {
		t.Fatalf("vault was modified on invalid input: %v", entries)
	}
}

func TestHistoryUntilDoesNotDefaultSinceToToday(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("PAPER_VAULT", vault)
	storage, err := store.Open(vault)
	if err != nil {
		t.Fatal(err)
	}
	createdAt := time.Now().AddDate(0, 0, -2)
	id, err := record.NewID(createdAt, "Earlier work")
	if err != nil {
		t.Fatal(err)
	}
	work, err := record.NewWork(record.Common{
		ID: id, CreatedAt: createdAt, Summary: "Earlier work",
		Project: record.Project{Name: "paper", Key: "paper", Root: "/tmp/paper"},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Write(work); err != nil {
		t.Fatal(err)
	}
	until := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	output := run(t, nil, "history", "--until", until, "--format", "json")
	var works []outputRecord
	if err := json.Unmarshal([]byte(output), &works); err != nil {
		t.Fatal(err)
	}
	if len(works) != 1 {
		t.Fatalf("got %d records, want 1", len(works))
	}
}

func TestRecordFlagsCanSurroundID(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("PAPER_VAULT", vault)
	created := run(t, nil, "log", "--summary", "Flag ordering", "--format", "json")
	var work outputRecord
	if err := json.Unmarshal([]byte(created), &work); err != nil {
		t.Fatal(err)
	}
	shown := run(t, nil, "show", "--vault", vault, work.ID, "--format", "json")
	if !strings.Contains(shown, work.ID) {
		t.Fatalf("show output does not contain ID: %s", shown)
	}
}

func TestInitDoesNotPersistUnusableVault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("PAPER_VAULT", "")
	working := t.TempDir()
	if _, err := config.Write(working); err != nil {
		t.Fatal(err)
	}
	unusable := home + "/not-a-directory"
	if err := os.WriteFile(unusable, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err := Run(t.Context(), []string{"init", "--vault", unusable}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("expected unusable vault error")
	}
	resolved, err := config.Resolve("")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != working {
		t.Fatalf("config changed to %q, want %q", resolved, working)
	}
}

func TestAgentWithoutActionPrintsDetailedHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(t.Context(), []string{"agent"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing action error")
	}
	for _, expected := range []string{"Manage Paper's user-level agent skills.", "kimi", "pi", "paper agent install TARGET"} {
		if !strings.Contains(stderr.String(), expected) {
			t.Fatalf("agent help does not contain %q:\n%s", expected, stderr.String())
		}
	}
}

func TestAgentHelpIsSuccessful(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Run(t.Context(), []string{"agent", "--help"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Every supported agent") {
		t.Fatalf("unexpected help output:\n%s", stdout.String())
	}
}

func TestVersion(t *testing.T) {
	previousVersion, previousCommit, previousBuildDate := Version, Commit, BuildDate
	Version, Commit, BuildDate = "1.2.3", "abc123", "2026-08-04T18:00:00Z"
	t.Cleanup(func() { Version, Commit, BuildDate = previousVersion, previousCommit, previousBuildDate })

	output := run(t, nil, "--version")
	for _, expected := range []string{"paper 1.2.3", "commit: abc123", "built: 2026-08-04T18:00:00Z"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("version output does not contain %q:\n%s", expected, output)
		}
	}
}

func run(t *testing.T, stdin *strings.Reader, args ...string) string {
	t.Helper()
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	var stdout, stderr bytes.Buffer
	if err := Run(t.Context(), args, stdin, &stdout, &stderr); err != nil {
		t.Fatalf("Run(%v): %v; stderr: %s", args, err, stderr.String())
	}
	return stdout.String()
}
