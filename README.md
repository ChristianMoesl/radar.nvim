# radar.nvim

`radar.nvim` is a small Neovim radar for keeping track of engineering work that needs your attention.

The goal is to show a tiny statusline summary like:

```text
Radar 🔴1 🟡3 🔵2
```

Where:

- 🔴 needs immediate attention
- 🟡 needs attention
- 🔵 is in progress

Details are available from Neovim with `:Radar`.

## Architecture

This project is split into two parts:

- `radar.nvim`: a small Lua Neovim plugin for the statusline and detail UI
- `radar`: a Go binary with CLI commands and a daemon mode

Neovim talks to the daemon through a Unix socket. This lets multiple Neovim sessions share the same state without each session polling GitHub/Jira/etc. independently.

```text
Neovim statusline -> radar.nvim -> Unix socket -> radar daemon -> GitHub/Jira/Pi/etc.
```

## Current status

This is early scaffolding. The daemon currently uses the `gh` CLI to fetch GitHub review request notifications and open pull requests authored by you. Jira/Pi integrations will be added later.

## Build

```sh
go build -o radar ./cmd/radar
```

## Run

Start the daemon:

```sh
./radar daemon
```

Query it from the CLI:

```sh
./radar status
./radar items
./radar refresh
```

Stop or restart the daemon:

```sh
./radar stop
./radar restart
```

## GitHub

GitHub integration currently uses the GitHub CLI. Make sure this works first:

```sh
gh auth status
```

The daemon currently tracks:

- PR review requests assigned directly to you as `needs attention`
- open PRs authored by you as `in progress`

## Local state

The daemon stores the latest attention items locally:

```sh
./radar state-path
```

By default this is `$XDG_STATE_HOME/radar/items.json` or `~/.local/state/radar/items.json`.

Override it with `RADAR_STATE=/path/to/items.json`.

## Logs

The daemon writes logs to:

```sh
./radar log-path
```

By default this is `$XDG_STATE_HOME/radar/radar.log` or `~/.local/state/radar/radar.log`.

Follow logs with:

```sh
tail -f "$(./radar log-path)"
```

Override the log path with `RADAR_LOG=/path/to/radar.log`.

Development logs use a pretty human-readable colored format with source locations by default. Routine refresh details are hidden unless debug logging is enabled.

Set log level with:

```sh
RADAR_LOG_LEVEL=debug ./radar daemon
```

Supported levels: `debug`, `info`, `warn`, `error`. Default is `info`.

Set `RADAR_ENV=production` to disable source locations. Set `RADAR_LOG_COLOR=0` to disable colored logs.

## Neovim setup

Example:

```lua
require("radar").setup({
  radar_cmd = "/path/to/radar",
})
```

Statusline example:

```lua
vim.o.statusline = vim.o.statusline .. "%{v:lua.require'radar'.statusline()}"
```

Commands:

```vim
:Radar
:RadarRefresh
:RadarStart
```
