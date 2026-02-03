# gdocker - Docker TUI

A lightweight terminal user interface for managing Docker containers and Docker Compose environments, built with [Bubble Tea](https://github.com/charmbracelet/bubbletea). Features Vim-style navigation and a clean interface.

## Features

- **Container Management**: View, start, stop, restart, and delete containers
- **Docker Compose Support**: Group and manage containers by compose projects
- **Live Logs**: View and scroll through container logs
- **Interactive Shell**: Exec into containers with automatic shell detection
- **Port Mappings**: View ports and open them directly in your browser
- **Environment Variables**: Inspect container environment variables
- **Vim Keybindings**: Navigate with `j/k/h/l`, jump with `gg/G`, and more

## Installation

### Prerequisites

- Go 1.21 or higher
- Docker daemon running

### Build from Source

```bash
cd /Users/ich/projects/gdocker
go build -o gdocker
```

### Run

```bash
./gdocker
```

## Keybindings

### Navigation

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `g` | Jump to top (press twice: `gg`) |
| `G` | Jump to bottom |
| `enter` / `space` | Expand/collapse compose project |

### Container Actions

| Key | Action |
|-----|--------|
| `s` | Start selected container |
| `S` | Stop selected container |
| `r` | Restart selected container |
| `d` | Delete selected container |
| `e` | Exec into container (interactive shell) |

### Views

| Key | Action |
|-----|--------|
| `l` | View container logs |
| `p` | View port mappings |
| `v` | View environment variables |
| `o` | Open selected port in browser (from ports view) |
| `esc` | Go back to details view |

### Application

| Key | Action |
|-----|--------|
| `q` | Quit application |
| `ctrl+c` | Force quit |

## Usage

### Basic Workflow

1. **Navigate**: Use `j/k` to move through containers
2. **Expand Compose**: Press `enter` on a compose project to see all containers
3. **View Details**: Container details show in the right panel
4. **Quick Actions**:
   - Press `s` to start a stopped container
   - Press `S` to stop a running container
   - Press `e` to open a shell inside the container
   - Press `l` to view logs
   - Press `p` to see port mappings
   - Press `v` to see environment variables

### Port Mapping & Browser

1. Select a container
2. Press `p` to view its port mappings
3. Use `j/k` to select a port
4. Press `o` or `enter` to open `http://localhost:[port]` in your browser
5. Works with common HTTP ports (80, 8080, 3000, 5000, etc.)

### Interactive Shell

1. Select a running container
2. Press `e` to exec into it
3. The TUI suspends and gives you an interactive shell
4. Type `exit` or press `Ctrl+D` to return to the TUI
5. Automatically detects available shells (`/bin/bash`, `/bin/sh`, `/bin/ash`)

## Project Structure

```
gdocker/
├── main.go         # Single-file implementation (~1000 lines)
├── go.mod          # Go module definition
├── go.sum          # Dependency checksums
├── gdocker         # Compiled binary
└── README.md       # This file
```

## Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components
- [Moby/Docker](https://github.com/moby/moby) - Docker client SDK

## Screenshots

```
┌─────────────────────────────────────────────────────────────────────┐
│  Docker TUI                                                         │
├──────────────────┬──────────────────────────────────────────────────┤
│                  │                                                  │
│  COMPOSE         │  Container Details                               │
│  ▼ my-app (8)    │                                                  │
│    ● nginx       │  Name: nginx                                     │
│    ● postgres    │  Status: Running                                 │
│    ● redis       │  Image: nginx:latest                             │
│                  │  Ports: 3 mapped                                 │
│                  │                                                  │
│                  │  l: logs • p: ports • v: env • e: exec           │
│                  │                                                  │
├──────────────────┴──────────────────────────────────────────────────┤
│  j/k: nav • s: start • S: stop • p: ports • v: env • e: exec       │
└─────────────────────────────────────────────────────────────────────┘
```

## Performance

- **Single file**: ~1050 lines of Go
- **Binary size**: ~11MB
- **Memory usage**: Minimal (native Go)
- **Startup time**: Instant

## License

MIT

## Contributing

This is a personal project, but feel free to fork and modify!
