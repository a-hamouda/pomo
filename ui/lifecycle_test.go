package ui

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Bahaaio/pomo/config"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/timer"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// callLog records hook invocations from stubbed runners.
type callLog struct {
	mu    sync.Mutex
	calls int
	tasks []config.Task
	ctxs  []context.Context
}

func (l *callLog) record(ctx context.Context, task config.Task) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls++
	l.tasks = append(l.tasks, task)
	l.ctxs = append(l.ctxs, ctx)
}

func (l *callLog) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.calls
}

func stubRunners(t *testing.T, startLog, postLog *callLog) {
	t.Helper()
	origStart, origPost := runStartCommands, runPostActions
	runStartCommands = func(ctx context.Context, task config.Task) {
		startLog.record(ctx, task)
	}
	runPostActions = func(ctx context.Context, task config.Task) *sync.WaitGroup {
		postLog.record(ctx, task)
		return &sync.WaitGroup{}
	}
	t.Cleanup(func() {
		runStartCommands, runPostActions = origStart, origPost
	})
}

func saveConfig(t *testing.T) {
	t.Helper()
	orig := config.C
	t.Cleanup(func() { config.C = orig })
}

func newLifecycleTestModel(task config.Task) Model {
	return Model{
		progressBar:     progress.New(progress.WithDefaultGradient()),
		timer:           timer.New(task.Duration),
		duration:        task.Duration,
		sessionState:    Running,
		currentTaskType: config.WorkTask,
		currentTask:     task,
		onSessionEnd:    "ask",
		longBreak:       config.LongBreak{Enabled: false},
		lifecycle:       &lifecycleState{},
	}
}

func spaceKey() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeySpace} }
func runeKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func update(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	updated, _ := m.Update(msg)
	next, ok := updated.(Model)
	assert.True(t, ok, "Update should return a ui.Model")
	return next
}

func settleProgressBar(t *testing.T, m *Model) {
	t.Helper()
	cmd := m.progressBar.SetPercent(1.0)
	for i := 0; i < 2000 && m.progressBar.IsAnimating(); i++ {
		if cmd == nil {
			break
		}
		msg := cmd()
		updated, c := m.progressBar.Update(msg)
		m.progressBar = updated.(progress.Model)
		cmd = c
	}
	assert.False(t, m.progressBar.IsAnimating(), "progress bar should settle")
	assert.Equal(t, 1.0, m.progressBar.Percent(), "progress bar should be complete")
}

func TestOnStartRunsOnInit(t *testing.T) {
	var startLog, postLog callLog
	stubRunners(t, &startLog, &postLog)
	saveConfig(t)
	t.Setenv("HOME", t.TempDir())

	task := config.Task{
		Title:    "work",
		Duration: 25 * time.Minute,
		OnStart:  [][]string{{"echo", "hi"}},
	}
	config.C.Work = task

	// constructing the model must not trigger external side effects
	m := NewModel(config.WorkTask, config.Config{
		OnSessionEnd: "ask",
		LongBreak:    config.LongBreak{Enabled: false},
	})
	assert.Equal(t, 0, startLog.count(), "NewModel must not invoke onStart")

	// the Bubble Tea startup boundary starts the session
	m.Init()
	assert.Eventually(t, func() bool { return startLog.count() == 1 }, 3*time.Second, 10*time.Millisecond,
		"initial session should invoke onStart exactly once")
	assert.Equal(t, 0, postLog.count(), "initial session must not invoke onEnd")
}

func TestOnStartPauseResumeAndMutation(t *testing.T) {
	var startLog, postLog callLog
	stubRunners(t, &startLog, &postLog)

	task := config.Task{Title: "work", Duration: 25 * time.Minute, OnStart: [][]string{{"echo", "hi"}}}
	m := newLifecycleTestModel(task)
	m.startSession(config.WorkTask, task, false)
	assert.Eventually(t, func() bool { return startLog.count() == 1 }, 3*time.Second, 10*time.Millisecond)

	// pause and resume through the real key-handling path
	m = update(t, m, spaceKey())
	assert.Equal(t, Paused, m.sessionState, "space should pause the session")
	m = update(t, m, spaceKey())
	assert.Equal(t, Running, m.sessionState, "space should resume the session")

	// duration mutations through the real key-handling path
	duration := m.duration
	m = update(t, m, runeKey('k'))
	assert.Greater(t, m.duration, duration, "increase key should extend the duration")
	m = update(t, m, runeKey('h'))

	// give any stray async invocation a chance to land, then assert
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, 1, startLog.count(), "pause/resume and mutations must not retrigger onStart")
	assert.Equal(t, 0, postLog.count(), "pause/resume and mutations must not invoke onEnd")
}

func TestOnStartNewSession(t *testing.T) {
	var startLog, postLog callLog
	stubRunners(t, &startLog, &postLog)

	work := config.Task{Title: "work", Duration: 25 * time.Minute, OnStart: [][]string{{"echo", "work"}}}
	m := newLifecycleTestModel(work)
	m.startSession(config.WorkTask, work, false)
	assert.Eventually(t, func() bool { return startLog.count() == 1 }, 3*time.Second, 10*time.Millisecond)

	next := config.Task{Title: "break", Duration: 5 * time.Minute, OnStart: [][]string{{"echo", "break"}}}
	m.startSession(config.BreakTask, next, false)
	assert.Eventually(t, func() bool { return startLog.count() == 2 }, 3*time.Second, 10*time.Millisecond,
		"a new session should invoke its own onStart")
}

