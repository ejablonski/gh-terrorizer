package timew

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestCalculateHours(t *testing.T) {
	tests := []struct {
		name      string
		seconds   int
		precision int
		expected  float64
	}{
		{"zero", 0, 2, 0.0},
		{"negative", -50, 2, 0.0},
		{"30 minutes", 1800, 2, 0.50},
		{"15 minutes", 900, 2, 0.25},
		{"1 hour", 3600, 2, 1.00},
		{"20 minutes", 1200, 2, 0.33},
		{"45 seconds with 2 decimals", 45, 2, 0.01},
		{"45 seconds with 3 decimals", 45, 3, 0.013},
		{"3 hours 45 minutes", 13500, 2, 3.75},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CalculateHours(tc.seconds, tc.precision)
			if math.Abs(got-tc.expected) > 1e-9 {
				t.Errorf("CalculateHours(%d, %d) = %f; expected %f", tc.seconds, tc.precision, got, tc.expected)
			}
		})
	}
}

func TestFormatHours(t *testing.T) {
	tests := []struct {
		hours    float64
		expected string
	}{
		{0.0, "0m"},
		{0.5, "30m"},
		{0.25, "15m"},
		{1.0, "1h"},
		{1.5, "1h 30m"},
		{2.75, "2h 45m"},
	}

	for _, tc := range tests {
		got := FormatHours(tc.hours)
		if got != tc.expected {
			t.Errorf("FormatHours(%f) = '%s'; expected '%s'", tc.hours, got, tc.expected)
		}
	}
}

func TestTracker_IsActive(t *testing.T) {
	mockRunner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "timew" && len(args) == 2 && args[0] == "get" && args[1] == "dom.active" {
			return []byte("1\n"), nil
		}
		return nil, errors.New("unexpected command")
	}

	tracker := NewTracker(mockRunner)
	active, err := tracker.IsActive(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !active {
		t.Error("expected active == true")
	}

	inactiveRunner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("0\n"), nil
	}
	inactiveTracker := NewTracker(inactiveRunner)
	active, err = inactiveTracker.IsActive(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if active {
		t.Error("expected active == false")
	}
}

func TestTracker_StartAndStop(t *testing.T) {
	started := false
	stopped := false

	mockRunner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if args[0] == "start" {
			started = true
			return []byte("Tracking started\n"), nil
		}
		if args[0] == "get" && args[1] == "dom.active" {
			return []byte("1\n"), nil
		}
		if args[0] == "stop" {
			stopped = true
			return []byte("Recorded @1 ...\n"), nil
		}
		if args[0] == "export" && len(args) == 2 && args[1] == "@1" {
			// Return a 1800 second interval (30 minutes)
			jsonPayload := `[{"id":1,"start":"20261030T170000Z","end":"20261030T173000Z","tags":["gh-42"]}]`
			return []byte(jsonPayload), nil
		}
		return nil, errors.New("unhandled command")
	}

	tracker := NewTracker(mockRunner)
	ctx := context.Background()

	if err := tracker.Start(ctx, "gh-42", ":gh"); err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	if !started {
		t.Error("expected start to be invoked")
	}

	elapsed, err := tracker.Stop(ctx)
	if err != nil {
		t.Fatalf("failed to stop: %v", err)
	}
	if !stopped {
		t.Error("expected stop to be invoked")
	}
	if elapsed != 1800 {
		t.Errorf("expected 1800 seconds elapsed, got %d", elapsed)
	}

	hours := CalculateHours(elapsed, 2)
	if hours != 0.50 {
		t.Errorf("expected 0.50 hours, got %f", hours)
	}
}


func TestTracker_GetActiveTag(t *testing.T) {
	mockRunner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if args[0] == "get" && args[1] == "dom.active" {
			return []byte("1\n"), nil
		}
		if args[0] == "get" && args[1] == "dom.active.tag.1" {
			return []byte("gh-99\n"), nil
		}
		return nil, errors.New("unhandled")
	}

	tracker := NewTracker(mockRunner)
	tag, err := tracker.GetActiveTag(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag != "gh-99" {
		t.Errorf("expected 'gh-99', got '%s'", tag)
	}

	// Fallback to export
	fallbackRunner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if args[0] == "get" && args[1] == "dom.active" {
			return []byte("1\n"), nil
		}
		if args[0] == "get" && args[1] == "dom.active.tag.1" {
			return []byte("DOM reference 'dom.active.tag.1' is not valid.\n"), nil
		}
		if args[0] == "export" {
			return []byte(`[{"id":1,"start":"20261030T170000Z","tags":["gh-fallback"]}]`), nil
		}
		return nil, errors.New("unhandled")
	}

	fbTracker := NewTracker(fallbackRunner)
	fbTag, err := fbTracker.GetActiveTag(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fbTag != "gh-fallback" {
		t.Errorf("expected 'gh-fallback', got '%s'", fbTag)
	}
}

func TestTracker_StopInactive(t *testing.T) {
	mockRunner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if args[0] == "get" && args[1] == "dom.active" {
			return []byte("0\n"), nil
		}
		return nil, errors.New("should not be called")
	}

	tracker := NewTracker(mockRunner)
	elapsed, err := tracker.Stop(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed != 0 {
		t.Errorf("expected 0 elapsed for inactive stop, got %d", elapsed)
	}
}

func TestTracker_StopErrors(t *testing.T) {
	// Stop command failure
	runnerStopErr := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if args[0] == "get" && args[1] == "dom.active" {
			return []byte("1\n"), nil
		}
		if args[0] == "stop" {
			return nil, errors.New("stop failed")
		}
		return nil, nil
	}
	tracker := NewTracker(runnerStopErr)
	_, err := tracker.Stop(context.Background())
	if err == nil {
		t.Error("expected error when stop command fails")
	}

	// Export invalid JSON
	runnerBadJSON := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if args[0] == "get" && args[1] == "dom.active" {
			return []byte("1\n"), nil
		}
		if args[0] == "stop" {
			return []byte("ok"), nil
		}
		if args[0] == "export" {
			return []byte("not-json"), nil
		}
		return nil, nil
	}
	tracker2 := NewTracker(runnerBadJSON)
	_, err = tracker2.Stop(context.Background())
	if err == nil {
		t.Error("expected error when export returns invalid JSON")
	}
}
