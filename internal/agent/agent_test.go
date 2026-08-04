package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallIsIdempotentAndUninstallIsOwned(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	first, err := Install(TargetClaude)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Install(TargetClaude)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Installed || !second.Current || first.Path != second.Path {
		t.Fatalf("unexpected statuses: %#v %#v", first, second)
	}
	if _, err := Uninstall(TargetClaude); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(first.Path); !os.IsNotExist(err) {
		t.Fatalf("skill still exists: %v", err)
	}
}

func TestInstallRefusesUnmanagedSkill(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	path := filepath.Join(home, ".config", "opencode", "skills", "paper", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(TargetOpenCode); err == nil {
		t.Fatal("expected unmanaged skill error")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "mine" {
		t.Fatalf("unmanaged skill was changed: %q", content)
	}
}

func TestInstallRecoversInterruptedManifestWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	path := filepath.Join(home, ".claude", "skills", "paper", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(skill), 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := Install(TargetClaude)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Installed || !status.Current {
		t.Fatalf("unexpected recovered status: %#v", status)
	}
}

func TestKimiAndPiPathsRespectOverrides(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	kimiHome := filepath.Join(home, "custom-kimi")
	piHome := filepath.Join(home, "custom-pi")
	t.Setenv("KIMI_CODE_HOME", kimiHome)
	t.Setenv("PI_CODING_AGENT_DIR", piHome)

	kimiPath, err := skillPath(TargetKimi)
	if err != nil {
		t.Fatal(err)
	}
	piPath, err := skillPath(TargetPi)
	if err != nil {
		t.Fatal(err)
	}
	if kimiPath != filepath.Join(kimiHome, "skills", "paper", "SKILL.md") {
		t.Fatalf("unexpected Kimi path: %s", kimiPath)
	}
	if piPath != filepath.Join(piHome, "skills", "paper", "SKILL.md") {
		t.Fatalf("unexpected Pi path: %s", piPath)
	}
}

func TestTargetSkillsUseCorrectLoggingIdentity(t *testing.T) {
	if !strings.Contains(skillFor(TargetKimi), "Use `kimi-code` as AGENT in Kimi Code.") {
		t.Fatal("Kimi skill does not contain its logging identity")
	}
	if !strings.Contains(skillFor(TargetPi), "Use `pi` as AGENT in Pi.") {
		t.Fatal("Pi skill does not contain its logging identity")
	}
	if skillFor(TargetClaude) != skill || skillFor(TargetOpenCode) != skill {
		t.Fatal("existing Claude Code or OpenCode skill content changed")
	}
}

func TestAllTargetsIncludesEveryAgent(t *testing.T) {
	targets, err := ParseTarget("all")
	if err != nil {
		t.Fatal(err)
	}
	expected := []Target{TargetClaude, TargetOpenCode, TargetKimi, TargetPi}
	if len(targets) != len(expected) {
		t.Fatalf("got %d targets, want %d", len(targets), len(expected))
	}
	for index := range expected {
		if targets[index] != expected[index] {
			t.Fatalf("target %d = %q, want %q", index, targets[index], expected[index])
		}
	}
}
