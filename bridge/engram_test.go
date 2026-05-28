package bridge

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakeEngramRunner struct {
	lookPathErr error
	runErr      error
	stderr      string
	command     string
	args        []string
}

func (runner *fakeEngramRunner) LookPath(file string) (string, error) {
	if runner.lookPathErr != nil {
		return "", runner.lookPathErr
	}
	return file, nil
}

func (runner *fakeEngramRunner) Run(name string, args ...string) (string, error) {
	runner.command = name
	runner.args = append([]string(nil), args...)
	return runner.stderr, runner.runErr
}

func TestEngramSaveArgs(t *testing.T) {
	got := engramSaveArgs("React", "content", "global")
	want := []string{
		"save",
		"Skill: React",
		"content",
		"--type", "config",
		"--project", "global",
		"--scope", "personal",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("engramSaveArgs() = %#v, want %#v", got, want)
	}
}

func TestSyncToEngramUsesRunner(t *testing.T) {
	runner := &fakeEngramRunner{}

	if err := syncToEngram(runner, "react", "React", "content", "global"); err != nil {
		t.Fatalf("syncToEngram() error = %v", err)
	}

	if runner.command != "engram" {
		t.Fatalf("command = %q, want engram", runner.command)
	}
	if !reflect.DeepEqual(runner.args, engramSaveArgs("React", "content", "global")) {
		t.Fatalf("args = %#v, want %#v", runner.args, engramSaveArgs("React", "content", "global"))
	}
}

func TestSyncToEngramMissingCLI(t *testing.T) {
	runner := &fakeEngramRunner{lookPathErr: errors.New("not found")}

	err := syncToEngram(runner, "react", "React", "content", "global")
	if err == nil {
		t.Fatal("syncToEngram() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "engram CLI not found on PATH") {
		t.Fatalf("syncToEngram() error = %q, want missing CLI error", err.Error())
	}
}

func TestSyncToEngramRunFailureIncludesStderr(t *testing.T) {
	runner := &fakeEngramRunner{runErr: errors.New("exit status 1"), stderr: "boom"}

	err := syncToEngram(runner, "react", "React", "content", "global")
	if err == nil {
		t.Fatal("syncToEngram() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "failed to run engram save") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("syncToEngram() error = %q, want command failure with stderr", err.Error())
	}
}
