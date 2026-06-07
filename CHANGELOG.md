# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
