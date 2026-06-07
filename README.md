# luctl

A command-line tool for managing a self-hosted [Luanti](https://www.luanti.org/) server.
It wraps the [ContentDB REST API](https://content.luanti.org/help/api/) and introduces
a declarative `luanti.toml` project manifest — similar to `package.json` or `pyproject.toml`
— so your server configuration, mod list, and world settings are all version-controlled
in one place.

The Luanti client ships a built-in Content browser, but it installs mods into the
*client's* local directory — not a remote server. `luctl` manages packages, configuration,
and backups directly on the server side.

---

## Features

- **Package management** — search ContentDB, install, update, and list mods and games;
  installing automatically enables the mod in `world.mt` and records it in `luanti.toml`
- **Mod enable/disable** — toggle `load_mod_<name>` in `world.mt` by name without editing
  any file manually
- **Project manifest** — `luanti.toml` declares server identity, filesystem paths, mod list,
  and arbitrary `minetest.conf` keys; commit it to reproduce the full server on any machine
- **Config sync** — push `[config]` values from `luanti.toml` into `minetest.conf` in one
  command, updating existing keys in-place without touching comments or unrelated settings
- **Server backups** — upload timestamped `tar.gz` archives to any S3-compatible provider;
  safe to run against a live server via SQLite `VACUUM INTO`
- **Server restore** — download and extract a named backup (or the most recent) into the
  world directory, with an interactive confirmation prompt and `--force` for automation
- **Shell autocompletion** — bash, zsh, fish, and PowerShell via `luctl completion`
- **Cross-platform** — single static binary, no CGo, no runtime dependencies

---

## Requirements

- **Go 1.22+** (the project is built with 1.26)
- Network access to `https://content.luanti.org` for package commands
- A Luanti server with its data directory accessible on the local filesystem

---

## Install

### Build from source

```sh
git clone https://github.com/brylie/luctl
cd luctl
go build -o luctl .
mv luctl /usr/local/bin/
```

### Cross-platform builds

No CGo, no runtime dependencies — cross-compile a single static binary for any platform:

| Platform              | Command                                                            |
| --------------------- | ------------------------------------------------------------------ |
| Linux (amd64)         | `GOOS=linux   GOARCH=amd64  go build -o luctl-linux-amd64 .`       |
| Linux (arm64)         | `GOOS=linux   GOARCH=arm64  go build -o luctl-linux-arm64 .`       |
| macOS (Apple Silicon) | `GOOS=darwin  GOARCH=arm64  go build -o luctl-darwin-arm64 .`      |
| macOS (Intel)         | `GOOS=darwin  GOARCH=amd64  go build -o luctl-darwin-amd64 .`      |
| Windows (amd64)       | `GOOS=windows GOARCH=amd64  go build -o luctl-windows-amd64.exe .` |

---

## Quick start

```sh
cd /opt/luanti                              # directory containing your server data
luctl project init                          # scaffold luanti.toml with sensible defaults
luctl package search farming               # search ContentDB
luctl package install TenPlus1/farming    # install, enable in world.mt, record in luanti.toml
luctl project sync                         # apply [config] values to minetest.conf
```

---

## Command reference

Run `luctl --help` for the full reference, or `luctl <command> --help` for any subcommand.

### `luctl package` — package management

| Command | Description |
| ------- | ----------- |
| `luctl package search <query>` | Search ContentDB by keyword; accepts `--type` and `--limit` |
| `luctl package info <author/name>` | Show metadata for a package |
| `luctl package install <author/name>` | Download, extract, enable in `world.mt`, and append to `luanti.toml` |
| `luctl package update <author/name>` | Re-download a package to its latest ContentDB release |
| `luctl package list` | List all installed mod directories |
| `luctl package enable <name>` | Set `load_mod_<name> = true` in `world.mt` |
| `luctl package disable <name>` | Set `load_mod_<name> = false` in `world.mt` |

### `luctl project` — manifest management

| Command | Description |
| ------- | ----------- |
| `luctl project init` | Scaffold `luanti.toml` with sensible defaults |
| `luctl project install` | Install every package declared in `luanti.toml` |
| `luctl project status` | Show which declared packages are installed or missing |
| `luctl project sync` | Apply `[config]` values to `minetest.conf` in-place |
| `luctl project fmt` | Sort and normalise the manifest (mod and game lists alphabetically) |

### `luctl server` — backup and restore

| Command | Description |
| ------- | ----------- |
| `luctl server backup create` | Archive world dir + `minetest.conf` and upload to S3 |
| `luctl server backup list` | List available backups with name, size, and modification date |
| `luctl server restore [backup-name]` | Download and extract a backup; uses the most recent if omitted |
| `luctl server restore --force [backup-name]` | Same, but skip the overwrite confirmation prompt |

---

## Configuration

### `luanti.toml` project manifest

Run `luctl project init` in your server directory to create a manifest:

```toml
[server]
  name = "My Server"
  admins = ["alice"]

[paths]
  mods_dir  = "./data/mods"
  games_dir = "./data/games"
  world_dir = "./data/worlds/world"
  conf_file = "./data/main-config/minetest.conf"

[config]
  # Any minetest.conf key can go here; applied by `luctl project sync`
  enable_damage   = true
  creative_mode   = false
  server_announce = false

[packages]
  mods  = ["TenPlus1/farming", "sfan5/worldedit"]
  games = ["Luanti/minetest_game"]
```

### `[backup]` section

Add a `[backup]` section to configure S3-compatible storage for `luctl server` commands:

```toml
[backup]
  bucket   = "my-luanti-backups"
  endpoint = "https://nyc3.digitaloceanspaces.com"   # region endpoint — NOT the bucket-specific URL
  region   = "nyc3"
  prefix   = "luanti/"
```

> **DigitalOcean Spaces:** use the *region* endpoint (`https://REGION.digitaloceanspaces.com`),
> **not** the bucket-specific URL (`https://BUCKET.REGION.digitaloceanspaces.com`).
> Path-style access combined with the bucket-specific URL produces a doubled path that breaks listing.

---

## Server backups

`luctl server backup create` archives the world directory and `minetest.conf` as a
timestamped `tar.gz` and uploads it to any S3-compatible provider (DigitalOcean Spaces,
Backblaze B2, MinIO, AWS S3, …).

Backups are safe to run against a live server. SQLite world databases (`map.sqlite`,
`players.sqlite`, etc.) are snapshotted via SQLite's `VACUUM INTO`, which acquires only a
shared read lock and produces a consistent point-in-time copy. SQLite auxiliary files
(`-journal`, `-wal`, `-shm`) are excluded automatically.

### Credentials

Never pass credentials on the command line — they appear in `ps aux` and shell history.
Store them in a file readable only by the service account:

```sh
sudo install -d -m 700 /etc/luctl
sudo install -m 600 /dev/null /etc/luctl/credentials
sudo nano /etc/luctl/credentials
```

`/etc/luctl/credentials`:

```sh
LUCTL_S3_ACCESS_KEY=your-access-key
LUCTL_S3_SECRET_KEY=your-secret-key
```

See `.env.example` for additional credential-loading patterns (interactive `read -s`, direnv).

### Restoring a backup

```sh
luctl server backup list                          # see what's available
luctl server restore                              # restore most recent (prompts for confirmation)
luctl server restore backup-2026-01-15.tar.gz    # restore a specific backup
luctl server restore --force                      # skip confirmation (for scripts/cron)
```

`luctl server restore` warns before overwriting the world directory and requires an
explicit `y` to proceed. Pass `--force` to skip the prompt in automated contexts.

### Scheduled backups — systemd timer

Create `/etc/systemd/system/luctl-backup.service`:

```ini
[Unit]
Description=Luanti server backup
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
User=luanti
WorkingDirectory=/opt/luanti
ExecStart=/usr/local/bin/luctl server backup create
EnvironmentFile=/etc/luctl/credentials
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ReadWritePaths=/opt/luanti
```

Create `/etc/systemd/system/luctl-backup.timer`:

```ini
[Unit]
Description=Daily Luanti server backup

[Timer]
OnCalendar=daily
RandomizedDelaySec=30min
Persistent=true

[Install]
WantedBy=timers.target
```

Enable and verify:

```sh
systemctl daemon-reload
systemctl enable --now luctl-backup.timer
systemctl list-timers luctl-backup.timer
systemctl start luctl-backup.service   # run once to test
journalctl -u luctl-backup.service -n 20
```

### Scheduled backups — cron

Wrap the command in a script so credentials are never visible on the command line:

```sh
#!/bin/sh
. /etc/luctl/credentials
exec /usr/local/bin/luctl server backup create
```

Save as `/usr/local/bin/luctl-backup`, then:

```sh
sudo chmod 700 /usr/local/bin/luctl-backup
sudo chown root:root /usr/local/bin/luctl-backup
```

Add to the crontab of the user that owns the server directory:

```cron
0 2 * * * /usr/local/bin/luctl-backup >> /var/log/luctl-backup.log 2>&1
```

---

## Shell autocompletion

```sh
# zsh
luctl completion zsh > "${fpath[1]}/_luctl"

# bash
luctl completion bash > /etc/bash_completion.d/luctl

# fish
luctl completion fish > ~/.config/fish/completions/luctl.fish
```

---

## Development

### Setup

Install [mise](https://mise.jdx.dev/getting-started.html), then run:

```sh
mise install                              # installs Go, golangci-lint, markdownlint-cli2, prek, …
prek install                              # register the pre-commit hook
prek install --hook-type pre-push         # register the pre-push coverage gate
```

All tools are pinned in `mise.toml` and installed locally to the project.

### Day-to-day commands

```sh
go run . package search mobs              # run without building
go test ./...                             # run tests
mise exec -- golangci-lint run ./...      # lint Go
mise exec -- markdownlint-cli2 "**/*.md"  # lint markdown
mise exec -- prek run --all-files         # run all hooks manually
```

### Hooks

Pre-commit (every `git commit`):

- **golangci-lint** — full lint suite across all Go packages
- **markdownlint-cli2** — style checks on staged `.md` files

Pre-push (every `git push`):

- **coverage** — runs the full test suite; fails if total coverage drops below 80%

Linter config: `.golangci.yml` (errcheck, staticcheck, gosec, noctx, bodyclose, revive).
Markdown config: `.markdownlint.json`.
