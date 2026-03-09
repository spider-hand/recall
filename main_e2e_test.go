//go:build e2e

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binaryPath string

func TestMain(m *testing.M) {
	tmpBinary := filepath.Join(os.TempDir(), "recall-test")
	cmd := exec.Command("go", "build", "-o", tmpBinary, ".")
	if err := cmd.Run(); err != nil {
		panic("failed to build binary: " + err.Error())
	}
	binaryPath = tmpBinary

	code := m.Run()

	os.Remove(tmpBinary)
	os.Exit(code)
}

func setupTestDir(t *testing.T, initialEntries []Entry) string {
	t.Helper()
	dir := t.TempDir()

	if initialEntries != nil {
		data, err := json.MarshalIndent(initialEntries, "", "  ")
		if err != nil {
			t.Fatalf("failed to marshal entries: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "entries.json"), data, 0644); err != nil {
			t.Fatalf("failed to write entries.json: %v", err)
		}
	}

	return dir
}

func runCommand(t *testing.T, dataDir string, stdin string, args ...string) string {
	t.Helper()

	cmd := exec.Command(binaryPath, args...)
	cmd.Env = append(os.Environ(), "DATA_DIR="+dataDir)

	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	output, _ := cmd.CombinedOutput()
	return string(output)
}

func assertContains(t *testing.T, output, want string) {
	t.Helper()
	if !strings.Contains(output, want) {
		t.Errorf("expected output to contain %q, got: %s", want, output)
	}
}

func assertNotContains(t *testing.T, output, notWant string) {
	t.Helper()
	if strings.Contains(output, notWant) {
		t.Errorf("expected output NOT to contain %q, got: %s", notWant, output)
	}
}

func assertEntryCount(t *testing.T, entries []Entry, want int) {
	t.Helper()
	if len(entries) != want {
		t.Errorf("expected %d entries, got %d", want, len(entries))
	}
}

func readEntries(t *testing.T, dataDir string) []Entry {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dataDir, "entries.json"))
	if err != nil {
		return []Entry{}
	}

	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("failed to unmarshal entries: %v", err)
	}
	return entries
}

func TestHelp(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		contains []string
	}{
		{
			name:     "with --help flag",
			args:     []string{"--help"},
			contains: []string{"Usage:", "--add", "--edit", "--delete", "--list"},
		},
		{
			name:     "with -h short flag",
			args:     []string{"-h"},
			contains: []string{"Usage:", "-a, --add", "-e, --edit", "-d, --delete", "-l, --list"},
		},
		{
			name:     "with no args",
			args:     []string{},
			contains: []string{"Usage:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := setupTestDir(t, nil)
			output := runCommand(t, dataDir, "", tt.args...)

			for _, want := range tt.contains {
				assertContains(t, output, want)
			}
		})
	}
}

func TestAdd(t *testing.T) {
	tests := []struct {
		name           string
		stdin          string
		wantEntryCount int
		wantContains   string
	}{
		{
			name:           "commands provided",
			stdin:          "deploy to production\ngit push origin main\n\n",
			wantEntryCount: 1,
			wantContains:   "",
		},
		{
			name:           "no commands",
			stdin:          "deploy\n\n",
			wantEntryCount: 0,
			wantContains:   "no commands added",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := setupTestDir(t, []Entry{})
			output := runCommand(t, dataDir, tt.stdin, "--add")

			entries := readEntries(t, dataDir)
			assertEntryCount(t, entries, tt.wantEntryCount)

			if tt.wantContains != "" {
				assertContains(t, output, tt.wantContains)
			}
		})
	}

	t.Run("with -a short flag", func(t *testing.T) {
		dataDir := setupTestDir(t, []Entry{})
		runCommand(t, dataDir, "test entry\ntest cmd\n\n", "-a")

		entries := readEntries(t, dataDir)
		assertEntryCount(t, entries, 1)
	})
}

