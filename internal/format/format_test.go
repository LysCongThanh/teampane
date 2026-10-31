package format

import (
	"reflect"
	"testing"
)

func TestExpand(t *testing.T) {
	vars := map[string]string{
		"pane_id": "%3", "session_name": "teampane", "window_id": "@1",
		"window_index": "1", "pane_index": "2", "pane_title": "researcher",
	}
	cases := []struct {
		in, want string
		missing  []string
	}{
		{"#{pane_id}", "%3", nil},
		{"#{session_name}:#{window_id}.#{pane_id}", "teampane:@1.%3", nil},
		{"#S:#I.#P", "teampane:1.2", nil},
		{"#D #T", "%3 researcher", nil},
		{"100## done", "100# done", nil},
		{"plain text", "plain text", nil},
		{"#{pane_width}x", "x", []string{"pane_width"}},
		{"#{?pane_active,yes,no}", "", []string{"?pane_active,yes,no"}},
		{"#{t:window_activity}", "", []string{"t:window_activity"}},
		{"open #{pane_id", "open #{pane_id", nil},
		{"#X", "#X", nil},
		{"trailing #", "trailing #", nil},
	}
	for _, c := range cases {
		got, missing := Expand(c.in, vars)
		if got != c.want || !reflect.DeepEqual(missing, c.missing) {
			t.Errorf("Expand(%q) = %q %v, want %q %v", c.in, got, missing, c.want, c.missing)
		}
	}
}

// Claude Code probes terminal capabilities with a tab-separated format of
// variables teampane cannot know; every one must be reported missing.
func TestExpandReportsAllUnknownProbeVars(t *testing.T) {
	probe := "#{client_termtype}\t#{allow-passthrough}\t#{focus-events}\t#{pane_top}"
	_, missing := Expand(probe, map[string]string{"pane_id": "%1"})
	if len(missing) != 4 {
		t.Fatalf("missing = %v", missing)
	}
}
