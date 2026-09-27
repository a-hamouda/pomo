package setup

import (
	"testing"
	"time"

	"github.com/Bahaaio/pomo/db"
	tea "github.com/charmbracelet/bubbletea"
)

func send(m Model, msg tea.Msg) Model {
	updated, _ := m.Update(msg)
	return updated.(Model)
}

func TestAcceptDefaults(t *testing.T) {
	m := New([]db.SavedTask{{Name: "Write report", Color: "#5A56E0"}}, "Write report", 25*time.Minute, 5*time.Minute)
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})

	result := m.Result()
	if !m.done || result.Task.Name != "Write report" || result.Task.Color != "#5A56E0" || result.WorkDuration != 25*time.Minute || result.BreakDuration != 5*time.Minute {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestCreateTaskWithColor(t *testing.T) {
	m := New([]db.SavedTask{{Name: "Existing"}}, "Existing", 25*time.Minute, 5*time.Minute)
	m = send(m, tea.KeyMsg{Type: tea.KeyDown})
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Deep work")})
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = send(m, tea.KeyMsg{Type: tea.KeyUp})
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	m.colorInput.SetValue("#123456")
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})

	result := m.Result()
	if !m.done || result.Task.Name != "Deep work" || result.Task.Color != "#123456" {
		t.Fatalf("unexpected result: %#v", result)
	}
}
