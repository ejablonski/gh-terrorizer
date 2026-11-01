package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("expected non-nil default config")
	}
	if cfg.Timewarrior.HoursCustomField != "Hours Spent" {
		t.Errorf("expected 'Hours Spent', got '%s'", cfg.Timewarrior.HoursCustomField)
	}
	if cfg.Timewarrior.RoundingPrecision != 2 {
		t.Errorf("expected 2, got %d", cfg.Timewarrior.RoundingPrecision)
	}
	if !cfg.Timewarrior.Enabled {
		t.Error("expected timewarrior enabled by default")
	}
	if cfg.Theme.TokyoNightVariant != "storm" {
		t.Errorf("expected 'storm', got '%s'", cfg.Theme.TokyoNightVariant)
	}
	if !cfg.Theme.ShowIcons {
		t.Error("expected icons enabled by default")
	}
}

func TestLoadConfig_NonExistent(t *testing.T) {
	cfg, err := LoadConfig("/non/existent/path/config.toml")
	if err != nil {
		t.Fatalf("expected nil error on missing config, got %v", err)
	}
	if cfg == nil {
		t.Fatal("expected default config returned")
	}
	if cfg.Timewarrior.HoursCustomField != "Hours Spent" {
		t.Errorf("expected default field, got '%s'", cfg.Timewarrior.HoursCustomField)
	}
}

func TestLoadConfig_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := `
default_owner = "myorg"
default_repo = "myrepo"
project_number = 42

[timewarrior]
enabled = false
hours_custom_field = "Time Tracked"
rounding_precision = 3
tag_prefix = "task-"

[theme]
variant = "night"
show_icons = false
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.DefaultOwner != "myorg" {
		t.Errorf("expected 'myorg', got '%s'", cfg.DefaultOwner)
	}
	if cfg.DefaultRepo != "myrepo" {
		t.Errorf("expected 'myrepo', got '%s'", cfg.DefaultRepo)
	}
	if cfg.ProjectNumber != 42 {
		t.Errorf("expected 42, got %d", cfg.ProjectNumber)
	}
	if cfg.Timewarrior.Enabled {
		t.Error("expected timewarrior disabled")
	}
	if cfg.Timewarrior.HoursCustomField != "Time Tracked" {
		t.Errorf("expected 'Time Tracked', got '%s'", cfg.Timewarrior.HoursCustomField)
	}
	if cfg.Timewarrior.RoundingPrecision != 3 {
		t.Errorf("expected 3, got %d", cfg.Timewarrior.RoundingPrecision)
	}
	if cfg.Theme.TokyoNightVariant != "night" {
		t.Errorf("expected 'night', got '%s'", cfg.Theme.TokyoNightVariant)
	}
	if cfg.Theme.ShowIcons {
		t.Error("expected icons disabled")
	}
}

func TestLoadConfig_IssueTypeLabels(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := `
[theme]
variant = "storm"
show_icons = true

[theme.issue_type_labels]
"Bug" = "#f7768e"
"FEATURE" = "#7aa2f7"
"task" = "#e0af68"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(cfg.Theme.IssueTypeLabels) != 3 {
		t.Fatalf("expected 3 issue type labels, got %d", len(cfg.Theme.IssueTypeLabels))
	}
	if hex, ok := cfg.Theme.IssueTypeLabels["bug"]; !ok || hex != "#f7768e" {
		t.Errorf("expected bug -> #f7768e, got %s (ok=%v)", hex, ok)
	}
	if hex, ok := cfg.Theme.IssueTypeLabels["feature"]; !ok || hex != "#7aa2f7" {
		t.Errorf("expected feature -> #7aa2f7, got %s (ok=%v)", hex, ok)
	}
	if hex, ok := cfg.Theme.IssueTypeLabels["task"]; !ok || hex != "#e0af68" {
		t.Errorf("expected task -> #e0af68, got %s (ok=%v)", hex, ok)
	}
}

func TestExpandPath(t *testing.T) {
	expanded, err := ExpandPath("~/test/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, "test/path")
	if expanded != expected {
		t.Errorf("expected '%s', got '%s'", expected, expanded)
	}

	plain, err := ExpandPath("/absolute/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plain != "/absolute/path" {
		t.Errorf("expected unchanged path, got '%s'", plain)
	}
}

func TestDetectGitRemote(t *testing.T) {
	// Should detect origin for this git repository
	owner, repo, err := DetectGitRemote()
	if err != nil {
		t.Logf("git remote detection returned err (might be CI/offline): %v", err)
		return
	}
	if owner != "ejablonski" {
		t.Errorf("expected owner 'ejablonski', got '%s'", owner)
	}
	if repo != "gh-terrorizer" {
		t.Errorf("expected repo 'gh-terrorizer', got '%s'", repo)
	}
}
