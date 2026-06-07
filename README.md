# luctl

A command-line tool for managing a self-hosted [Luanti](https://www.luanti.org/) server.
It wraps the [ContentDB REST API](https://content.luanti.org/help/api/) and introduces
a declarative `luanti.toml` project manifest — similar to `package.json` or `pyproject.toml`
— so your server configuration, mod list, and world settings are all version-controlled
in one place.

---

## Why

The Luanti client ships a built-in Content browser, but it installs mods into the
*client's* local directory — not a remote server. When running a self-hosted server,
managing packages means dropping files into the server's `mods/` directory by hand,
then remembering to enable them in `minetest.conf` and `world.mt`.

`luctl` automates the entire workflow:

- **Package management** — search ContentDB, install, update, and list mods and games.
  Installing a mod in a project directory automatically enables it in `world.mt` and
  records it in `luanti.toml`.

- **Project manifest (`luanti.toml`)** — declares your server identity, mod list,
  filesystem paths, and arbitrary `minetest.conf` key/value pairs. Commit it to
  version control so the server is fully reproducible from a single file.

- **Config sync** — push `[config]` values from `luanti.toml` into `minetest.conf`
  with one command, updating existing keys in-place without touching your comments.

- **Mod enable/disable** — toggle mods in `world.mt` by name, without opening any file.

---

## Requirements

- **Go 1.22+** (the project is built with 1.26)
- Network access to `https://content.luanti.org`
- A Luanti server with its data directory accessible on the local filesystem

---

## Install

### Build from source

```sh
git clone https://github.com/brylie/luctl
cd luctl/cli
go build -o luctl .
```

Move the binary somewhere on your `$PATH`:

```sh
mv luctl /usr/local/bin/
```

### Cross-platform releases

Use `GOOS` and `GOARCH` to cross-compile a single static binary for any platform.
No CGo, no runtime dependencies.

| Platform              | Command                                                            |
| --------------------- | ------------------------------------------------------------------ |
| Linux (amd64)         | `GOOS=linux   GOARCH=amd64  go build -o luctl-linux-amd64 .`       |
| Linux (arm64)         | `GOOS=linux   GOARCH=arm64  go build -o luctl-linux-arm64 .`       |
| macOS (Apple Silicon) | `GOOS=darwin  GOARCH=arm64  go build -o luctl-darwin-arm64 .`      |
| macOS (Intel)         | `GOOS=darwin  GOARCH=amd64  go build -o luctl-darwin-amd64 .`      |
| Windows (amd64)       | `GOOS=windows GOARCH=amd64  go build -o luctl-windows-amd64.exe .` |

---

## Usage

Run `luctl --help` for the full command reference, or `luctl <command> --help` for
details on any subcommand. The two top-level namespaces are:

- **`luctl package`** — search, install, update, list, enable, and disable packages
- **`luctl project`** — init, install, status, sync, fmt a `luanti.toml` manifest

### The `luanti.toml` project file

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
  # Any minetest.conf key can go here
  enable_damage  = true
  creative_mode  = false
  server_announce = false

[packages]
  mods  = ["TenPlus1/farming", "sfan5/worldedit"]
  games = ["Luanti/minetest_game"]
```

Once the manifest exists:

- `luctl package install <author/name>` installs to the correct directory, enables
  the mod in `world.mt`, and appends it to `[packages]` — all in one step.
- `luctl project install` reproduces the full package list on a fresh machine.
- `luctl project sync` pushes `[config]` values into `minetest.conf`.
- `luctl project fmt` sorts and normalises the manifest in-place.

---

## Server backups

`luctl server backup create` uploads a timestamped tar.gz of your world directory and
`minetest.conf` to any S3-compatible provider (DigitalOcean Spaces, Backblaze B2, MinIO,
AWS S3, …).

Backups are safe to run against a live server. SQLite world databases (`map.sqlite`,
`players.sqlite`, etc.) are snapshotted with SQLite's `VACUUM INTO` command, which
acquires only a shared read lock and produces a consistent point-in-time copy. SQLite
auxiliary files (`-journal`, `-wal`, `-shm`) are excluded automatically.

Add a `[backup]` section to your `luanti.toml`:

```toml
[backup]
  bucket   = "my-luanti-backups"
  endpoint = "https://nyc3.digitaloceanspaces.com"   # region endpoint — NOT the bucket-specific URL
  region   = "nyc3"
  prefix   = "luanti/"
```

> **DigitalOcean Spaces:** use the *region* endpoint (`https://REGION.digitaloceanspaces.com`),
> **not** the bucket-specific URL (`https://BUCKET.REGION.digitaloceanspaces.com`).
> The bucket-specific URL combined with path-style access (which `luctl` uses for provider
> compatibility) causes a doubled path and breaks listing.

### Credentials

Never pass credentials on the command line — they appear in `ps aux` and shell history.
Store them in a file readable only by the service account:

```sh
sudo install -d -m 700 /etc/luctl
sudo install -m 600 /dev/null /etc/luctl/credentials
sudo nano /etc/luctl/credentials   # fill in the two lines below, then save
```

`/etc/luctl/credentials`:

```sh
LUCTL_S3_ACCESS_KEY=your-access-key
LUCTL_S3_SECRET_KEY=your-secret-key
```

See `.env.example` for additional safe options (interactive `read -s`, direnv).

### Scheduled backups — systemd timer (Ubuntu / Debian / Arch / Fedora)

Create the service unit `/etc/systemd/system/luctl-backup.service`:

```ini
[Unit]
Description=Luanti server backup
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
User=luanti
WorkingDirectory=/opt/luanti          # directory containing luanti.toml
ExecStart=/usr/local/bin/luctl server backup create
EnvironmentFile=/etc/luctl/credentials

# Basic hardening
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ReadWritePaths=/opt/luanti
```

> Replace `User=luanti` and `WorkingDirectory=` with the account and directory that
> own your server files. `EnvironmentFile=` loads credentials without exposing them in
> the process list or the journal.

Create the timer unit `/etc/systemd/system/luctl-backup.timer`:

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

Enable and start:

```sh
systemctl daemon-reload
systemctl enable --now luctl-backup.timer

# Verify the timer is scheduled
systemctl list-timers luctl-backup.timer

# Run a backup immediately to test
systemctl start luctl-backup.service

# Check the result
journalctl -u luctl-backup.service -n 20
```

### Scheduled backups — cron (alternative)

If you prefer cron, wrap the command in a script so the credentials are never on the
cron command line (which is visible in `ps aux` during execution).

Create `/usr/local/bin/luctl-backup` (owned by root, executable only by the service user):

```sh
#!/bin/sh
# Load credentials from a protected file; they will not appear in the process list.
. /etc/luctl/credentials
exec /usr/local/bin/luctl server backup create
```

```sh
sudo chmod 700 /usr/local/bin/luctl-backup
sudo chown root:root /usr/local/bin/luctl-backup
```

Add to the crontab of the user that owns the server directory (`crontab -e`):

```cron
# Daily backup at 02:00, log to file
0 2 * * * /usr/local/bin/luctl-backup >> /var/log/luctl-backup.log 2>&1
```

---

## Shell autocompletion

Cobra generates autocompletion scripts for bash, zsh, fish, and PowerShell:

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
prek install --hook-type pre-push         # register the pre-push hook (coverage gate)
```

That's it — all tools are pinned in `mise.toml` and installed locally to the project.

### Day-to-day commands

```sh
# Run without building
go run . package search mobs

# Run tests
go test ./...

# Lint (requires golangci-lint via mise)
mise exec -- golangci-lint run ./...

# Lint markdown
mise exec -- markdownlint-cli2 "**/*.md"
```

The linter config is in `.golangci.yml`. It enforces correctness rules (errcheck,
staticcheck, gosec), HTTP safety (noctx, bodyclose), and style (revive, nlreturn,
godot). Markdown rules are in `.markdownlint.json`.

### Pre-commit hooks

Install the hooks once after cloning:

```sh
mise install                              # install all tools (go, golangci-lint, markdownlint-cli2, prek, …)
prek install                              # register the pre-commit hook
prek install --hook-type pre-push         # register the pre-push hook (coverage gate)
```

Pre-commit hooks run on every `git commit`:

- **golangci-lint** — full lint suite on all Go packages (`./...`)
- **markdownlint-cli2** — markdown style checks on staged `.md` files

The pre-push hook runs on every `git push`:

- **coverage** — runs the full test suite and fails if total coverage drops below 80%
  (`go run ./scripts/check_coverage.go`; uses only Go, works on all platforms)

Run all hooks manually at any time:

```sh
mise exec -- prek run --all-files
```
