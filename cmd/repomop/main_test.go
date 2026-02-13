package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDryRunDoesNotDelete(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	mustMkdirAll(t, filepath.Join(project, "node_modules"))
	mustWriteFile(t, filepath.Join(project, "package.json"), "{}")
	mustWriteFile(t, filepath.Join(project, "node_modules", "a.js"), "console.log('x')")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run([]string{"--path", root, "--dry-run"}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "repomop dry-run") {
		t.Fatalf("expected dry-run output, got: %s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(project, "node_modules")); err != nil {
		t.Fatalf("expected node_modules to remain, stat error: %v", err)
	}
}

func TestRunYesDeletesAllArtifacts(t *testing.T) {
	root := t.TempDir()

	jsProject := filepath.Join(root, "js")
	mustMkdirAll(t, filepath.Join(jsProject, "node_modules"))
	mustWriteFile(t, filepath.Join(jsProject, "package.json"), "{}")
	mustWriteFile(t, filepath.Join(jsProject, "node_modules", "a.js"), "console.log('x')")

	pyProject := filepath.Join(root, "py")
	venv := filepath.Join(pyProject, "my_env")
	mustMkdirAll(t, venv)
	mustWriteFile(t, filepath.Join(venv, "pyvenv.cfg"), "home=/usr/bin")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run([]string{"--path", root, "--yes"}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Deleted: 2") {
		t.Fatalf("expected delete summary in output, got: %s", stdout.String())
	}

	if _, err := os.Stat(filepath.Join(jsProject, "node_modules")); !os.IsNotExist(err) {
		t.Fatalf("expected node_modules deleted, stat err=%v", err)
	}
	if _, err := os.Stat(venv); !os.IsNotExist(err) {
		t.Fatalf("expected venv deleted, stat err=%v", err)
	}
}

func TestRunRejectsInvalidFlags(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run([]string{"--dry-run", "--yes"}, stdout, stderr)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "cannot be used together") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
