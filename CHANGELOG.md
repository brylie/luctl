# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

#### Server backup and restore (`luctl server`)

- `luctl server backup create` — archives the world directory and `minetest.conf` as a
  timestamped tar.gz and uploads it to any S3-compatible provider (DigitalOcean Spaces,
  Backblaze B2, MinIO, AWS S3, …).
- `luctl server backup list` — lists available backups in the configured bucket with
  name, size, and modification timestamp.
- `luctl server restore [backup-name]` — downloads and extracts a backup into the world
  directory; uses the most recent backup when no name is given.
- **Live-safe SQLite backup** — world databases are snapshotted via SQLite's `VACUUM INTO`,
  which holds only a shared read lock. Backups can run against a live server without
  downtime or data inconsistency. SQLite auxiliary files (`-journal`, `-wal`, `-shm`) are
  excluded from archives automatically.
- `[backup]` section in `luanti.toml` — configures bucket, region endpoint, and key
  prefix. Credentials (`LUCTL_S3_ACCESS_KEY` / `LUCTL_S3_SECRET_KEY`) are read from
  environment variables only and are never written to disk.
- `.env.example` — documents three OWASP-safe credential-loading patterns (`source`,
  `read -rs`, direnv) that keep secrets out of shell history and `ps aux`.
- Scheduled backup guidance in `README.md` — systemd timer unit with `EnvironmentFile=`
  hardening (`NoNewPrivileges`, `ProtectSystem=strict`) and a cron wrapper-script approach.

## [0.1.0] - 2026-06-07

Initial release.

### Added

#### Package management (`luctl package`)

- Search ContentDB for mods, games, and texture packs by keyword, with optional
  type and result-limit filters.
- Fetch detailed metadata for any ContentDB package by `author/name`.
- Install a package directly into a server's mods or games directory, with
  decompression-bomb protection and a 500 MB read limit.
- List all currently installed mod directories.
- Update (re-download) an installed package to its latest ContentDB release.
- Enable or disable a mod in `world.mt` by name (`luctl package enable`,
  `luctl package disable`), with in-place key update — existing lines are
  rewritten, new keys are appended.

#### Project manifest (`luctl project`)

- `luanti.toml` — a declarative project manifest inspired by `pyproject.toml`
  and `package.json`, tracking server identity, filesystem paths, gameplay
  configuration, and package dependencies.
- `luctl project init` — scaffolds a `luanti.toml` with sensible defaults.
- `luctl project install` — installs every declared package from the manifest,
  placing mods and games in the correct directories and enabling each mod in
  `world.mt` automatically.
- `luctl project status` — shows which declared packages are installed or missing.
- `luctl project fmt` — sorts and normalises `luanti.toml` in-place (mod and
  game lists sorted alphabetically).
- `luctl project sync` — applies every key in the `[config]` table to the
  server's `minetest.conf`, updating existing lines in-place without touching
  comments or unrelated settings.

#### Auto-save and auto-enable on install

- `luctl package install` in a project directory automatically appends the
  package to `luanti.toml` and enables it in `world.mt`. Pass `--no-save` to
  skip the manifest update or `--no-enable` to skip `world.mt`.

#### `luanti.toml` format

- `[server]` — name, description, port, admins.
- `[world]` — game id, mapgen.
- `[paths]` — `mods_dir`, `games_dir`, `world_dir`, `conf_file`.
- `[config]` — arbitrary `minetest.conf` key/value pairs applied by
  `luctl project sync`.
- `[packages]` — compact `mods = [...]` and `games = [...]` string lists
  (replaces the verbose `[[packages]]` array-of-tables format).
