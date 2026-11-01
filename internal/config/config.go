package config

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Config defines the top-level application configuration.
type Config struct {
	DefaultOwner  string            `toml:"default_owner"`
	DefaultRepo   string            `toml:"default_repo"`
	ProjectNumber int               `toml:"project_number"`
	Timewarrior   TimewarriorConfig `toml:"timewarrior"`
	Theme         ThemeConfig       `toml:"theme"`
}

// TimewarriorConfig configures integration with the timew CLI.
type TimewarriorConfig struct {
	Enabled           bool   `toml:"enabled"`
	HoursCustomField  string `toml:"hours_custom_field"`
	RoundingPrecision int    `toml:"rounding_precision"`
	TagPrefix         string `toml:"tag_prefix"`
}

// ThemeConfig configures visual theming and iconography.
type ThemeConfig struct {
	TokyoNightVariant string            `toml:"variant"`
	ShowIcons         bool              `toml:"show_icons"`
	IssueTypeLabels   map[string]string `toml:"issue_type_labels"`
}

// DefaultConfig returns a configuration populated with production defaults.
func DefaultConfig() *Config {
	return &Config{
		DefaultOwner:  "",
		DefaultRepo:   "",
		ProjectNumber: 0,
		Timewarrior: TimewarriorConfig{
			Enabled:           true,
			HoursCustomField:  "Hours Spent",
			RoundingPrecision: 2,
			TagPrefix:         "gh-",
		},
		Theme: ThemeConfig{
			TokyoNightVariant: "storm",
			ShowIcons:         true,
			IssueTypeLabels:   make(map[string]string),
		},
	}
}

// ExpandPath expands ~ to the user's home directory.
func ExpandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

// DefaultConfigPath returns ~/.config/gh-terrorizer/config.toml.
func DefaultConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		home, hErr := os.UserHomeDir()
		if hErr != nil {
			return "", err
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "gh-terrorizer", "config.toml"), nil
}

// LoadConfig loads configuration from path, falling back to defaults.
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path == "" {
		if envPath := os.Getenv("GH_TERRORIZER_CONFIG"); envPath != "" {
			path = envPath
		} else {
			defaultPath, err := DefaultConfigPath()
			if err == nil {
				path = defaultPath
			}
		}
	}

	if path != "" {
		expanded, err := ExpandPath(path)
		if err == nil {
			path = expanded
		}

		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// No config file present; return defaults without error.
				return cfg, nil
			}
			return nil, err
		}

		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	// Validate & sanitize bounds
	if cfg.Timewarrior.HoursCustomField == "" {
		cfg.Timewarrior.HoursCustomField = "Hours Spent"
	}
	if cfg.Timewarrior.RoundingPrecision < 0 {
		cfg.Timewarrior.RoundingPrecision = 2
	}
	if cfg.Theme.TokyoNightVariant == "" {
		cfg.Theme.TokyoNightVariant = "storm"
	}
	if cfg.Theme.IssueTypeLabels != nil {
		normalized := make(map[string]string, len(cfg.Theme.IssueTypeLabels))
		for k, v := range cfg.Theme.IssueTypeLabels {
			normalized[strings.ToLower(k)] = v
		}
		cfg.Theme.IssueTypeLabels = normalized
	} else {
		cfg.Theme.IssueTypeLabels = make(map[string]string)
	}

	return cfg, nil
}

// DetectGitRemote attempts to parse the owner and repo from `git remote get-url origin`.
func DetectGitRemote() (owner, repo string, err error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", "", err
	}
	urlStr := strings.TrimSpace(string(out))

	// Match git@github.com:owner/repo.git or https://github.com/owner/repo(.git)
	reSSH := regexp.MustCompile(`github\.com[:/]([^/]+)/([^/\.]+)(?:\.git)?`)
	matches := reSSH.FindStringSubmatch(urlStr)
	if len(matches) == 3 {
		return matches[1], matches[2], nil
	}

	return "", "", errors.New("could not parse GitHub owner/repo from remote URL")
}