func TestOnStartCancelledOnNextSession(t *testing.T) {
	saveConfig(t)
	origStart := runStartCommands
	release := make(chan struct{})
	var log callLog
	runStartCommands = func(ctx context.Context, task config.Task) {
		log.record(ctx, task)
		select {
		case <-release:
		case <-ctx.Done():
		}
	}
	t.Cleanup(func() {
		runStartCommands = origStart
		close(release)
	})

	work := config.Task{Title: "work", Duration: 25 * time.Minute, OnStart: [][]string{{"echo", "work"}}}
	m := newLifecycleTestModel(work)
	m.startSession(config.WorkTask, work, false)
	assert.Eventually(t, func() bool { return log.count() == 1 }, 3*time.Second, 10*time.Millisecond)

	next := config.Task{Title: "break", Duration: 5 * time.Minute, OnStart: [][]string{{"echo", "break"}}}
	m.startSession(config.BreakTask, next, false)

	log.mu.Lock()
	firstCtx := log.ctxs[0]
	log.mu.Unlock()
	assert.Eventually(t, func() bool { return firstCtx.Err() != nil }, 2*time.Second, 10*time.Millisecond,
		"starting the next session should cancel the previous onStart commands")
	assert.Eventually(t, func() bool { return log.count() == 2 }, 3*time.Second, 10*time.Millisecond)
}

func TestOnStartCancelledOnCompletion(t *testing.T) {
	// blocking start stub: the context can only be cancelled by the
	// session lifecycle, not by the command finishing on its own
	origStart, origPost := runStartCommands, runPostActions
	release := make(chan struct{})
	var startLog, postLog callLog
	runStartCommands = func(ctx context.Context, task config.Task) {
		startLog.record(ctx, task)
		select {
		case <-release:
		case <-ctx.Done():
		}
	}
	runPostActions = func(ctx context.Context, task config.Task) *sync.WaitGroup {
		postLog.record(ctx, task)
		return &sync.WaitGroup{}
	}
	t.Cleanup(func() {
		runStartCommands, runPostActions = origStart, origPost
		close(release)
	})

	task := config.Task{
		Title:    "work",
		Duration: 25 * time.Minute,
		OnStart:  [][]string{{"echo", "hi"}},
		OnEnd:    [][]string{{"echo", "bye"}},
	}
	m := newLifecycleTestModel(task)

	// start through the Bubble Tea startup boundary (value-receiver Init
	// registers cancellation in the shared lifecycle state)
	m.Init()
	assert.Eventually(t, func() bool { return startLog.count() == 1 }, 3*time.Second, 10*time.Millisecond)

	m.handleCompletion()

	startLog.mu.Lock()
	firstCtx := startLog.ctxs[0]
	startLog.mu.Unlock()
	assert.Eventually(t, func() bool { return firstCtx.Err() != nil }, 2*time.Second, 10*time.Millisecond,
		"session end should cancel still-running onStart commands")
	assert.Equal(t, 1, postLog.count(), "completion should invoke onEnd")
}

func TestOnEndCompletionOnce(t *testing.T) {
	var startLog, postLog callLog
	stubRunners(t, &startLog, &postLog)

	task := config.Task{
		Title:    "work",
		Duration: 25 * time.Minute,
		OnEnd:    [][]string{{"echo", "bye"}},
	}
	m := newLifecycleTestModel(task)

	settleProgressBar(t, &m)
	m.handleProgressBarFrame(progress.FrameMsg{})

	assert.Equal(t, 1, postLog.count(), "normal completion should invoke onEnd")
	postLog.mu.Lock()
	got := postLog.tasks[0].OnEnd
	postLog.mu.Unlock()
	assert.Equal(t, task.OnEnd, got, "onEnd should run the configured commands")
	assert.Equal(t, 0, startLog.count(), "completion must not retrigger onStart")

	// a repeated completion frame for the same transition must not run onEnd again
	m.handleProgressBarFrame(progress.FrameMsg{})
	assert.Equal(t, 1, postLog.count(), "onEnd must not execute more than once per end transition")
}

func TestOnEndSkipAndQuit(t *testing.T) {
	var startLog, postLog callLog
	stubRunners(t, &startLog, &postLog)
	saveConfig(t)

	config.C.Break = config.Task{
		Title:    "break",
		Duration: 5 * time.Minute,
		OnStart:  [][]string{{"echo", "break"}},
		OnEnd:    [][]string{{"echo", "break done"}},
	}

	// skip starts the next session without running the current onEnd
	work := config.Task{Title: "work", Duration: 25 * time.Minute, OnEnd: [][]string{{"echo", "bye"}}}
	m := newLifecycleTestModel(work)
	m = update(t, m, runeKey('s'))
	assert.Equal(t, config.BreakTask, m.currentTaskType, "skip should start the next session")
	assert.Eventually(t, func() bool { return startLog.count() == 1 }, 3*time.Second, 10*time.Millisecond,
		"skip should trigger the next session's onStart")
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, 0, postLog.count(), "skip must not invoke onEnd")

	// quit does not start onEnd commands either
	m = newLifecycleTestModel(work)
	m = update(t, m, runeKey('q'))
	assert.Equal(t, Quitting, m.sessionState, "quit should leave the session")
	assert.Equal(t, 0, postLog.count(), "quit must not invoke onEnd")
}
