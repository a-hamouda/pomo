package stats

import (
	"testing"
	"time"

	"github.com/Bahaaio/pomo/db"
	"github.com/Bahaaio/pomo/ui/colors"
	tea "github.com/charmbracelet/bubbletea"
)

func TestTaskColor(t *testing.T) {
	if colors.TaskColor("OCP Java") == colors.TaskColor("Other") {
		t.Fatal("different task names should have different colors")
	}
	if colors.TaskColor("OCP Java") != colors.TaskColor("OCP Java") {
		t.Fatal("task colors should be stable")
	}
}

func TestNavigationAndTaskFilter(t *testing.T) {
	m := New()
	m.taskStats = []db.TaskStat{{Task: "OCP Java"}, {Task: "Other"}}
	today := m.periodEnd

	if cmd := m.handleKeys(tea.KeyMsg{Type: tea.KeyLeft}); cmd == nil || !m.periodEnd.Equal(today.AddDate(0, 0, -7)) {
		t.Fatal("left should load the previous week")
	}
	if cmd := m.handleKeys(tea.KeyMsg{Type: tea.KeyDown}); cmd == nil || m.selectedTask() != "OCP Java" {
		t.Fatal("down should select the next task")
	}
	m.periodEnd = today
	m.handleKeys(tea.KeyMsg{Type: tea.KeyRight})
	if !sameDay(m.periodEnd, startOfDay(time.Now())) {
		t.Fatal("right should not navigate beyond today")
	}
}
