# Development

## Build and run

```bash
go build -o gdocker .
./gdocker
```

Or with Makefile:

```bash
make build
make run
```

## Test

```bash
go test ./...
```

## Useful make targets

- `make build`
- `make run`
- `make test`
- `make fmt`
- `make vet`
- `make clean`

## Project layout

- `main.go`: app entrypoint
- `models/`: state, update loop, commands, handlers
- `docker/`: Docker client operations and data loading
- `ui/`: rendering and visual components
- `ui/logviewer/`: log view interactions and selection logic
- `config/`: config schema and loading

## Runtime wiring

The project avoids circular dependencies by wiring package-level function pointers during init.

Examples:

- `models` declares operation hooks (`LoadLogsFunc`, `ConnectDockerHostFunc`, etc.)
- `docker` and `ui` assign those hooks in `init()`

This keeps state/update logic centralized in `models` while delegating side effects and rendering to dedicated packages.
