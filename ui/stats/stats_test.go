package stats_test

import (
	"testing"

	"github.com/Bahaaio/pomo/ui/colors"
)

func TestTaskColor(t *testing.T) {
	if colors.TaskColor("OCP Java") == colors.TaskColor("Other") {
		t.Fatal("different task names should have different colors")
	}
	if colors.TaskColor("OCP Java") != colors.TaskColor("OCP Java") {
		t.Fatal("task colors should be stable")
	}
}
