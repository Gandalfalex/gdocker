# GDocker

GDocker is a terminal UI for managing Docker containers, volumes, images, and networks.

## Install

### From source

```bash
git clone https://github.com/Gandalfalex/gdocker.git
cd gdocker
go build -o gdocker .
```

### Requirements

- Go 1.26+
- A reachable Docker daemon (local socket, `DOCKER_HOST`, or SSH endpoint)

## Quick start

```bash
./gdocker
```

Core controls:

- `1-4`: switch resource type (containers, volumes, images, networks)
- `j` / `k`: move cursor
- `space` / `enter`: expand or collapse compose project
- `:`: command mode
- `?` or `:help`: open full in-app help
- `:q`: quit

Remote host switch (inside GDocker command mode):

```text
:ssh orangepi
```

## Documentation

- [Documentation index](docs/README.md)
- [Usage and keybindings](docs/usage.md)
- [Configuration](docs/configuration.md)
- [Remote Docker over SSH](docs/ssh.md)
- [Development](docs/development.md)

## License

MIT. See [LICENSE](LICENSE).
