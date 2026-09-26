package ui

import (
	"testing"
	"time"

	"github.com/Bahaaio/pomo/config"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/timer"
	"github.com/stretchr/testify/assert"
)

func newOnStartTestModel(task config.Task) Model {
	return Model{
		progressBar:     progress.New(progress.WithDefaultGradient()),
		timer:           timer.New(task.Duration),
		duration:        task.Duration,
		sessionState:    Running,
		currentTaskType: config.WorkTask,
		currentTask:     task,
		onSessionEnd:    "ask",
	}
}

func TestOnStartLifecycle(t *testing.T) {
	orig := runOnStart
	defer func() { runOnStart = orig }()

	calls := 0
	runOnStart = func(task config.Task) { calls++ }

	task := config.Task{
		Title:    "work",
		Duration: 25 * time.Minute,
		OnStart:  [][]string{{"echo", "hi"}},
		OnEnd:    [][]string{{"echo", "bye"}},
	}

	m := newOnStartTestModel(task)

	// executes on session start, exactly once
	m.Init()
	assert.Equal(t, 1, calls, "Init should invoke onStart exactly once")

	// pause/resume does not retrigger
	m.sessionState = Paused
	m.sessionState = Running
	assert.Equal(t, 1, calls, "pause/resume must not retrigger onStart")

	// normal mutations do not retrigger
	m.duration += time.Minute
	_ = m.updateProgressBar()
	assert.Equal(t, 1, calls, "duration mutation must not retrigger onStart")

	// starting a new session triggers it again
	next := config.Task{Title: "break", Duration: 5 * time.Minute, OnStart: [][]string{{"echo", "break"}}}
	m.startSession(config.BreakTask, next, false)
	assert.Equal(t, 2, calls, "new session should invoke onStart again")
}
