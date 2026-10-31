// Command teampane makes Claude Code Agent Teams' split-pane mode work in
// kitty, WezTerm and zellij by acting as a tmux-compatible shim.
//
// Invoked as `tmux` (through the symlink `teampane install` creates) it
// handles tmux commands; invoked as `teampane` it is a normal CLI.
package main

import (
	"os"
	"path/filepath"

	"github.com/LysCongThanh/teampane/internal/cli"
	"github.com/LysCongThanh/teampane/internal/shim"
)

func main() {
	if filepath.Base(os.Args[0]) == "tmux" {
		os.Exit(shim.Main(os.Args[1:]))
	}
	os.Exit(cli.Main(os.Args[1:]))
}
