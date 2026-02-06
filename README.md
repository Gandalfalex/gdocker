# GDocker

A modern, vim-inspired terminal user interface (TUI) for managing Docker containers, volumes, images, and networks. Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) for a fast, responsive experience.

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue.svg)

## ✨ Features

- 🐳 **Complete Docker Management**: Containers, volumes, images, and networks
- 📦 **Docker Compose Support**: Automatic project grouping and management
- ⌨️ **Vim-like Commands**: Familiar `:q`, `:help`, and command mode
- 📊 **Live Container Stats**: Real-time CPU, memory, and network monitoring
- 📝 **Smart Log Viewer**: Search, navigate, and highlight log entries
- 🔍 **Container Inspect**: View full JSON configuration
- 🌐 **Port Management**: Quick browser launch for exposed ports
- 🎨 **Clean UI**: Color-coded status indicators and intuitive navigation
- 🚀 **Lightweight**: Single binary, minimal dependencies

## 📦 Installation

### From Source

```bash
git clone https://github.com/Gandalfalex/gdocker.git
cd gdocker
make build
make install  # Installs to /usr/local/bin
```

Or manually:

```bash
go build -o gdocker
./gdocker
```

### Prerequisites

- Go 1.21 or higher
- Docker daemon running
- Access to Docker socket (typically `/var/run/docker.sock`)

## 🎮 Usage

Simply run:

```bash
gdocker
```

### Quick Start

1. Use **1-4** to switch between containers, volumes, images, and networks
2. Navigate with **j/k** (vim-style) or arrow keys
3. Press **:** to enter command mode
4. Type **:help** for a complete command reference
5. Press **:q** to quit

## ⌨️ Keybindings

### Navigation

| Key | Action |
|-----|--------|
| `1-4` | Switch between containers/volumes/images/networks |
| `j` / `k` / `↓` / `↑` | Move cursor down/up |
| `g` / `G` | Jump to top/bottom |
| `space` / `enter` | Toggle project expansion |
| `esc` | Go back / close view |

### Command Mode

Press `:` to enter command mode, then type:

| Command | Action |
|---------|--------|
| `:q` / `:quit` | Quit application |
| `:s` / `:start` | Start selected container |
| `:S` / `:stop` | Stop selected container |
| `:help` / `:h` | Show help window |
| `:noh` | Clear search highlighting |

### Container Actions (Details View)

| Key | Action |
|-----|--------|
| `r` | Restart container |
| `d` | Delete container/volume/image |
| `l` | View logs |
| `e` | Execute shell (docker exec) |
| `p` | View port mappings |
| `v` | View environment variables |
| `t` | View/refresh stats |
| `i` | View inspect (JSON) |

### Logs View

| Key | Action |
|-----|--------|
| `j` / `k` | Scroll logs |
| `g` / `G` | Jump to top/bottom |
| `?` | Search in logs |
| `n` / `N` | Next/previous search result |
| `:noh` | Clear search highlighting |
| `esc` | Back to details |

### Ports View

| Key | Action |
|-----|--------|
| `j` / `k` | Select port |
| `o` / `enter` | Open port in browser |
| `esc` | Back to details |

### Stats View

| Key | Action |
|-----|--------|
| `t` | Refresh stats |
| `esc` | Back to details |

## 🎯 Features in Detail

### Docker Compose Integration

GDocker automatically detects and groups containers by their compose project:

```
▼ my-project (3 containers)
  ● web
  ● database
  ● cache
```

Press `space` or `enter` to expand/collapse projects.

### Smart Log Search

1. Press `l` to view container logs
2. Press `?` to start searching
3. Type your search term and press `Enter`
4. Navigate results with `n` (next) and `N` (previous)
5. Use `:noh` to clear highlighting

### Port Management

1. Select a container and press `p`
2. Navigate ports with `j/k`
3. Press `o` or `enter` to open `http://localhost:[port]` in your browser
4. Works with any mapped HTTP port

### Interactive Shell

1. Select a running container
2. Press `e` to exec into it
3. Automatically detects available shells (`bash`, `sh`, `ash`)
4. Type `exit` to return to GDocker

### Real-time Stats

1. Select a running container
2. Press `t` to view live stats
3. Shows CPU %, memory usage, network I/O, block I/O, and PIDs
4. Press `t` again to refresh

### Container Inspect

1. Select a container
2. Press `i` to view full JSON configuration
3. Scroll through with `j/k` or `g/G`

## ⚙️ Configuration

GDocker supports custom keybindings through a YAML configuration file.

### Creating a Config File

1. Copy the example configuration:
   ```bash
   cp config.yaml.example ~/.config/gdocker/config.yaml
   ```

2. Edit the file to customize your keybindings:
   ```bash
   nano ~/.config/gdocker/config.yaml
   ```

### Configuration Format

The config file uses YAML format. Here's an example:

