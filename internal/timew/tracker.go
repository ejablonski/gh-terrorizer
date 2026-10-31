package timew

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"strings"
	"time"
)

// TimewTimeFormat is the compact UTC format used by timewarrior export (e.g. 20261029T152321Z).
const TimewTimeFormat = "20060102T150405Z"

// ExportInterval represents a recorded or active interval from `timew export`.
type ExportInterval struct {
	ID         int      `json:"id"`
	Start      string   `json:"start"`
	End        string   `json:"end,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Annotation string   `json:"annotation,omitempty"`
}

// CommandRunner is a function type for running CLI commands, enabling unit testing with mocks.
type CommandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

// DefaultCommandRunner executes commands using os/exec.
func DefaultCommandRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}

// Tracker provides time tracking operations over the timew CLI.
type Tracker struct {
	runner CommandRunner
}

// NewTracker creates a new Timewarrior tracker instance.
func NewTracker(runner CommandRunner) *Tracker {
	if runner == nil {
		runner = DefaultCommandRunner
	}
	return &Tracker{runner: runner}
}

// IsActive checks if timewarrior currently has an active tracking session.
func (t *Tracker) IsActive(ctx context.Context) (bool, error) {
	out, err := t.runner(ctx, "timew", "get", "dom.active")
	if err != nil {
		return false, err
	}
	val := strings.TrimSpace(string(out))
	return val == "1", nil
}

// GetActiveTag returns the primary tag of the currently active session, or empty if none.
func (t *Tracker) GetActiveTag(ctx context.Context) (string, error) {
	active, err := t.IsActive(ctx)
	if err != nil || !active {
		return "", err
	}

	out, err := t.runner(ctx, "timew", "get", "dom.active.tag.1")
	if err == nil {
		tag := strings.TrimSpace(string(out))
		if tag != "" && !strings.Contains(tag, "not valid") {
			return tag, nil
		}
	}

	// Fallback to inspecting timew export
	intervals, err := t.Export(ctx, "")
	if err != nil {
		return "", err
	}
	for _, interval := range intervals {
		if interval.End == "" && len(interval.Tags) > 0 {
			return interval.Tags[0], nil
		}
	}

	return "", nil
}

// Start begins tracking time for a given tag and optional annotations/tags.
func (t *Tracker) Start(ctx context.Context, tag string, extra ...string) error {
	args := []string{"start", tag}
	if len(extra) > 0 {
		args = append(args, extra...)
	}
	_, err := t.runner(ctx, "timew", args...)
	return err
}

// Stop stops the currently running tracking session and returns the elapsed duration in seconds.
func (t *Tracker) Stop(ctx context.Context) (int, error) {
	active, err := t.IsActive(ctx)
	if err != nil {
		return 0, err
	}
	if !active {
		return 0, nil
	}

	// Stop tracking
	if _, err := t.runner(ctx, "timew", "stop"); err != nil {
		return 0, fmt.Errorf("failed to stop timew: %w", err)
	}

	// Read the most recently closed interval (@1)
	out, err := t.runner(ctx, "timew", "export", "@1")
	if err != nil {
		return 0, fmt.Errorf("failed to export stopped interval: %w", err)
	}

	var intervals []ExportInterval
	if err := json.Unmarshal(out, &intervals); err != nil {
		return 0, fmt.Errorf("failed to parse timew export JSON: %w", err)
	}

	if len(intervals) == 0 {
		return 0, errors.New("no interval returned after timew stop")
	}

	recent := intervals[0]
	if recent.Start == "" || recent.End == "" {
		return 0, errors.New("stopped interval has missing start or end timestamp")
	}

	startT, err := time.Parse(TimewTimeFormat, recent.Start)
	if err != nil {
		return 0, fmt.Errorf("failed to parse interval start time: %w", err)
	}

	endT, err := time.Parse(TimewTimeFormat, recent.End)
	if err != nil {
		return 0, fmt.Errorf("failed to parse interval end time: %w", err)
	}

	duration := endT.Sub(startT)
	if duration < 0 {
		duration = 0
	}

	return int(duration.Seconds()), nil
}

// Export runs `timew export` with optional filters and unmarshals the JSON array.
func (t *Tracker) Export(ctx context.Context, filter string) ([]ExportInterval, error) {
	args := []string{"export"}
	if filter != "" {
		args = append(args, filter)
	}
	out, err := t.runner(ctx, "timew", args...)
	if err != nil {
		return nil, err
	}

	var intervals []ExportInterval
	if err := json.Unmarshal(out, &intervals); err != nil {
		return nil, err
	}
	return intervals, nil
}

// CalculateHours converts elapsed seconds into decimal hours rounded to precision decimal places.
func CalculateHours(elapsedSeconds int, precision int) float64 {
	if elapsedSeconds <= 0 {
		return 0.0
	}
	if precision < 0 {
		precision = 2
	}
	deltaH := float64(elapsedSeconds) / 3600.0
	factor := math.Pow10(precision)
	return math.Round(deltaH*factor) / factor
}

// FormatHours formats decimal hours for display in the UI (e.g. 0.50 -> "30m", 1.5 -> "1h 30m").
func FormatHours(hours float64) string {
	if hours <= 0 {
		return "0m"
	}
	totalMinutes := int(math.Round(hours * 60.0))
	h := totalMinutes / 60
	m := totalMinutes % 60

	if h > 0 && m > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	} else if h > 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dm", m)
}