func TestList(t *testing.T) {
	tests := []struct {
		name           string
		initialEntries []Entry
		wantContains   []string
	}{
		{
			name: "has entries",
			initialEntries: []Entry{
				{Description: "deploy", Commands: []string{"git push"}},
			},
			wantContains: []string{"deploy", "$ git push"},
		},
		{
			name:           "empty",
			initialEntries: []Entry{},
			wantContains:   []string{"no entries"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := setupTestDir(t, tt.initialEntries)
			output := runCommand(t, dataDir, "", "--list")

			for _, want := range tt.wantContains {
				assertContains(t, output, want)
			}
		})
	}

	t.Run("with -l short flag", func(t *testing.T) {
		dataDir := setupTestDir(t, []Entry{
			{Description: "test entry", Commands: []string{"test cmd"}},
		})
		output := runCommand(t, dataDir, "", "-l")

		assertContains(t, output, "test entry")
		assertContains(t, output, "$ test cmd")
	})
}

func TestQuery(t *testing.T) {
	tests := []struct {
		name           string
		initialEntries []Entry
		query          []string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "no results",
			initialEntries: []Entry{
				{Description: "deploy", Commands: []string{"git push"}},
			},
			query:        []string{"nonexistent"},
			wantContains: []string{"no results"},
		},
		{
			name: "single result",
			initialEntries: []Entry{
				{Description: "deploy to production", Commands: []string{"git push", "kubectl apply"}},
				{Description: "build docker image", Commands: []string{"docker build"}},
			},
			query:          []string{"deploy"},
			wantContains:   []string{"deploy to production", "$ git push", "$ kubectl apply"},
			wantNotContain: []string{"1)"},
		},
		{
			name: "multiple results",
			initialEntries: []Entry{
				{Description: "git push to main", Commands: []string{"git push origin main"}},
				{Description: "git pull from main", Commands: []string{"git pull origin main"}},
				{Description: "docker build", Commands: []string{"docker build ."}},
			},
			query:        []string{"git"},
			wantContains: []string{"1)", "2)", "$ git push", "$ git pull"},
		},
		{
			name: "max 3 results",
			initialEntries: []Entry{
				{Description: "git command one", Commands: []string{"cmd1"}},
				{Description: "git command two", Commands: []string{"cmd2"}},
				{Description: "git command three", Commands: []string{"cmd3"}},
				{Description: "git command four", Commands: []string{"cmd4"}},
				{Description: "git command five", Commands: []string{"cmd5"}},
			},
			query:          []string{"git"},
			wantContains:   []string{"1)", "2)", "3)"},
			wantNotContain: []string{"4)", "5)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := setupTestDir(t, tt.initialEntries)
			output := runCommand(t, dataDir, "", tt.query...)

			for _, want := range tt.wantContains {
				assertContains(t, output, want)
			}

			for _, notWant := range tt.wantNotContain {
				assertNotContains(t, output, notWant)
			}
		})
	}
}

