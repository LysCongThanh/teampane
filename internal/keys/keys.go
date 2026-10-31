// Package keys translates tmux send-keys arguments into the bytes a
// terminal would receive, following tmux semantics: each argument is either
// a key name (Enter, C-c, Up, ...) or literal text; arguments are
// concatenated without separators.
package keys

import "strings"

// named maps tmux key names (lower-cased) to terminal input bytes.
var named = map[string]string{
	"enter": "\r", "c-m": "\r", "kpenter": "\r",
	"tab": "\t", "c-i": "\t", "btab": "\x1b[Z",
	"space": " ", "escape": "\x1b", "c-[": "\x1b",
	"bspace": "\x7f", "dc": "\x1b[3~", "ic": "\x1b[2~",
	"up": "\x1b[A", "down": "\x1b[B", "right": "\x1b[C", "left": "\x1b[D",
	"home": "\x1b[H", "end": "\x1b[F",
	"ppage": "\x1b[5~", "pageup": "\x1b[5~", "pgup": "\x1b[5~",
	"npage": "\x1b[6~", "pagedown": "\x1b[6~", "pgdn": "\x1b[6~",
}

// Lookup returns the bytes for a single tmux key name.
func Lookup(key string) (string, bool) {
	k := strings.ToLower(key)
	if v, ok := named[k]; ok {
		return v, true
	}
	// C-a .. C-z and C-@ .. C-_ (control characters).
	if len(k) == 3 && k[0] == 'c' && k[1] == '-' {
		ch := k[2]
		switch {
		case ch >= 'a' && ch <= 'z':
			return string(rune(ch - 'a' + 1)), true
		case ch >= '@' && ch <= '_':
			return string(rune(ch - '@')), true
		}
	}
	// M-x: Escape prefix (meta).
	if len(key) == 3 && (key[0] == 'M' || key[0] == 'm') && key[1] == '-' {
		return "\x1b" + key[2:], true
	}
	return "", false
}

// Encode converts send-keys arguments into terminal input. With literal
// (-l) every argument is sent verbatim.
func Encode(args []string, literal bool) string {
	var b strings.Builder
	for _, a := range args {
		if !literal {
			if v, ok := Lookup(a); ok {
				b.WriteString(v)
				continue
			}
		}
		b.WriteString(a)
	}
	return b.String()
}

// CommandLine reports whether args form "text... Enter": plain text
// arguments terminated by a single Enter key. It returns the text, which is
// what a shell would execute. This is how Claude Code (and #40168) start a
// teammate when split-window is called without a command.
func CommandLine(args []string, literal bool) (string, bool) {
	if literal || len(args) < 2 {
		return "", false
	}
	last := strings.ToLower(args[len(args)-1])
	if last != "enter" && last != "c-m" {
		return "", false
	}
	var b strings.Builder
	for _, a := range args[:len(args)-1] {
		if _, isKey := Lookup(a); isKey {
			return "", false
		}
		b.WriteString(a)
	}
	text := b.String()
	if strings.TrimSpace(text) == "" {
		return "", false
	}
	return text, true
}
