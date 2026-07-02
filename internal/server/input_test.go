package server

import "testing"

func TestInputKeyNameSpecialKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		wantKey  string
		consumed int
	}{
		{name: "arrow", input: []byte("\x1b[A"), wantKey: "Up", consumed: 3},
		{name: "home csi", input: []byte("\x1b[H"), wantKey: "Home", consumed: 3},
		{name: "end ss3", input: []byte("\x1bOF"), wantKey: "End", consumed: 3},
		{name: "delete", input: []byte("\x1b[3~"), wantKey: "Delete", consumed: 4},
		{name: "page down", input: []byte("\x1b[6~"), wantKey: "PageDown", consumed: 4},
		{name: "f1 ss3", input: []byte("\x1bOP"), wantKey: "F1", consumed: 3},
		{name: "f5 csi", input: []byte("\x1b[15~"), wantKey: "F5", consumed: 5},
		{name: "f12 csi", input: []byte("\x1b[24~"), wantKey: "F12", consumed: 5},
		{name: "ctrl left", input: []byte("\x1b[1;5D"), wantKey: "C-Left", consumed: 6},
		{name: "shift page up", input: []byte("\x1b[5;2~"), wantKey: "S-PageUp", consumed: 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKey, consumed := inputKeyName(tt.input)
			if gotKey != tt.wantKey || consumed != tt.consumed {
				t.Fatalf("inputKeyName() = %q, %d; want %q, %d", gotKey, consumed, tt.wantKey, tt.consumed)
			}
		})
	}
}