```yaml
keybindings:
  navigation:
    up: ["k", "up"]              # Move cursor up
    down: ["j", "down"]          # Move cursor down
    top: ["g"]                   # Jump to top
    bottom: ["G"]                # Jump to bottom
    toggle_expand: [" ", "enter"] # Toggle project expansion
    switch_container: ["1"]      # Switch to containers view
    switch_volume: ["2"]         # Switch to volumes view
    switch_image: ["3"]          # Switch to images view
    switch_network: ["4"]        # Switch to networks view

  container:
    restart: ["r"]               # Restart container
    delete: ["d"]                # Delete container/volume/image
    logs: ["l"]                  # View logs
    exec: ["e"]                  # Execute shell
    ports: ["p"]                 # View ports
    env: ["v"]                   # View environment variables
    stats: ["t"]                 # View stats
    inspect: ["i"]               # View inspect JSON
    open_port: ["o", "enter"]    # Open port in browser
    refresh_stats: ["t"]         # Refresh stats

  logs:
    search: ["?"]                # Start search
    next_result: ["n"]           # Next search result
    prev_result: ["N"]           # Previous search result

  commands:
    enter: [":"]                 # Enter command mode

  views:
    back: ["esc"]                # Go back / close view

  general:
    force_quit: ["ctrl+c"]       # Force quit application
```

### Multiple Key Bindings

You can assign multiple keys to the same action by using an array:

```yaml
navigation:
  down: ["j", "down", "ctrl+n"]  # All three keys will move down
```

### Default Configuration

If no config file is found at `~/.config/gdocker/config.yaml`, GDocker uses the default vim-like keybindings shown in the keybindings section above.

### Config Location

- **Linux/macOS**: `~/.config/gdocker/config.yaml`
- **Example file**: `config.yaml.example` in the repository

## 🏗️ Project Structure

```
gdocker/
├── main.go              # Application entry point
├── models/
│   ├── models.go        # Data structures and types
│   ├── update.go        # State management and key handlers
│   ├── state.go         # Future architecture (composition)
│   └── builder.go       # Model builder pattern
├── docker/
│   ├── operations.go    # Docker API operations
│   └── information.go   # Data loading functions
├── ui/
│   └── ui.go           # Rendering and UI components
├── config/
│   └── keybindings.go  # Configuration and keybinding management
├── config.yaml.example # Example configuration file
├── Makefile            # Build and install targets
└── README.md           # This file
```

## 🛠️ Development

### Building

```bash
make build          # Build binary
make install        # Install to /usr/local/bin
make uninstall      # Remove from system
```

### Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [Docker SDK](https://github.com/moby/moby) - Docker API client
- [YAML v3](https://github.com/go-yaml/yaml) - Configuration file parsing

### Code Organization

The project follows a clean architecture with separation of concerns:

- **models/** - Core business logic and state management
- **docker/** - Docker API interactions
- **ui/** - View rendering and styling
- **config/** - Configuration and keybinding management
- **main.go** - Application initialization

Function pointers are used to avoid circular imports while maintaining clean package boundaries.

## 🎨 Screenshots

### Main View
```
╭──────────────────────────────────────────────────────────────────╮
│ 🐳 GDocker    Containers: 8 (5 running)  Volumes: 12  Images: 20 │
├─────────────────────────┬────────────────────────────────────────┤
│ Containers [8]          │ Details                                │
│                         │                                        │
│ ▼ coding-agent-workflow │ Name: coding-agent-workflow-web        │
│   ● web                 │ Status: Running                        │
│   ● database            │ Image: nginx:alpine                    │
│   ● redis               │ ID: 9cf427...                          │
│ ▼ myapp                 │ Project: coding-agent-workflow         │
│   ● app                 │ Created: 2 days ago                    │
│   ■ worker              │                                        │
│ ● standalone-nginx      │ Ports: 1 mapped                        │
│                         │                                        │
├─────────────────────────┴────────────────────────────────────────┤
│ :s: start • :S: stop • r: restart • d: delete • l: logs • :: cmd │
╰──────────────────────────────────────────────────────────────────╯
```

### Help Window
Press `:help` to see the comprehensive help overlay with all commands and shortcuts.

## 🚀 Performance

- **Binary Size**: ~12MB (statically linked)
- **Memory Usage**: < 20MB typical
- **Startup Time**: Instant
- **Language**: Pure Go (no runtime dependencies)

## 📋 Roadmap

- [ ] Browse volume contents
- [ ] Follow logs in real-time (tail -f)
- [ ] Network management operations
- [ ] Container creation wizard
- [ ] Export/import configurations
- [ ] Multi-container actions
- [ ] Custom color themes

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For major changes:

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - Excellent TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Beautiful terminal styling
- [Lazydocker](https://github.com/jesseduffield/lazydocker) - Inspiration for the project
- The Docker community for amazing tools and documentation

## License

- GitHub: [@Gandalfalex](https://github.com/Gandalfalex)
- Issues: [GitHub Issues](https://github.com/Gandalfalex/gdocker/issues)

---

This is a personal project, but feel free to fork and modify!
