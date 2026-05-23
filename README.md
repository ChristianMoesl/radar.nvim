# cockpit.nvim

`cockpit.nvim` is a small Neovim cockpit for keeping track of engineering work that needs your attention.

The goal is to show a tiny statusline summary like:

```text
Cockpit 🔴1 🟡3 🔵2
```

Where:

- 🔴 needs immediate attention
- 🟡 needs attention
- 🔵 is in progress

Details are available from Neovim with `:Cockpit`.

## Architecture

This project is split into two parts:

- `cockpit.nvim`: a small Lua Neovim plugin for the statusline and detail UI
- `cockpit`: a Go binary with CLI commands and a daemon mode

Neovim talks to the daemon through a Unix socket. This lets multiple Neovim sessions share the same state without each session polling GitHub/Jira/etc. independently.

```text
Neovim statusline -> cockpit.nvim -> Unix socket -> cockpit daemon -> GitHub/Jira/Pi/etc.
```

## Current status

This is early scaffolding. The daemon currently uses the `gh` CLI to fetch GitHub review request notifications and open pull requests authored by you. Jira/Pi integrations will be added later.

## Build

```sh
go build -o cockpit ./cmd/cockpit
```

## Run

Start the daemon:

```sh
./cockpit daemon
```

Query it from the CLI:

```sh
./cockpit status
./cockpit items
./cockpit refresh
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
./cockpit state-path
```

By default this is `$XDG_STATE_HOME/cockpit/items.json` or `~/.local/state/cockpit/items.json`.

Override it with `COCKPIT_STATE=/path/to/items.json`.

## Logs

The daemon writes logs to:

```sh
./cockpit log-path
```

By default this is `$XDG_STATE_HOME/cockpit/cockpit.log` or `~/.local/state/cockpit/cockpit.log`.

Follow logs with:

```sh
tail -f "$(./cockpit log-path)"
```

Override the log path with `COCKPIT_LOG=/path/to/cockpit.log`.

Development logs use a pretty human-readable colored format with source locations by default. Set `COCKPIT_ENV=production` to disable source locations. Set `COCKPIT_LOG_COLOR=0` to disable colored logs.

## Neovim setup

Example:

```lua
require("cockpit").setup({
  cockpit_cmd = "/path/to/cockpit",
})
```

Statusline example:

```lua
vim.o.statusline = vim.o.statusline .. "%{v:lua.require'cockpit'.statusline()}"
```

Commands:

```vim
:Cockpit
:CockpitRefresh
:CockpitStart
```
