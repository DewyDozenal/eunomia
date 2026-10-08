# GitHub Projects TUI

A terminal application for managing issues on a GitHub Projects v2 board. It is built with Go, Bubble Tea, Bubbles, and Lip Gloss.

## Requirements

- Go 1.24 or newer
- A GitHub token supplied through `GH_TOKEN` or `GITHUB_TOKEN`, or the [GitHub CLI](https://cli.github.com/) authenticated with `gh auth login`
- Access to the project and permission to edit its issues and status field

To use a fine-grained personal access token, export it as `GH_TOKEN` (preferred) or `GITHUB_TOKEN` before starting the app:

```sh
export GH_TOKEN=github_pat_...
go run .
```

The app uses `GH_TOKEN` first, then `GITHUB_TOKEN`, and falls back to `gh auth token` when neither is set. It does not save the token. Configure the token for the selected project owner and repositories, with access to read/write the project and read/write issues in its repositories. For a classic token via GitHub CLI, run `gh auth refresh -s project -s repo` if GitHub reports an authorization error.

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
| `a` | Add a comment to the selected issue |
| `m` | Move the issue to a status |
| `r` | Refresh the board |
| `c` | Connect to a different project |
| `q` | Quit |

In the issue editor, use `Tab` to switch between title and description, `Ctrl+S` to save, and `Esc` to cancel. Press `a` while an issue is selected or open to compose a comment; `Ctrl+S` posts it and `Esc` cancels. Issues without a status are grouped under **No status**.

The board renders issue cards with alternating gray backgrounds and shows each issue number, title, and assignees. With mouse support enabled in the terminal, drag a card and release it over another status column to change its status.
