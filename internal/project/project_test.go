package project

import "testing"

func TestRemoteKey(t *testing.T) {
	tests := map[string]string{
		"git@github.com:gabe/paper.git":       "github.com/gabe/paper",
		"https://github.com/gabe/paper.git":   "github.com/gabe/paper",
		"ssh://git@gitlab.com/gabe/paper.git": "gitlab.com/gabe/paper",
	}
	for remote, expected := range tests {
		if actual := remoteKey(remote); actual != expected {
			t.Errorf("remoteKey(%q) = %q, want %q", remote, actual, expected)
		}
	}
}
