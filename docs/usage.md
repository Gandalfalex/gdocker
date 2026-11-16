# Usage and Keybindings

## Navigation

| Key | Action |
| --- | --- |
| `1-4` | Switch between containers, volumes, images, and networks |
| `j` / `k` / `up` / `down` | Move selection |
| `gg` / `G` | Jump to top or bottom (`42G` jumps to line 42) |
| `ctrl+d` / `ctrl+u` | Half page down or up |
| `H` / `M` / `L` | Top, middle or bottom of the screen |
| `zz` / `zt` / `zb` | Put the cursor line at center, top or bottom |
| `ctrl+e` / `ctrl+y` | Scroll the view without moving the cursor |
| `space` / `enter` | Expand or collapse a compose project |
| `esc` | Go back from sub-view or close help |
| `?` | Open help (scroll with `j` / `k`, close with `esc`, `q` or `?`) |
| `ctrl+c` | Quit immediately, from any view |

## Container actions (details view)

| Key | Action |
| --- | --- |
| `:s` / `:S` | Start or stop selected container |
| `r` | Restart selected container |
| `d` | Delete selected container, volume or image (asks `y` to confirm) |
| `l` | Open logs |
| `e` | Exec shell in container |
| `p` | Open ports view |
| `v` | Open environment view |
| `t` | Open stats (refreshes every 2s; `t` refreshes now) |
| `i` | Open inspect JSON |

## Command mode

Press `:` to open command mode.

| Command | Action |
| --- | --- |
| `:q`, `:quit` | Quit GDocker |
| `:s`, `:start` | Start selected container |
| `:S`, `:stop` | Stop selected container |
| `:help`, `:h` | Open help overlay |
| `:noh`, `:nohlsearch` | Clear log search highlight |
| `:ssh <alias|host|local>` | Switch Docker host at runtime |

Notes:

- `Tab` / `shift+tab` complete command names and `:ssh` targets; `up` / `down` recall earlier commands.
- Unique prefixes work (`:hel` for `:help`), and `:42` jumps to line 42 of the current view.
- These motions work in every scrolling view (list, logs, env, inspect).
- Pasting into the command line and the log search prompt works.
- Completion candidates come from `~/.ssh/config`, `docker.host_aliases`, and `local`.

## Logs view

| Key | Action |
| --- | --- |
| `j` / `k` | Scroll |
| `g` / `G` | Jump top or bottom |
| `/` or `?` | Search (jumps to the most recent match) |
| `n` / `N` | Next or previous match |
| `f` | Toggle follow mode (the view keeps up while the cursor is on the last line) |
| `w` | Toggle wrapping |
| `V` | Visual line selection |
| `v` | Visual character selection |
| `w` / `b` / `e` / `h` / `l` | Word and char movement in char-visual mode |
| `y` | Yank current line or selection |
| `esc` | Return to details |

## Env and inspect views

| Key | Action |
| --- | --- |
| `j` / `k` | Scroll |
| `g` / `G` | Jump top or bottom |
| `esc` | Return to details |

## Ports view

| Key | Action |
| --- | --- |
| `j` / `k` | Select port |
| `o` / `enter` | Open selected mapped port in browser |
| `esc` | Return to details |

## Help in app

- Press `?` or `:help` for the built-in key reference; it reflects your configured bindings.
