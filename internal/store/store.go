package store

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gabrielchantayan/paper/internal/record"
	"github.com/gofrs/flock"
)

type Store struct{ root string }

type Stored struct {
	Record record.Record
	Path   string
	Raw    []byte
}

func Open(root string) (Store, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return Store{}, fmt.Errorf("resolve vault: %w", err)
	}
	for _, path := range []string{filepath.Join(root, "work"), filepath.Join(root, "cuts")} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return Store{}, fmt.Errorf("create vault directory %s: %w", path, err)
		}
	}
	return Store{root: root}, nil
}

func (store Store) Root() string { return store.root }

func (store Store) Write(value record.Record) (Stored, error) {
	data, err := record.Encode(value)
	if err != nil {
		return Stored{}, err
	}
	common := value.CommonFields()
	var directory string
	if value.RecordType() == record.TypeWork {
		directory = filepath.Join(store.root, "work", common.CreatedAt.Format("2006"), common.CreatedAt.Format("01"))
	} else {
		directory = filepath.Join(store.root, "cuts")
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return Stored{}, fmt.Errorf("create record directory: %w", err)
	}
	path := filepath.Join(directory, string(common.ID)+".md")
	if err := createAtomic(path, data); err != nil {
		return Stored{}, err
	}
	return Stored{Record: value, Path: path, Raw: data}, nil
}

func (store Store) Work() ([]Stored, error) {
	items, err := store.scan(filepath.Join(store.root, "work"))
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.Record.RecordType() != record.TypeWork {
			return nil, fmt.Errorf("cut record stored in work tree: %s", item.Path)
		}
	}
	return items, nil
}

func (store Store) Cuts(includeResolved bool) ([]Stored, error) {
	items, err := store.scan(filepath.Join(store.root, "cuts"))
	if err != nil {
		return nil, err
	}
	result := items[:0]
	for _, item := range items {
		if item.Record.RecordType() != record.TypeCut {
			return nil, fmt.Errorf("work record stored in cuts tree: %s", item.Path)
		}
		if _, open := item.Record.(record.OpenCut); includeResolved || open {
			result = append(result, item)
		}
	}
	return result, nil
}

func (store Store) Get(prefix string) (Stored, error) {
	if strings.TrimSpace(prefix) == "" {
		return Stored{}, fmt.Errorf("record ID prefix must not be empty")
	}
	work, err := store.Work()
	if err != nil {
		return Stored{}, err
	}
	cuts, err := store.Cuts(true)
	if err != nil {
		return Stored{}, err
	}
	all := append(work, cuts...)
	var matches []Stored
	for _, item := range all {
		if strings.HasPrefix(string(item.Record.CommonFields().ID), prefix) {
			matches = append(matches, item)
		}
	}
	if len(matches) == 0 {
		return Stored{}, fmt.Errorf("no record matches %q", prefix)
	}
	if len(matches) > 1 {
		return Stored{}, fmt.Errorf("record prefix %q is ambiguous", prefix)
	}
	return matches[0], nil
}

func (store Store) Resolve(prefix, resolution string, resolvedAt time.Time) (result Stored, err error) {
	lock := flock.New(filepath.Join(store.root, "cuts", ".paper-resolve.lock"))
	if err := lock.Lock(); err != nil {
		return Stored{}, fmt.Errorf("lock cut store: %w", err)
	}
	defer func() { err = errors.Join(err, lock.Unlock()) }()

	stored, err := store.Get(prefix)
	if err != nil {
		return Stored{}, err
	}
	open, ok := stored.Record.(record.OpenCut)
	if !ok {
		if _, resolved := stored.Record.(record.ResolvedCut); resolved {
			return Stored{}, fmt.Errorf("cut %s is already resolved", stored.Record.CommonFields().ID)
		}
		return Stored{}, fmt.Errorf("record %s is not a cut", stored.Record.CommonFields().ID)
	}
	resolved, err := record.Resolve(open, resolvedAt, resolution)
	if err != nil {
		return Stored{}, err
	}
	data, err := record.Encode(resolved)
	if err != nil {
		return Stored{}, err
	}
	current, err := os.ReadFile(stored.Path)
	if err != nil {
		return Stored{}, fmt.Errorf("re-read cut before resolution: %w", err)
	}
	if !bytes.Equal(current, stored.Raw) {
		return Stored{}, fmt.Errorf("cut %s changed while it was being resolved", stored.Record.CommonFields().ID)
	}
	if err := replaceAtomic(stored.Path, data); err != nil {
		return Stored{}, err
	}
	return Stored{Record: resolved, Path: stored.Path, Raw: data}, nil
}

func (store Store) scan(root string) ([]Stored, error) {
	var result []Stored
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		value, err := record.Parse(data)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		result = append(result, Stored{Record: value, Path: path, Raw: data})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read records: %w", err)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Record.CommonFields().CreatedAt.After(result[j].Record.CommonFields().CreatedAt)
	})
	return result, nil
}

func createAtomic(path string, data []byte) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("record already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect record path: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".paper-record-*")
	if err != nil {
		return fmt.Errorf("create temporary record: %w", err)
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err := file.Write(data); err != nil {
		file.Close()
		return fmt.Errorf("write record: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync record: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close record: %w", err)
	}
	if err := os.Link(temporary, path); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("record already exists: %s", path)
		}
		return fmt.Errorf("create record: %w", err)
	}
	return nil
}

func replaceAtomic(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".paper-record-*")
	if err != nil {
		return fmt.Errorf("create temporary record: %w", err)
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err := file.Write(data); err != nil {
		file.Close()
		return fmt.Errorf("write record: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync record: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close record: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("replace record: %w", err)
	}
	return nil
}
