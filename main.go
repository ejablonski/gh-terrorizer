package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ejablonski/gh-terrorizer/internal/config"
	"github.com/ejablonski/gh-terrorizer/internal/github"
	"github.com/ejablonski/gh-terrorizer/internal/timew"
	"github.com/ejablonski/gh-terrorizer/internal/tui"
)

var version = "0.1.0"

func main() {
	ownerFlag := flag.String("owner", "", "GitHub organization or user owning the project")
	repoFlag := flag.String("repo", "", "GitHub repository name (optional, used for git remote inference)")
	projectFlag := flag.Int("project", 0, "GitHub Projects v2 number")
	configFlag := flag.String("config", "", "Path to configuration file")
	versionFlag := flag.Bool("version", false, "Print version and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: gh-terrorizer [flags]\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *versionFlag {
		fmt.Printf("gh-terrorizer v%s\n", version)
		os.Exit(0)
	}

	// 1. Load configuration
	cfg, err := config.LoadConfig(*configFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load configuration: %v\n", err)
		cfg = config.DefaultConfig()
	}

	// 2. Resolve Owner and Repo
	if *ownerFlag != "" {
		cfg.DefaultOwner = *ownerFlag
	}
	if *repoFlag != "" {
		cfg.DefaultRepo = *repoFlag
	}
	if *projectFlag > 0 {
		cfg.ProjectNumber = *projectFlag
	}

	// Auto-detect owner & repo from git remote if omitted
	if cfg.DefaultOwner == "" || cfg.DefaultRepo == "" {
		detectedOwner, detectedRepo, err := config.DetectGitRemote()
		if err == nil {
			if cfg.DefaultOwner == "" {
				cfg.DefaultOwner = detectedOwner
			}
			if cfg.DefaultRepo == "" {
				cfg.DefaultRepo = detectedRepo
			}
		}
	}

	if cfg.DefaultOwner == "" {
		fmt.Fprintf(os.Stderr, "Error: missing required owner. Specify via --owner or in config.toml.\n")
		flag.Usage()
		os.Exit(1)
	}

	if cfg.ProjectNumber <= 0 {
		fmt.Fprintf(os.Stderr, "Error: missing required project number. Specify via --project <number> or in config.toml.\n")
		flag.Usage()
		os.Exit(1)
	}

	// 3. Pre-flight health checks
	// 3a. Check timew binary
	if _, err := exec.LookPath("timew"); err != nil {
		if cfg.Timewarrior.Enabled {
			fmt.Fprintf(os.Stderr, "Notice: 'timew' binary not found in PATH. Time tracking disabled.\n")
			cfg.Timewarrior.Enabled = false
		}
	}

	// 3b. Check GitHub CLI OAuth scopes
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := github.CheckProjectScope(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// 4. Initialize adapters
	ghClient, err := github.NewClient(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing GitHub client: %v\n", err)
		os.Exit(1)
	}

	timeTracker := timew.NewTracker(nil)

	// 5. Initialize Bubble Tea TUI
	model := tui.NewModel(ghClient, timeTracker, cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Application error: %v\n", err)
		os.Exit(1)
	}
}
