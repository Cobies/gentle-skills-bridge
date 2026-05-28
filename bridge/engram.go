package bridge

import (
	"bytes"
	"fmt"
	"os/exec"
)

type engramRunner interface {
	LookPath(file string) (string, error)
	Run(name string, args ...string) (string, error)
}

type execEngramRunner struct{}

func (execEngramRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (execEngramRunner) Run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return stderr.String(), err
	}

	return stderr.String(), nil
}

// SyncToEngram registers a skill in Engram database using the engram CLI.
func SyncToEngram(name string, title string, content string, project string) error {
	return syncToEngram(execEngramRunner{}, name, title, content, project)
}

func syncToEngram(runner engramRunner, name string, title string, content string, project string) error {
	// Check if engram command is available on PATH
	_, err := runner.LookPath("engram")
	if err != nil {
		return fmt.Errorf("engram CLI not found on PATH: %w", err)
	}

	// We use 'engram save' to register the skill in Engram.
	// Since Engram is an append-only log, it's best to save it under the project context.
	stderr, err := runner.Run("engram", engramSaveArgs(title, content, project)...)
	if err != nil {
		return fmt.Errorf("failed to run engram save: %w, stderr: %s", err, stderr)
	}

	return nil
}

func engramSaveArgs(title string, content string, project string) []string {
	return []string{
		"save",
		fmt.Sprintf("Skill: %s", title),
		content,
		"--type", "config",
		"--project", project,
		"--scope", "personal",
	}
}
