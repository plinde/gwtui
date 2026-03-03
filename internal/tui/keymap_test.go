package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
)

func TestDefaultKeyMap_AllBindingsPresent(t *testing.T) {
	km := defaultKeyMap()

	bindings := map[string]key.Binding{
		"Up":      km.Up,
		"Down":    km.Down,
		"Toggle":  km.Toggle,
		"All":     km.All,
		"None":    km.None,
		"Confirm": km.Confirm,
		"Enter":   km.Enter,
		"Help":    km.Help,
		"Back":    km.Back,
		"Refresh": km.Refresh,
		"Quit":    km.Quit,
	}

	for name, b := range bindings {
		if !b.Enabled() {
			t.Errorf("binding %s is not enabled", name)
		}
		help := b.Help()
		if help.Key == "" {
			t.Errorf("binding %s has no help key text", name)
		}
		if help.Desc == "" {
			t.Errorf("binding %s has no help description", name)
		}
	}
}

func TestDefaultKeyMap_ExpectedKeys(t *testing.T) {
	km := defaultKeyMap()

	tests := []struct {
		name     string
		binding  key.Binding
		wantKeys []string
	}{
		{"Up", km.Up, []string{"up", "k"}},
		{"Down", km.Down, []string{"down", "j"}},
		{"Toggle", km.Toggle, []string{" "}},
		{"All", km.All, []string{"a"}},
		{"None", km.None, []string{"n"}},
		{"Confirm", km.Confirm, []string{"tab"}},
		{"Enter", km.Enter, []string{"enter"}},
		{"Help", km.Help, []string{"?"}},
		{"Back", km.Back, []string{"backspace", "delete", "ctrl+h"}},
		{"Refresh", km.Refresh, []string{"r"}},
		{"Quit", km.Quit, []string{"q", "ctrl+c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys := tt.binding.Keys()
			if len(keys) != len(tt.wantKeys) {
				t.Errorf("expected %d keys, got %d: %v", len(tt.wantKeys), len(keys), keys)
				return
			}
			for i, k := range keys {
				if k != tt.wantKeys[i] {
					t.Errorf("key[%d]: expected %q, got %q", i, tt.wantKeys[i], k)
				}
			}
		})
	}
}
