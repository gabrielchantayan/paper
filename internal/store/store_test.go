package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gabrielchantayan/paper/internal/record"
)

func TestWriteGetAndResolve(t *testing.T) {
	storage, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cut := testCut(t, "20260804T143012-broken-link-a7c2", "Broken link")
	written, err := storage.Write(cut)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(written.Path) != ".md" {
		t.Fatalf("unexpected path %s", written.Path)
	}
	found, err := storage.Get("20260804T143012-broken")
	if err != nil {
		t.Fatal(err)
	}
	if found.Record.CommonFields().ID != cut.ID {
		t.Fatalf("got %s, want %s", found.Record.CommonFields().ID, cut.ID)
	}
	resolved, err := storage.Resolve("20260804T143012", "Fixed documentation.", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := resolved.Record.(record.ResolvedCut); !ok {
		t.Fatalf("got %T, want ResolvedCut", resolved.Record)
	}
	open, err := storage.Cuts(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 0 {
		t.Fatalf("got %d open cuts, want 0", len(open))
	}
}

func TestConcurrentWrites(t *testing.T) {
	storage, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	const count = 20
	var group sync.WaitGroup
	errors := make(chan error, count)
	for index := range count {
		group.Add(1)
		go func() {
			defer group.Done()
			id := fmt.Sprintf("20260804T1430%02d-work-item-%04x", index, index)
			common := testCommon(record.ID(id), fmt.Sprintf("Work %d", index))
			work, err := record.NewWork(common, "")
			if err == nil {
				_, err = storage.Write(work)
			}
			errors <- err
		}()
	}
	group.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	items, err := storage.Work()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != count {
		t.Fatalf("got %d records, want %d", len(items), count)
	}
}

func TestConcurrentResolutionHasOneWinner(t *testing.T) {
	storage, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cut := testCut(t, "20260804T143012-broken-link-a7c2", "Broken link")
	if _, err := storage.Write(cut); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	for _, resolution := range []string{"First fix", "Second fix"} {
		go func() {
			_, err := storage.Resolve(string(cut.ID), resolution, cut.CreatedAt.Add(time.Hour))
			results <- err
		}()
	}
	var successes int
	for range 2 {
		if err := <-results; err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("got %d successful resolutions, want 1", successes)
	}
}

func TestMalformedRecordFailsWholeScan(t *testing.T) {
	storage, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(storage.Root(), "cuts", "bad.md")
	if err := os.WriteFile(path, []byte("not frontmatter"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = storage.Cuts(false)
	if err == nil || !strings.Contains(err.Error(), "must begin with YAML frontmatter") {
		t.Fatalf("expected parse failure, got %v", err)
	}
}

func TestGetIgnoresMarkdownOutsideRecordTrees(t *testing.T) {
	storage, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storage.Root(), "README.md"), []byte("# Vault"), 0o644); err != nil {
		t.Fatal(err)
	}
	cut := testCut(t, "20260804T143012-broken-link-a7c2", "Broken link")
	if _, err := storage.Write(cut); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Get(string(cut.ID)); err != nil {
		t.Fatal(err)
	}
}

func testCut(t *testing.T, id, summary string) record.OpenCut {
	t.Helper()
	cut, err := record.NewOpenCut(testCommon(record.ID(id), summary), record.CutFields{Kind: record.CutKindBrokenLink, Description: "It is gone."})
	if err != nil {
		t.Fatal(err)
	}
	return cut
}

func testCommon(id record.ID, summary string) record.Common {
	createdAt, err := time.Parse("20060102T150405", string(id)[:15])
	if err != nil {
		panic(err)
	}
	return record.Common{
		ID: id, CreatedAt: createdAt, Summary: summary,
		Project: record.Project{Name: "paper", Key: "paper", Root: "/tmp/paper"},
	}
}
