package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofrs/flock"
)

type Target string

const (
	TargetClaude   Target = "claude"
	TargetOpenCode Target = "opencode"
	TargetKimi     Target = "kimi"
	TargetPi       Target = "pi"
)

const skill = `---
name: paper
description: Records completed work and actionable friction. Use during software work to log cuts immediately and meaningful outcomes at completion.
---

# Paper

Paper is the durable work and friction log. Use it without interrupting the task.

## Before Working

Run ` + "`paper cuts --format json`" + ` to learn whether relevant friction is already known.

## Record Friction Immediately

When a tool call dead-ends, a link is broken, documentation misleads, a command fails unexpectedly, or the environment or workflow creates avoidable friction:

1. Check ` + "`paper cuts --format json`" + ` for an existing matching cut.
2. If none exists, run ` + "`paper cut --summary \"...\" --description \"...\" --kind KIND --agent AGENT`" + `.
3. Include the attempted action, impact, workaround, evidence, or suggested fix when known.
4. Continue the original task. Logging a cut is not a substitute for finishing the work.

Do not log routine failures caused by your own typo, expected exploratory dead ends, or duplicates.

## Record Completed Work

After completing meaningful work, write one concise outcome with ` + "`paper log --summary \"...\" --agent AGENT`" + `. Use ` + "`--details`" + ` when the summary cannot explain the result and verification. Do not log every command or intermediate action.

Use ` + "`claude-code`" + ` as AGENT in Claude Code and ` + "`opencode`" + ` in OpenCode.
`

type manifest struct {
	Owner  string `json:"owner"`
	SHA256 string `json:"sha256"`
}

type Status struct {
	Target    Target `json:"target"`
	Path      string `json:"path"`
	Installed bool   `json:"installed"`
	Present   bool   `json:"present"`
	Modified  bool   `json:"modified"`
	Current   bool   `json:"current"`
}

func ParseTarget(value string) ([]Target, error) {
	switch Target(value) {
	case TargetClaude:
		return []Target{TargetClaude}, nil
	case TargetOpenCode:
		return []Target{TargetOpenCode}, nil
	case TargetKimi:
		return []Target{TargetKimi}, nil
	case TargetPi:
		return []Target{TargetPi}, nil
	case "all":
		return []Target{TargetClaude, TargetOpenCode, TargetKimi, TargetPi}, nil
	default:
		return nil, fmt.Errorf("invalid agent target %q; expected claude, opencode, kimi, pi, or all", value)
	}
}

func DisplayName(target Target) string {
	switch target {
	case TargetClaude:
		return "Claude Code"
	case TargetOpenCode:
		return "OpenCode"
	case TargetKimi:
		return "Kimi Code"
	case TargetPi:
		return "Pi"
	default:
		return string(target)
	}
}

func AgentName(target Target) string {
	switch target {
	case TargetClaude:
		return "claude-code"
	case TargetOpenCode:
		return "opencode"
	case TargetKimi:
		return "kimi-code"
	case TargetPi:
		return "pi"
	default:
		return string(target)
	}
}

