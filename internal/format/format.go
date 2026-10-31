// Package format expands the subset of tmux format strings ("#{pane_id}",
// "#D", "##", ...) that callers such as Claude Code ask for.
package format

import "strings"

// shortAliases maps tmux single-character format aliases to variable names.
var shortAliases = map[byte]string{
	'D': "pane_id",
	'P': "pane_index",
	'S': "session_name",
	'I': "window_index",
	'W': "window_name",
	'T': "pane_title",
	'H': "host",
}

// Expand replaces format variables in f with values from vars. Variables
// that are not in vars, and any construct teampane cannot evaluate
// (conditionals, modifiers, nested formats), are reported in missing and
// expand to the empty string, mirroring tmux's behaviour for unknown names.
// Callers should treat a non-empty missing list as "cannot answer".
func Expand(f string, vars map[string]string) (out string, missing []string) {
	var b strings.Builder
	for i := 0; i < len(f); i++ {
		c := f[i]
		if c != '#' || i+1 >= len(f) {
			b.WriteByte(c)
			continue
		}
		next := f[i+1]
		switch {
		case next == '#':
			b.WriteByte('#')
			i++
		case next == '{':
			end := strings.IndexByte(f[i+2:], '}')
			if end < 0 {
				// Unterminated: tmux prints it literally.
				b.WriteString(f[i:])
				return b.String(), missing
			}
			name := f[i+2 : i+2+end]
			if v, ok := lookup(name, vars); ok {
				b.WriteString(v)
			} else {
				missing = append(missing, name)
			}
			i += 2 + end
		default:
			if name, ok := shortAliases[next]; ok {
				if v, ok := vars[name]; ok {
					b.WriteString(v)
				} else {
					missing = append(missing, name)
				}
				i++
				continue
			}
			b.WriteByte(c)
		}
	}
	return b.String(), missing
}

// lookup resolves a plain variable name. Names with modifiers or
// conditionals ("?", ":", "{", "=") are never resolved.
func lookup(name string, vars map[string]string) (string, bool) {
	if name == "" || strings.ContainsAny(name, "?:{=,") {
		return "", false
	}
	v, ok := vars[name]
	return v, ok
}
