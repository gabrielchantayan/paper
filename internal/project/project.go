package project

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gabrielchantayan/paper/internal/record"
)

type Context struct {
	Project   record.Project
	Directory string
	Git       *record.Git
}

func Discover(ctx context.Context, directory, nameOverride string) (Context, error) {
	directory, err := filepath.Abs(directory)
	if err != nil {
		return Context{}, fmt.Errorf("resolve current directory: %w", err)
	}
	root, err := git(ctx, directory, "rev-parse", "--show-toplevel")
	if err != nil {
		if !isNotRepository(err) {
			return Context{}, fmt.Errorf("discover Git repository: %w", err)
		}
		name := filepath.Base(directory)
		if nameOverride != "" {
			name = nameOverride
		}
		return Context{Project: record.Project{Name: name, Key: directory, Root: directory}}, nil
	}

	remote, err := git(ctx, directory, "config", "--get", "remote.origin.url")
	if err != nil && !isExitCode(err, 1, "") {
		return Context{}, fmt.Errorf("read Git origin: %w", err)
	}
	name := filepath.Base(root)
	key := root
	if remote != "" {
		key = remoteKey(remote)
		if base := filepath.Base(key); base != "." && base != "/" {
			name = base
		}
	}
	if nameOverride != "" {
		name = nameOverride
	}
	relative, err := filepath.Rel(root, directory)
	if err != nil {
		return Context{}, fmt.Errorf("resolve project-relative directory: %w", err)
	}
	if relative == "." {
		relative = ""
	}
	branch, err := git(ctx, directory, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil && !isExitCode(err, 1, "") {
		return Context{}, fmt.Errorf("read Git branch: %w", err)
	}
	commit, err := git(ctx, directory, "rev-parse", "--verify", "HEAD")
	if err != nil && !isUnbornHead(err) {
		return Context{}, fmt.Errorf("read Git commit: %w", err)
	}
	var gitInfo *record.Git
	if branch != "" || commit != "" {
		gitInfo = &record.Git{Branch: branch, Commit: commit}
	}
	return Context{
		Project:   record.Project{Name: name, Key: key, Root: root, Remote: remote},
		Directory: relative, Git: gitInfo,
	}, nil
}

func git(ctx context.Context, directory string, arguments ...string) (string, error) {
	args := append([]string{"-C", directory}, arguments...)
	output, err := exec.CommandContext(ctx, "git", args...).CombinedOutput()
	if err != nil {
		return "", gitError{err: err, output: strings.TrimSpace(string(output))}
	}
	return strings.TrimSpace(string(output)), nil
}

type gitError struct {
	err    error
	output string
}

func (err gitError) Error() string {
	if err.output == "" {
		return err.err.Error()
	}
	return err.output
}

func (err gitError) Unwrap() error { return err.err }

func isNotRepository(err error) bool {
	var commandError gitError
	return errors.As(err, &commandError) && strings.Contains(strings.ToLower(commandError.output), "not a git repository")
}

func isExitCode(err error, code int, output string) bool {
	var commandError gitError
	if !errors.As(err, &commandError) || commandError.output != output {
		return false
	}
	var exitError *exec.ExitError
	return errors.As(commandError.err, &exitError) && exitError.ExitCode() == code
}

func isUnbornHead(err error) bool {
	var commandError gitError
	if !errors.As(err, &commandError) {
		return false
	}
	output := strings.ToLower(commandError.output)
	return strings.Contains(output, "needed a single revision") ||
		strings.Contains(output, "unknown revision or path not in the working tree") ||
		strings.Contains(output, "ambiguous argument 'head'")
}

func remoteKey(remote string) string {
	value := strings.TrimSuffix(strings.TrimSpace(remote), ".git")
	if index := strings.Index(value, "://"); index >= 0 {
		value = value[index+3:]
		if at := strings.Index(value, "@"); at >= 0 {
			value = value[at+1:]
		}
	} else if at := strings.Index(value, "@"); at >= 0 {
		value = value[at+1:]
		value = strings.Replace(value, ":", "/", 1)
	}
	return strings.Trim(value, "/")
}
