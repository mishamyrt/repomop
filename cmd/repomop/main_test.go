package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"repomop/internal/scanner"
	"repomop/internal/testutil"
)

func TestRunDryRunDoesNotDelete(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	testutil.MkdirAll(t, filepath.Join(project, "node_modules"))
	testutil.WriteFile(t, filepath.Join(project, "package.json"), "{}")
	testutil.WriteFile(t, filepath.Join(project, "node_modules", "a.js"), "console.log('x')")

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
	testutil.MkdirAll(t, filepath.Join(jsProject, "node_modules"))
	testutil.WriteFile(t, filepath.Join(jsProject, "package.json"), "{}")
	testutil.WriteFile(t, filepath.Join(jsProject, "node_modules", "a.js"), "console.log('x')")

	pyProject := filepath.Join(root, "py")
	venv := filepath.Join(pyProject, "my_env")
	testutil.MkdirAll(t, venv)
	testutil.WriteFile(t, filepath.Join(venv, "pyvenv.cfg"), "home=/usr/bin")

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

func TestPrintDryRunUsesSizePathTypeFormat(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	artifacts := []scanner.Artifact{
		{
			Kind:        scanner.ArtifactNodeModule,
			Path:        filepath.Join(root, "workspace", "node_modules"),
			ProjectRoot: filepath.Join(root, "workspace"),
			SizeBytes:   2048,
		},
	}

	stdout := &bytes.Buffer{}
	printDryRun(stdout, root, artifacts, nil)

	output := stdout.String()
	if strings.Contains(output, "(project:") {
		t.Fatalf("project column must not be present in dry-run output:\n%s", output)
	}

	pattern := regexp.MustCompile(`(?m)^-\s+2\.0 KiB\s+workspace/node_modules\s+node-modules$`)
	if !pattern.MatchString(output) {
		t.Fatalf("artifact line must be in 'size path type' order:\n%s", output)
	}
}

