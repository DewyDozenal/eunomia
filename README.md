# GitHub Projects TUI

A terminal application for managing issues on a GitHub Projects v2 board. It is built with Go, Bubble Tea, Bubbles, and Lip Gloss.

## Requirements

- Go 1.24 or newer
- [GitHub CLI](https://cli.github.com/) installed and authenticated with `gh auth login`
- Access to the project and permission to edit its issues and status field

The app uses `gh auth token` to obtain a token at launch. It does not save the token. The token needs project read/write and repository issue access; run `gh auth refresh -s project -s repo` if GitHub reports an authorization error.

## Run

```sh
go run .
```

Build and verify the application with `make`, or run individual checks:

```sh
make build
make test
make vet
```

Install the executable to `~/.local/bin/gh-project-tui` with:

```sh
make install
```

Override the install directory with `make install INSTALL_DIR=/path/to/bin`.

On first launch, enter a project URL, for example:

```text
https://github.com/orgs/OWNER/projects/1
https://github.com/users/OWNER/projects/1
```

The project must have a single-select field named **Status**. The selected URL is saved in the operating system's user config directory under `gh-project-tui/config.json`.

## Controls

| Key | Action |
| --- | --- |
| `←` / `→` or `h` / `l` | Select a status column |
| `↑` / `↓` or `k` / `j` | Select an issue |
| `Enter` | View issue details |
| `e` | Edit the issue title and description |
| `m` | Move the issue to a status |
| `r` | Refresh the board |
| `c` | Connect to a different project |
| `q` | Quit |

In the issue editor, use `Tab` to switch between title and description, `Ctrl+S` to save, and `Esc` to cancel. Issues without a status are grouped under **No status**.

The board renders issue cards with alternating gray backgrounds and shows each issue number, title, and assignees. With mouse support enabled in the terminal, drag a card and release it over another status column to change its status.
