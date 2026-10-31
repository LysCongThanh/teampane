# Contributing to teampane

Thanks for helping. Small, focused PRs are easiest to review.

## Ground rules

- Go files stay under ~200 lines, `snake_case.go` names, a comment on every exported identifier.
- Real tests only: unit tests for pure code, golden command lines for backends, and the end-to-end suite in
  `internal/e2e` (it builds the binary and drives it through a `tmux` symlink with the headless backend).
- `go vet ./...` and `go test ./...` must pass. CI also runs `golangci-lint` (advisory).
- Never send anything to a terminal you did not create: tests use the headless backend unless an explicit
  opt-in variable is set.

## Reporting an unsupported tmux call

Run Claude Code with `TEAMPANE_DEBUG=1 teampane run claude`, reproduce, then attach
`$XDG_STATE_HOME/teampane/unknown.log` (and `debug.log` if relevant) to an issue together with `claude --version`.
Check it for anything private first — the logs contain the tmux arguments.

## Adding a backend

A backend is one file in `internal/backend/` implementing `Backend` (see `backend.go`):

```go
type Backend interface {
	Name() string
	CurrentPane() string // native id of the calling pane, from env
	ScopeKey() string    // identifies the terminal instance (socket, session)
	Split(req SplitRequest) (native string, err error)
	SendText(native, text string) error
	GetText(native string, scrollback bool) (string, error)
	Close(native string) error // closing a gone pane must not fail
	SetTitle(native, title string) error
	Focus(native string) error
	Check() []Finding // used by `teampane doctor`
}
```

Steps:

1. Create `internal/backend/<name>.go`. Run every external command through the `Runner` you are given
   (`R.Run(Cmd{...})`) — never call `os/exec` directly — so command lines can be golden-tested.
2. `Split` must start `req.Argv` (the pane host) in the new pane, place it next to `req.Target`, honour
   `req.Focus == false` (tmux `-d`) when the terminal allows it, and return the native id of the new pane.
3. Register it in `New` and, if it can be auto-detected, in `DetectName` (innermost multiplexer first) and add its
   pane-id variable to `backendPaneEnv` in `internal/panehost/pane_files.go`.
4. Add a golden test: call `exercise(t, &YourBackend{R: r})` in `backend_golden_test.go`, run
   `go test ./internal/backend -update`, and review `testdata/<name>.golden` line by line.
5. Optional: an integration test like `kitty_integration_test.go`, skipped unless `TEAMPANE_IT_<NAME>=1`.
6. Document setup requirements in the README table.

## Commit messages

Conventional commits (`feat:`, `fix:`, `test:`, `docs:`, `refactor:`, `ci:`).
