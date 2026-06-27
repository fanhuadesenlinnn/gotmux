package command

import (
	"reflect"
	"testing"
)

func TestParseStringCommands(t *testing.T) {
	got, err := ParseString(`new-session -d -s work; new-window -n "two words"; display-message 'hello world'`)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"new-session", "-d", "-s", "work"},
		{"new-window", "-n", "two words"},
		{"display-message", "hello world"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseString() = %#v, want %#v", got, want)
	}
}

func TestParseArgvSemicolon(t *testing.T) {
	got, err := ParseArgv([]string{"new", "-d", "-s", "work", ";", "ls"})
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"new", "-d", "-s", "work"}, {"ls"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseArgv() = %#v, want %#v", got, want)
	}
}

func TestParseScript(t *testing.T) {
	got, err := ParseScript(`
# ignored
set -g status off
bind-key C-a send-prefix; display-message "bound"
`)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"set", "-g", "status", "off"},
		{"bind-key", "C-a", "send-prefix"},
		{"display-message", "bound"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseScript() = %#v, want %#v", got, want)
	}
}
