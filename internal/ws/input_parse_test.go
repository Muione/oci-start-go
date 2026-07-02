// Package ws — input_parse_test.go: regression for the silent input-drop bug.
// The browser sends {"type":"input","data":"<keystrokes>"} where `data` is a
// JSON STRING (the keystrokes), not an object. The old code unmarshaled into
// struct{Data string} (expecting {"data":"..."}) which silently failed on a
// raw string → d.Data empty → no bytes written to ssh stdin → typing did
// nothing. parseInputData unmarshals the raw JSON string directly.
package ws

import (
	"encoding/json"
	"testing"
)

func TestParseInputData(t *testing.T) {
	cases := []struct {
		name string
		raw  string // a JSON string literal (what the browser sends as the `data` field)
		want string
	}{
		{"plain", `"hello"`, "hello"},
		{"carriage return", `"\r"`, "\r"},
		{"newline", `"\n"`, "\n"},
		{"tab", `"\t"`, "\t"},
		{"empty string", `""`, ""},
		{"multi-char with CR", `"ls -la\r"`, "ls -la\r"},
	}
	for _, c := range cases {
		got, err := parseInputData(json.RawMessage(c.raw))
		if err != nil {
			t.Errorf("%s: err=%v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestParseInputData_RejectsObject(t *testing.T) {
	// An object {"data":"a"} must be rejected (the old buggy shape) — the
	// browser never sends this for input, but if it did we shouldn't silently
	// accept garbage.
	_, err := parseInputData(json.RawMessage(`{"data":"a"}`))
	if err == nil {
		t.Error("object should fail to parse as a string")
	}
}
