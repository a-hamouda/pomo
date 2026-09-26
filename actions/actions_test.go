package actions

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Bahaaio/pomo/config"
	"github.com/stretchr/testify/assert"
)

func TestStartAndEndShareExecution(t *testing.T) {
	dir := t.TempDir()
	startFile := filepath.Join(dir, "started")
	endFile := filepath.Join(dir, "ended")

	task := config.Task{
		Notification: config.Notification{Enabled: false},
		OnStart:      [][]string{{"touch", startFile}},
		OnEnd:        [][]string{{"touch", endFile}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	RunStartActions(ctx, task)
	assert.FileExists(t, startFile, "onStart should execute its commands")

	wg := RunPostActions(ctx, task)
	wg.Wait()
	assert.FileExists(t, endFile, "onEnd should execute its commands")
}
