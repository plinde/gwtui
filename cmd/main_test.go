package main

import (
	"strings"
	"testing"
)

func TestPrintShellInit_Zsh(t *testing.T) {
	err := printShellInit("zsh")
	if err != nil {
		t.Fatalf("unexpected error for zsh: %v", err)
	}
}

func TestPrintShellInit_Bash(t *testing.T) {
	err := printShellInit("bash")
	if err != nil {
		t.Fatalf("unexpected error for bash: %v", err)
	}
}

func TestPrintShellInit_Unsupported(t *testing.T) {
	err := printShellInit("fish")
	if err == nil {
		t.Fatal("expected error for unsupported shell")
	}
	if !strings.Contains(err.Error(), "unsupported shell") {
		t.Errorf("expected 'unsupported shell' in error, got: %v", err)
	}
}

func TestShellInitScript_ContainsGwFunction(t *testing.T) {
	if !strings.Contains(shellInitScript, "gw()") {
		t.Error("shell init script should define gw() function")
	}
	if !strings.Contains(shellInitScript, "gwtui") {
		t.Error("shell init script should reference gwtui")
	}
	if !strings.Contains(shellInitScript, "cd") {
		t.Error("shell init script should contain cd command")
	}
}
