# Architecture

`cockpit.nvim` is split into a small Neovim UI plugin and a Go backend daemon.

## Components

- `lua/cockpit/`: Neovim integration, statusline component, notifications, and floating detail window.
- `cmd/cockpit/`: single Go binary with CLI and daemon mode.
- `internal/server/`: Unix socket API used by Neovim and CLI commands.
- `internal/collector/`: orchestrates data collection from external systems.
- `internal/github/`: GitHub collectors and remote state resolution.
- `internal/state/`: local persistent item store.

## Process model

There is one long-running daemon per user:

```text
Neovim sessions -> cockpit CLI -> Unix socket -> cockpit daemon -> collectors
```

Multiple Neovim sessions share the same daemon and state. This avoids duplicated polling and keeps statusline reads fast.

The binary is intentionally single-file from a user perspective:

```sh
cockpit daemon
cockpit status
cockpit items
cockpit refresh
cockpit stop
cockpit restart
```

## Communication

Neovim does not call GitHub directly. It shells out to `cockpit`, which talks to the daemon over a Unix socket.

The socket protocol is newline-delimited JSON with a tiny request model:

```json
{ "method": "items" }
{ "method": "summary" }
{ "method": "refresh" }
```

## Item lifecycle

Cockpit has three active categories and one historical category:

- `immediate`
- `attention`
- `in_progress`
- `done`

Collectors ingest only active items. They should not blindly import already-completed remote items.

`done` is derived from previously tracked items. If an item was active in the local store and later disappears from the active collector result, the relevant integration checks the remote state. If the remote item resolved today, Cockpit moves it to `done`.

This keeps `done` meaningful: it means “something Cockpit was tracking became resolved today.”

## Local state

The daemon stores the latest known items on disk:

```text
$XDG_STATE_HOME/cockpit/items.json
```

This allows fast startup and lets Neovim show cached information immediately.

## GitHub integration

GitHub access currently uses the `gh` CLI.

Current GitHub collectors:

- review requests assigned directly to the user -> `attention`
- open PRs authored by the user -> `in_progress`

Resolution:

- previously tracked authored PR disappears from open PRs
- daemon fetches that PR remotely
- if it was closed or merged today, it becomes `done`

## Neovim UI

The statusline is the primary UI and must stay cheap and glanceable.

The floating window is secondary and shows detailed item information. Opening it should use cached daemon state and must not block on network refreshes.

Manual refresh is available from the floating window with `r`.

## Logging

The daemon writes logs to:

```text
$XDG_STATE_HOME/cockpit/cockpit.log
```

Default log level is `info`. Routine polling details should stay at `debug` to avoid excessive disk logging.

Use:

```sh
COCKPIT_LOG_LEVEL=debug cockpit daemon
```

for development debugging.