func TestEdit(t *testing.T) {
	tests := []struct {
		name            string
		initialEntries  []Entry
		args            []string
		stdin           string
		wantContains    string
		wantDescription string
		wantCommands    []string
	}{
		{
			name:         "missing query",
			args:         []string{"--edit"},
			stdin:        "",
			wantContains: "usage: recall -e, --edit <query>",
		},
		{
			name: "no results",
			initialEntries: []Entry{
				{Description: "deploy", Commands: []string{"cmd"}},
			},
			args:         []string{"--edit", "nonexistent"},
			stdin:        "",
			wantContains: "no results",
		},
		{
			name: "invalid selection",
			initialEntries: []Entry{
				{Description: "deploy", Commands: []string{"cmd"}},
			},
			args:         []string{"--edit", "deploy"},
			stdin:        "99\n",
			wantContains: "invalid selection",
		},
		{
			name: "keep description, update commands",
			initialEntries: []Entry{
				{Description: "original desc", Commands: []string{"old cmd"}},
			},
			args:            []string{"--edit", "original"},
			stdin:           "1\n\nnew cmd1\nnew cmd2\n\n",
			wantContains:    "updated",
			wantDescription: "original desc",
			wantCommands:    []string{"new cmd1", "new cmd2"},
		},
		{
			name: "update description, keep commands",
			initialEntries: []Entry{
				{Description: "original desc", Commands: []string{"original cmd"}},
			},
			args:            []string{"--edit", "original"},
			stdin:           "1\nnew description\n\n",
			wantContains:    "updated",
			wantDescription: "new description",
			wantCommands:    []string{"original cmd"},
		},
		{
			name: "update both",
			initialEntries: []Entry{
				{Description: "original desc", Commands: []string{"original cmd"}},
			},
			args:            []string{"--edit", "original"},
			stdin:           "1\nnew description\nnew cmd\n\n",
			wantContains:    "updated",
			wantDescription: "new description",
			wantCommands:    []string{"new cmd"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := setupTestDir(t, tt.initialEntries)
			output := runCommand(t, dataDir, tt.stdin, tt.args...)

			if tt.wantContains != "" {
				assertContains(t, output, tt.wantContains)
			}

			if tt.wantDescription != "" || tt.wantCommands != nil {
				entries := readEntries(t, dataDir)
				if len(entries) == 0 {
					t.Fatal("expected entries to exist")
				}

				if tt.wantDescription != "" && entries[0].Description != tt.wantDescription {
					t.Errorf("expected description %q, got %q", tt.wantDescription, entries[0].Description)
				}

				if tt.wantCommands != nil {
					if len(entries[0].Commands) != len(tt.wantCommands) {
						t.Errorf("expected %d commands, got %d", len(tt.wantCommands), len(entries[0].Commands))
					}
					for i, want := range tt.wantCommands {
						if i < len(entries[0].Commands) && entries[0].Commands[i] != want {
							t.Errorf("expected command[%d] %q, got %q", i, want, entries[0].Commands[i])
						}
					}
				}
			}
		})
	}

	t.Run("with -e short flag", func(t *testing.T) {
		dataDir := setupTestDir(t, []Entry{
			{Description: "test entry", Commands: []string{"old cmd"}},
		})
		runCommand(t, dataDir, "1\nnew desc\nnew cmd\n\n", "-e", "test")

		entries := readEntries(t, dataDir)
		if entries[0].Description != "new desc" {
			t.Errorf("expected description 'new desc', got %q", entries[0].Description)
		}
	})
}

func TestDelete(t *testing.T) {
	tests := []struct {
		name           string
		initialEntries []Entry
		args           []string
		stdin          string
		wantContains   string
		wantRemaining  int
	}{
		{
			name:         "missing query",
			args:         []string{"--delete"},
			stdin:        "",
			wantContains: "usage: recall -d, --delete <query>",
		},
		{
			name: "no results",
			initialEntries: []Entry{
				{Description: "deploy", Commands: []string{"cmd"}},
			},
			args:          []string{"--delete", "nonexistent"},
			stdin:         "",
			wantContains:  "no results",
			wantRemaining: 1,
		},
		{
			name: "invalid selection",
			initialEntries: []Entry{
				{Description: "deploy", Commands: []string{"cmd"}},
			},
			args:          []string{"--delete", "deploy"},
			stdin:         "99\n",
			wantContains:  "invalid selection",
			wantRemaining: 1,
		},
		{
			name: "success",
			initialEntries: []Entry{
				{Description: "to delete", Commands: []string{"cmd1"}},
				{Description: "to keep", Commands: []string{"cmd2"}},
			},
			args:          []string{"--delete", "delete"},
			stdin:         "1\n",
			wantContains:  "deleted",
			wantRemaining: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := setupTestDir(t, tt.initialEntries)
			output := runCommand(t, dataDir, tt.stdin, tt.args...)

			if tt.wantContains != "" {
				assertContains(t, output, tt.wantContains)
			}

			if tt.wantRemaining > 0 {
				entries := readEntries(t, dataDir)
				assertEntryCount(t, entries, tt.wantRemaining)
			}
		})
	}

	t.Run("with -d short flag", func(t *testing.T) {
		dataDir := setupTestDir(t, []Entry{
			{Description: "to delete", Commands: []string{"cmd"}},
		})
		output := runCommand(t, dataDir, "1\n", "-d", "delete")

		assertContains(t, output, "deleted")
		entries := readEntries(t, dataDir)
		assertEntryCount(t, entries, 0)
	})
}
