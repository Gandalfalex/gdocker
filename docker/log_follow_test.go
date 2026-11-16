package docker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"gdocker/models"

	tea "charm.land/bubbletea/v2"
	"github.com/moby/moby/client"
)

func resetLogFollowerStateForTest() {
	logFollowMu.Lock()
	defer logFollowMu.Unlock()
	if activeFollower != nil {
		activeFollower.cancel()
		activeFollower = nil
	}
	logFollowRun = runLogFollower
}

func TestFollowLogsDeliversStreamMessage(t *testing.T) {
	resetLogFollowerStateForTest()
	t.Cleanup(resetLogFollowerStateForTest)

	logFollowRun = func(_ context.Context, _ *client.Client, _ string, since time.Time, ch chan tea.Msg) {
		defer close(ch)
		ch <- models.LogsFollowedMsg{
			Lines: []string{"line-a"},
			Since: since.Add(time.Second),
		}
	}

	c := models.Container{FullID: "full-1"}
	m := models.Model{
		ViewMode: models.ViewLogs,
		Cursor:   0,
		Items: []models.ListItem{
			{IsContainer: true, Container: &c},
		},
	}
	m.LogViewer.FollowingLogs = true

	cmd := FollowLogs(&m)
	if cmd == nil {
		t.Fatalf("expected non-nil follow cmd")
	}
	msg := cmd()

	followed, ok := msg.(models.LogsFollowedMsg)
	if !ok {
		t.Fatalf("expected LogsFollowedMsg, got %T", msg)
	}
	if len(followed.Lines) != 1 || followed.Lines[0] != "line-a" {
		t.Fatalf("unexpected followed lines: %#v", followed.Lines)
	}
}

func TestFollowLogsReturnsStoppedMessageOnClosedChannel(t *testing.T) {
	resetLogFollowerStateForTest()
	t.Cleanup(resetLogFollowerStateForTest)

	logFollowRun = func(_ context.Context, _ *client.Client, _ string, _ time.Time, ch chan tea.Msg) {
		close(ch)
	}

	c := models.Container{FullID: "full-1"}
	m := models.Model{
		ViewMode: models.ViewLogs,
		Cursor:   0,
		Items: []models.ListItem{
			{IsContainer: true, Container: &c},
		},
	}
	m.LogViewer.FollowingLogs = true

	cmd := FollowLogs(&m)
	if cmd == nil {
		t.Fatalf("expected non-nil follow cmd")
	}
	msg := cmd()
	if _, ok := msg.(models.LogFollowStoppedMsg); !ok {
		t.Fatalf("expected LogFollowStoppedMsg, got %T", msg)
	}
}

func TestEnsureLogFollowerReusesSameContainerFollower(t *testing.T) {
	resetLogFollowerStateForTest()
	t.Cleanup(resetLogFollowerStateForTest)

	var calls int32
	done := make(chan struct{})
	logFollowRun = func(ctx context.Context, _ *client.Client, _ string, _ time.Time, ch chan tea.Msg) {
		defer close(ch)
		atomic.AddInt32(&calls, 1)
		<-ctx.Done()
		close(done)
	}

	s1 := ensureLogFollower(nil, "container-a", time.Now())
	s2 := ensureLogFollower(nil, "container-a", time.Now())
	if s1 == nil || s2 == nil {
		t.Fatalf("expected non-nil follower state")
	}
	if s1 != s2 {
		t.Fatalf("expected same follower state for identical container")
	}
	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&calls) < 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected one runner call, got %d", got)
	}

	StopFollowLogs(nil)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for follower goroutine to stop")
	}
}

func TestEnsureLogFollowerSwitchingContainerCancelsPrevious(t *testing.T) {
	resetLogFollowerStateForTest()
	t.Cleanup(resetLogFollowerStateForTest)

	started := make(chan string, 2)
	canceled := make(chan string, 2)
	logFollowRun = func(ctx context.Context, _ *client.Client, containerID string, _ time.Time, ch chan tea.Msg) {
		defer close(ch)
		started <- containerID
		<-ctx.Done()
		canceled <- containerID
	}

	ensureLogFollower(nil, "container-a", time.Now())
	ensureLogFollower(nil, "container-b", time.Now())

	select {
	case id := <-canceled:
		if id != "container-a" {
			t.Fatalf("expected container-a to be canceled first, got %s", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for previous follower cancellation")
	}

	logFollowMu.Lock()
	active := activeFollower
	logFollowMu.Unlock()
	if active == nil || active.containerID != "container-b" {
		t.Fatalf("expected active follower for container-b, got %#v", active)
	}

	// Drain starts to ensure both goroutines started.
	gotStarts := map[string]bool{}
	for i := 0; i < 2; i++ {
		select {
		case id := <-started:
			gotStarts[id] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for follower start notifications")
		}
	}
	if !gotStarts["container-a"] || !gotStarts["container-b"] {
		t.Fatalf("missing start notifications: %#v", gotStarts)
	}
}

func TestStopFollowLogsClearsActiveFollower(t *testing.T) {
	resetLogFollowerStateForTest()
	t.Cleanup(resetLogFollowerStateForTest)

	stopped := make(chan struct{})
	logFollowRun = func(ctx context.Context, _ *client.Client, _ string, _ time.Time, ch chan tea.Msg) {
		defer close(ch)
		<-ctx.Done()
		close(stopped)
	}

	ensureLogFollower(nil, "container-a", time.Now())
	StopFollowLogs(nil)

	logFollowMu.Lock()
	active := activeFollower
	logFollowMu.Unlock()
	if active != nil {
		t.Fatalf("expected active follower to be nil after stop")
	}

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for follower stop signal")
	}
}
