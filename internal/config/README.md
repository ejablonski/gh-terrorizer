# config

## Overview
Package `config` provides TOML configuration parsing, path expansion, environment variable overrides, production defaults, and Git repository remote detection for `gh-terrorizer`.

## Interface
- `Config`: Root configuration struct defining `DefaultOwner`, `DefaultRepo`, `ProjectNumber`, `Timewarrior`, and `Theme`.
- `TimewarriorConfig`: Sub-configuration struct controlling `Enabled`, `HoursCustomField`, `RoundingPrecision`, and `TagPrefix`.
- `ThemeConfig`: Styling configuration struct specifying `TokyoNightVariant` ("storm" or "night") and `ShowIcons`.
- `DefaultConfig() *Config`: Returns a `Config` initialized with production defaults.
- `ExpandPath(path string) (string, error)`: Expands leading tilde (`~`) or `~/` to the current user's home directory.
- `DefaultConfigPath() (string, error)`: Resolves the default configuration file path (`~/.config/gh-terrorizer/config.toml`).
- `LoadConfig(path string) (*Config, error)`: Parses the specified or discovered TOML config file, falling back to defaults if absent.
- `DetectGitRemote() (owner, repo string, err error)`: Executes `git remote get-url origin` to extract GitHub owner and repository names.

## Invariants
- If `Timewarrior.HoursCustomField` is empty, it is forced to `"Hours Spent"`.
- If `Timewarrior.RoundingPrecision` is negative, it is normalized to `2`.
- If `Theme.TokyoNightVariant` is empty, it is normalized to `"storm"`.
- If the configuration file does not exist, `LoadConfig` returns `DefaultConfig()` without returning an error.

## Side Effects
- Reads filesystem files via `os.ReadFile` (`config.toml`).
- Inspects user environment variables (`GH_TERRORIZER_CONFIG`).
- Spawns a `git remote get-url origin` subprocess to inspect Git configuration.

## Verification
- Run: `go test ./internal/config`
