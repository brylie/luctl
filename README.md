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

```sh
# Run without building
go run . package search mobs

# Lint (requires golangci-lint via mise)
mise exec -- golangci-lint run ./...

# Run tests (once added)
go test ./...
```

The linter config is in `.golangci.yml`. It enforces correctness rules (errcheck,
staticcheck, gosec), HTTP safety (noctx, bodyclose), and style (revive, nlreturn,
godot). Run it before every commit.
