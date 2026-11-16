# Configuration

GDocker loads config from:

- Linux/macOS: `~/.config/gdocker/config.yaml`

Start from the repository example:

```bash
mkdir -p ~/.config/gdocker
cp config.yaml.example ~/.config/gdocker/config.yaml
```

## Sections

Top-level sections:

- `keybindings`
- `ui`
- `docker`

## Keybindings

You can bind multiple keys per action.

```yaml
keybindings:
  navigation:
    down: ["j", "down", "ctrl+n"]
```

Key names follow Bubble Tea v2: `space`, `enter`, `esc`, `tab`, `ctrl+c`, `shift+tab`, ... (a literal `" "` from older configs is still accepted for space). If the file cannot be parsed or two actions share a key, GDocker starts with the defaults and shows the reason in the status bar.

Important: command-mode commands themselves (`:q`, `:ssh`, etc.) are not remapped via `keybindings`; only the key to enter command mode is configurable (`keybindings.commands.enter`).

## UI options

| Key | Default | Description |
| --- | --- | --- |
| `ui.show_header_context` | `true` | Show `[resource • view]` in header |
| `ui.show_list_help_hint` | `true` | Show list-pane hint text |
| `ui.show_line_numbers` | `true` | Show line numbers in logs and inspect |
| `ui.wrap_logs` | `true` | Wrap log lines by default |
| `ui.max_project_preview_items` | `8` | Max containers shown in project detail preview |
| `ui.max_container_port_preview` | `4` | Max ports shown in details preview |
| `ui.max_image_tag_preview` | `6` | Max tags shown in image detail preview |

## Docker options

| Key | Default | Description |
| --- | --- | --- |
| `docker.host` | `""` | Docker endpoint. Empty uses environment/default socket |
| `docker.host_aliases` | `{}` | Aliases for `:ssh` command targets |
| `docker.auto_refresh_seconds` | `10` | Container list auto-refresh interval |
| `docker.log_tail_lines` | `1000` | Initial number of log lines loaded |

Example:

```yaml
docker:
  host: ""
  host_aliases:
    orangepi: "ssh://pi@orangepi"
    prod: "ssh://docker@10.0.0.20"
  auto_refresh_seconds: 10
  log_tail_lines: 1000
```

## Validation behavior

When GDocker loads config it sanitizes values:

- Empty alias names or values in `docker.host_aliases` are removed.
- `docker.auto_refresh_seconds < 1` resets to `10`.
- `docker.log_tail_lines < 1` resets to `1000`.
- UI preview counts below `1` are reset to `1`.
