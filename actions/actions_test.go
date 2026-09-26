package actions

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Bahaaio/pomo/config"
	"github.com/stretchr/testify/assert"
)

// TestHelperProcess re-executes the test binary as a portable stand-in for
// an external command: it creates the file given after "--" and exits.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	for i := 0; i+1 < len(os.Args); i++ {
		if os.Args[i] == "--" {
			if err := os.WriteFile(os.Args[i+1], []byte("ok"), 0o644); err != nil {
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	os.Exit(1)
}

func TestStartAndEndShareExecution(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")

	dir := t.TempDir()
	startFile := filepath.Join(dir, "started")
	endFile := filepath.Join(dir, "ended")

	helper := func(file string) []string {
		return []string{os.Args[0], "-test.run=TestHelperProcess", "--", file}
	}

	task := config.Task{
		Notification: config.Notification{Enabled: false},
		OnStart:      [][]string{helper(startFile)},
		OnEnd:        [][]string{helper(endFile)},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	RunStartActions(ctx, task)
	assert.FileExists(t, startFile, "onStart should execute its commands")

	wg := RunPostActions(ctx, task)
	wg.Wait()
	assert.FileExists(t, endFile, "onEnd should execute its commands")
}
