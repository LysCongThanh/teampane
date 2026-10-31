package keys

import "testing"

func TestEncode(t *testing.T) {
	cases := []struct {
		args    []string
		literal bool
		want    string
	}{
		{[]string{"ls -la", "Enter"}, false, "ls -la\r"},
		{[]string{"a", "b"}, false, "ab"},
		{[]string{"C-c"}, false, "\x03"},
		{[]string{"C-d", "Up", "Escape"}, false, "\x04\x1b[A\x1b"},
		{[]string{"M-x"}, false, "\x1bx"},
		{[]string{"Enter", "C-c"}, true, "EnterC-c"},
		{[]string{"Space", "Tab", "BSpace"}, false, " \t\x7f"},
	}
	for _, c := range cases {
		if got := Encode(c.args, c.literal); got != c.want {
			t.Errorf("Encode(%q,%v) = %q, want %q", c.args, c.literal, got, c.want)
		}
	}
}

func TestCommandLine(t *testing.T) {
	cases := []struct {
		args []string
		want string
		ok   bool
	}{
		{[]string{"cd /x && claude --agent-id a", "Enter"}, "cd /x && claude --agent-id a", true},
		{[]string{"echo ", "hi", "C-m"}, "echo hi", true},
		{[]string{"echo hi"}, "", false},
		{[]string{"C-c", "Enter"}, "", false},
		{[]string{"   ", "Enter"}, "", false},
	}
	for _, c := range cases {
		got, ok := CommandLine(c.args, false)
		if got != c.want || ok != c.ok {
			t.Errorf("CommandLine(%q) = %q,%v want %q,%v", c.args, got, ok, c.want, c.ok)
		}
	}
	if _, ok := CommandLine([]string{"x", "Enter"}, true); ok {
		t.Error("literal mode must never be treated as a command line")
	}
}