func Install(target Target) (result Status, err error) {
	content := skillFor(target)
	path, err := skillPath(target)
	if err != nil {
		return Status{}, err
	}
	lock, err := lockSkillRoot(path)
	if err != nil {
		return Status{}, err
	}
	defer func() { err = errors.Join(err, lock.Unlock()) }()
	status, err := inspect(target, path)
	if err != nil {
		return Status{}, err
	}
	if status.Installed && !status.Modified && status.Current {
		return status, nil
	}
	if status.Installed && status.Present && !status.Current {
		return Status{}, fmt.Errorf("managed skill at %s is outdated; uninstall it before installing this version", path)
	}
	if status.Modified && !status.Current {
		return Status{}, fmt.Errorf("refusing to overwrite modified or unmanaged skill at %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Status{}, fmt.Errorf("create skill directory: %w", err)
	}
	if !status.Current {
		if status.Installed && status.Present {
			err = atomicWrite(path, []byte(content))
		} else {
			err = createAtomic(path, []byte(content))
		}
		if err != nil {
			return Status{}, err
		}
	}
	metadata, err := json.MarshalIndent(manifest{Owner: "paper", SHA256: digest([]byte(content))}, "", "  ")
	if err != nil {
		return Status{}, fmt.Errorf("encode skill manifest: %w", err)
	}
	metadata = append(metadata, '\n')
	if err := atomicWrite(filepath.Join(filepath.Dir(path), ".paper-managed.json"), metadata); err != nil {
		return Status{}, err
	}
	return Status{Target: target, Path: path, Installed: true, Present: true, Current: true}, nil
}

func Uninstall(target Target) (result Status, err error) {
	path, err := skillPath(target)
	if err != nil {
		return Status{}, err
	}
	if _, statErr := os.Stat(filepath.Dir(path)); errors.Is(statErr, os.ErrNotExist) {
		return Status{Target: target, Path: path}, nil
	}
	lock, err := lockSkillRoot(path)
	if err != nil {
		return Status{}, err
	}
	defer func() { err = errors.Join(err, lock.Unlock()) }()
	status, err := inspect(target, path)
	if err != nil {
		return Status{}, err
	}
	if !status.Installed {
		if status.Modified {
			return Status{}, fmt.Errorf("refusing to remove unmanaged skill at %s", path)
		}
		return status, nil
	}
	if status.Modified {
		return Status{}, fmt.Errorf("refusing to remove modified skill at %s", path)
	}
	manifestPath := filepath.Join(filepath.Dir(path), ".paper-managed.json")
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Status{}, fmt.Errorf("remove skill: %w", err)
	}
	if err := os.Remove(manifestPath); err != nil {
		return Status{}, fmt.Errorf("remove skill manifest: %w", err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		return Status{}, fmt.Errorf("inspect skill directory after uninstall: %w", err)
	}
	if len(entries) == 0 {
		if err := os.Remove(filepath.Dir(path)); err != nil {
			return Status{}, fmt.Errorf("remove empty skill directory: %w", err)
		}
	}
	return Status{Target: target, Path: path}, nil
}

func Doctor(target Target) (Status, error) {
	path, err := skillPath(target)
	if err != nil {
		return Status{}, err
	}
	return inspect(target, path)
}

func inspect(target Target, path string) (Status, error) {
	contentForTarget := skillFor(target)
	status := Status{Target: target, Path: path}
	content, contentErr := os.ReadFile(path)
	manifestPath := filepath.Join(filepath.Dir(path), ".paper-managed.json")
	metadata, manifestErr := os.ReadFile(manifestPath)
	if errors.Is(contentErr, os.ErrNotExist) && errors.Is(manifestErr, os.ErrNotExist) {
		return status, nil
	}
	if contentErr != nil && !errors.Is(contentErr, os.ErrNotExist) {
		return Status{}, fmt.Errorf("read skill: %w", contentErr)
	}
	if manifestErr != nil && !errors.Is(manifestErr, os.ErrNotExist) {
		return Status{}, fmt.Errorf("read skill manifest: %w", manifestErr)
	}
	if errors.Is(manifestErr, os.ErrNotExist) {
		if contentErr == nil && string(content) == contentForTarget {
			status.Present = true
			status.Current = true
			return status, nil
		}
		status.Modified = true
		return status, nil
	}
	var owned manifest
	if err := json.Unmarshal(metadata, &owned); err != nil || owned.Owner != "paper" {
		status.Modified = true
		return status, nil
	}
	status.Installed = true
	if errors.Is(contentErr, os.ErrNotExist) {
		return status, nil
	}
	status.Present = true
	status.Modified = owned.SHA256 != digest(content)
	status.Current = string(content) == contentForTarget
	return status, nil
}

func skillPath(target Target) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	var root string
	switch target {
	case TargetClaude:
		root = os.Getenv("CLAUDE_CONFIG_DIR")
		if root == "" {
			root = filepath.Join(home, ".claude")
		}
	case TargetOpenCode:
		root = os.Getenv("OPENCODE_CONFIG_DIR")
		if root == "" {
			root = os.Getenv("XDG_CONFIG_HOME")
			if root == "" {
				root = filepath.Join(home, ".config")
			}
			root = filepath.Join(root, "opencode")
		}
	case TargetKimi:
		root = os.Getenv("KIMI_CODE_HOME")
		if root == "" {
			root = filepath.Join(home, ".kimi-code")
		}
	case TargetPi:
		root = os.Getenv("PI_CODING_AGENT_DIR")
		if root == "" {
			root = filepath.Join(home, ".pi", "agent")
		}
	default:
		return "", fmt.Errorf("invalid agent target %q", target)
	}
	return filepath.Join(root, "skills", "paper", "SKILL.md"), nil
}

func skillFor(target Target) string {
	const identities = "Use `claude-code` as AGENT in Claude Code and `opencode` in OpenCode."
	switch target {
	case TargetKimi:
		return strings.Replace(skill, identities, "Use `kimi-code` as AGENT in Kimi Code.", 1)
	case TargetPi:
		return strings.Replace(skill, identities, "Use `pi` as AGENT in Pi.", 1)
	default:
		return skill
	}
}

func atomicWrite(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".paper-skill-*")
	if err != nil {
		return fmt.Errorf("create temporary skill file: %w", err)
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err := file.Chmod(0o644); err != nil {
		file.Close()
		return fmt.Errorf("set skill permissions: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return fmt.Errorf("write skill: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync skill: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close skill: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("replace skill: %w", err)
	}
	return nil
}

func createAtomic(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".paper-skill-*")
	if err != nil {
		return fmt.Errorf("create temporary skill file: %w", err)
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err := file.Chmod(0o644); err != nil {
		file.Close()
		return fmt.Errorf("set skill permissions: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return fmt.Errorf("write skill: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync skill: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close skill: %w", err)
	}
	if err := os.Link(temporary, path); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("refusing to overwrite skill created concurrently at %s", path)
		}
		return fmt.Errorf("create skill: %w", err)
	}
	return nil
}

func lockSkillRoot(path string) (*flock.Flock, error) {
	root := filepath.Dir(filepath.Dir(path))
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create skill root: %w", err)
	}
	lock := flock.New(filepath.Join(root, ".paper.lock"))
	if err := lock.Lock(); err != nil {
		return nil, fmt.Errorf("lock skill root: %w", err)
	}
	return lock, nil
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
