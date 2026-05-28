package bridge

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResolveAgentSkillsDirs(t *testing.T) {
	home := filepath.Clean("/home/tester")

	tests := []struct {
		name   string
		agents []string
		want   []string
	}{
		{
			name:   "known agents",
			agents: []string{"claude-code", "opencode"},
			want: []string{
				filepath.Join(home, ".claude", "skills"),
				filepath.Join(home, ".config", "opencode", "skills"),
			},
		},
		{
			name:   "deduplicates shared paths",
			agents: []string{"antigravity", "pi"},
			want: []string{
				filepath.Join(home, ".gemini", "config", "skills"),
				filepath.Join(home, ".gemini", "skills"),
				filepath.Join(home, ".gemini", "antigravity", "skills"),
				filepath.Join(home, ".gemini", "antigravity-cli", "skills"),
				filepath.Join(home, ".pi", "skills"),
			},
		},
		{
			name:   "ignores unknown agents",
			agents: []string{"unknown", "opencode"},
			want: []string{
				filepath.Join(home, ".config", "opencode", "skills"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveAgentSkillsDirs(home, tt.agents)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ResolveAgentSkillsDirs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestDiscoverTargetsFromHome(t *testing.T) {
	home := t.TempDir()
	stateDir := filepath.Join(home, ".gentle-ai")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writeTestFile(t, filepath.Join(stateDir, "state.json"), `{"installed_agents":["opencode","claude-code"]}`)

	got, err := discoverTargets(home)
	if err != nil {
		t.Fatalf("discoverTargets() error = %v", err)
	}

	want := []string{
		filepath.Join(home, ".config", "opencode", "skills"),
		filepath.Join(home, ".claude", "skills"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discoverTargets() = %#v, want %#v", got, want)
	}
}

func TestDiscoverTargetsMissingStateReturnsEmpty(t *testing.T) {
	got, err := discoverTargets(t.TempDir())
	if err != nil {
		t.Fatalf("discoverTargets() error = %v", err)
	}
	if got != nil {
		t.Fatalf("discoverTargets() = %#v, want nil", got)
	}
}

func TestDiscoverTargetsInvalidJSON(t *testing.T) {
	home := t.TempDir()
	stateDir := filepath.Join(home, ".gentle-ai")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writeTestFile(t, filepath.Join(stateDir, "state.json"), `{invalid`)

	_, err := discoverTargets(home)
	if err == nil {
		t.Fatal("discoverTargets() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "failed to decode gentle-ai state") {
		t.Fatalf("discoverTargets() error = %q, want decode error", err.Error())
	}

	var syntaxErr *os.PathError
	if errors.As(err, &syntaxErr) {
		t.Fatalf("discoverTargets() error unexpectedly wraps PathError: %v", err)
	}
}
