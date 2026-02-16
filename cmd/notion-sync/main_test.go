package main

import (
	"os/exec"
	"reflect"
	"testing"
)

func TestReorderArgs_FlagsBeforePositional(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "already ordered",
			args: []string{"--force", "folder"},
			want: []string{"--force", "folder"},
		},
		{
			name: "positional before flag",
			args: []string{"folder", "--force"},
			want: []string{"--force", "folder"},
		},
		{
			name: "flag with value after positional",
			args: []string{"folder", "--api-key", "mykey"},
			want: []string{"--api-key", "mykey", "folder"},
		},
		{
			name: "boolean flag -f",
			args: []string{"folder", "-f"},
			want: []string{"-f", "folder"},
		},
		{
			name: "flag with equals",
			args: []string{"folder", "--output=./out"},
			want: []string{"--output=./out", "folder"},
		},
		{
			name: "double dash stops",
			args: []string{"--force", "--", "folder", "--not-a-flag"},
			want: []string{"--force", "--", "folder", "--not-a-flag"},
		},
		{
			name: "mixed complex",
			args: []string{"mydb", "--output", "./out", "-f", "--api-key", "key123"},
			want: []string{"--output", "./out", "-f", "--api-key", "key123", "mydb"},
		},
		{
			name: "empty",
			args: []string{},
			want: nil,
		},
		{
			name: "only positional",
			args: []string{"folder"},
			want: []string{"folder"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reorderArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("reorderArgs(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestCLI_NoArgs_ExitZero(t *testing.T) {
	cmd := exec.Command("go", "run", ".", )
	err := cmd.Run()
	if err != nil {
		t.Errorf("expected exit 0 with no args, got %v", err)
	}
}

func TestCLI_UnknownCommand_ExitOne(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "nonexistent-command")
	err := cmd.Run()
	if err == nil {
		t.Error("expected non-zero exit for unknown command")
	}
}

func TestCLI_Version(t *testing.T) {
	out, err := exec.Command("go", "run", ".", "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if len(out) == 0 {
		t.Error("expected version output")
	}
}
