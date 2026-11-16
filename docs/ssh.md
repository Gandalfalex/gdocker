# Remote Docker over SSH

GDocker can switch Docker targets at runtime from command mode.

## Command

```text
:ssh <alias|host|local>
```

Examples:

```text
:ssh orangepi
:ssh user@192.168.178.70
:ssh ssh://docker@server
:ssh local
```

- `local` resets to local/default Docker context.
- Bare targets are treated as SSH targets.

## Target resolution order

When you run `:ssh orangepi`, GDocker resolves in this order:

1. `~/.ssh/config` host alias (preferred)
2. `docker.host_aliases` in `config.yaml`
3. fallback to `ssh://<input>`

Why SSH config first: it preserves OpenSSH behavior (`HostName`, `User`, `Port`, `IdentityFile`, `ProxyJump`, includes, etc.).

## SSH config support

GDocker reads `~/.ssh/config` and follows `Include` directives when checking aliases and completion options.

## Tab completion

In command mode, `Tab` after `ssh` completes known targets from:

- SSH config aliases
- `docker.host_aliases`
- `local`

Press `Tab` repeatedly to cycle candidates.

## Quick connectivity check

If a target does not connect, verify SSH access outside GDocker:

```bash
ssh orangepi docker system dial-stdio
```

Expected behavior is an HTTP response (commonly `HTTP/1.1 400 Bad Request`), which indicates Docker's SSH transport endpoint is reachable.
