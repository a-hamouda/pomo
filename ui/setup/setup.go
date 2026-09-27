// Package setup implements the interactive pomodoro setup flow.
package setup

import (
	"fmt"
	"strings"
	"time"

	"github.com/Bahaaio/pomo/db"
	"github.com/Bahaaio/pomo/ui/colors"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type step byte

const (
	taskStep step = iota
	nameStep
	colorStep
	hexStep
	workStep
	breakStep
	confirmStep
)

var palette = []string{
	"#5A56E0", "#F25D94", "#4A9EFF", "#20C997",
	"#198754", "#E6A23C", "#FF4C4C", "#A070FF",
}

var (
	cardStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 3)
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(colors.PurpleLight)
	dimStyle   = lipgloss.NewStyle().Foreground(colors.DimGray)
	errorStyle = lipgloss.NewStyle().Foreground(colors.ErrorMessageFg)
)

type Result struct {
	Task          db.SavedTask
	WorkDuration  time.Duration
	BreakDuration time.Duration
	Cancelled     bool
}

type Model struct {
	tasks                   []db.SavedTask
	taskCursor, colorCursor int
	step                    step
	creating                bool
	selected                db.SavedTask
	nameInput               textinput.Model
	colorInput              textinput.Model
	workInput               textinput.Model
	breakInput              textinput.Model
	workDuration            time.Duration
	breakDuration           time.Duration
	err                     string
	done, cancelled         bool
	width, height           int
}

func New(tasks []db.SavedTask, defaultTask string, workDuration, breakDuration time.Duration) Model {
	tasks = withDefaultTask(tasks, defaultTask)
	nameInput := newInput("Write report")
	colorInput := newInput("#5A56E0")
	colorInput.SetValue("#5A56E0")
	workInput := newInput(workDuration.String())
	breakInput := newInput(breakDuration.String())

	return Model{
		tasks:         tasks,
		step:          taskStep,
		nameInput:     nameInput,
		colorInput:    colorInput,
		workInput:     workInput,
		breakInput:    breakInput,
		workDuration:  workDuration,
		breakDuration: breakDuration,
	}
}

func newInput(placeholder string) textinput.Model {
	input := textinput.New()
	input.Prompt = "❯ "
	input.PromptStyle = lipgloss.NewStyle().Foreground(colors.PurpleLight)
	input.Placeholder = placeholder
	input.CharLimit = 64
	input.Width = 32
	input.SetValue("")
	return input
}

func withDefaultTask(tasks []db.SavedTask, defaultTask string) []db.SavedTask {
	for i, task := range tasks {
		if task.Name == defaultTask {
			ordered := []db.SavedTask{task}
			ordered = append(ordered, tasks[:i]...)
			return append(ordered, tasks[i+1:]...)
		}
	}
	return append([]db.SavedTask{{Name: defaultTask}}, tasks...)
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.cancelled = true
			return m, tea.Quit
		}
		if msg.Type == tea.KeyEsc {
			return m, m.back()
		}
		return m, m.updateKey(msg)
	}
	return m, nil
}

func (m *Model) updateKey(msg tea.KeyMsg) tea.Cmd {
	switch m.step {
	case taskStep:
		return m.updateTask(msg)
	case colorStep:
		return m.updateColor(msg)
	case nameStep, hexStep, workStep, breakStep:
		return m.updateInput(msg)
	case confirmStep:
		if msg.Type == tea.KeyEnter || msg.String() == "y" {
			m.done = true
			return tea.Quit
		}
	}
	return nil
}

func (m *Model) updateTask(msg tea.KeyMsg) tea.Cmd {
	count := len(m.tasks) + 1
	switch msg.String() {
	case "up", "k":
		m.taskCursor = (m.taskCursor - 1 + count) % count
	case "down", "j":
		m.taskCursor = (m.taskCursor + 1) % count
	case "enter":
		m.err = ""
		if m.taskCursor == len(m.tasks) {
			m.creating = true
			m.nameInput.SetValue("")
			return m.setStep(nameStep)
		}
		m.creating = false
		m.selected = m.tasks[m.taskCursor]
		return m.setStep(workStep)
	}
	return nil
}

func (m *Model) updateColor(msg tea.KeyMsg) tea.Cmd {
	count := len(palette) + 1
	switch msg.String() {
	case "up", "k", "left", "h":
		m.colorCursor = (m.colorCursor - 1 + count) % count
	case "down", "j", "right", "l":
		m.colorCursor = (m.colorCursor + 1) % count
	case "enter":
		m.err = ""
		if m.colorCursor == len(palette) {
			return m.setStep(hexStep)
		}
		m.selected.Color = palette[m.colorCursor]
		return m.setStep(workStep)
	}
	return nil
}

func (m *Model) updateInput(msg tea.KeyMsg) tea.Cmd {
	if msg.Type == tea.KeyEnter {
		m.err = ""
		switch m.step {
		case nameStep:
			name := strings.TrimSpace(m.nameInput.Value())
			if name == "" {
				m.err = "Enter a task name"
				return nil
			}
			for _, task := range m.tasks {
				if task.Name == name {
					m.err = "That task already exists"
					return nil
				}
			}
			m.selected = db.SavedTask{Name: name}
			return m.setStep(colorStep)
		case hexStep:
			color := strings.ToUpper(strings.TrimSpace(m.colorInput.Value()))
			if !colors.IsValid(color) {
				m.err = "Use a six-digit hex color, for example #5A56E0"
				return nil
			}
			m.selected.Color = color
			return m.setStep(workStep)
		case workStep:
			if strings.TrimSpace(m.workInput.Value()) != "" {
				duration, err := parseDuration(m.workInput.Value())
				if err != nil {
					m.err = err.Error()
					return nil
				}
				m.workDuration = duration
			}
			return m.setStep(breakStep)
		case breakStep:
			if strings.TrimSpace(m.breakInput.Value()) != "" {
				duration, err := parseDuration(m.breakInput.Value())
				if err != nil {
					m.err = err.Error()
					return nil
				}
				m.breakDuration = duration
			}
			return m.setStep(confirmStep)
		}
	}

	var cmd tea.Cmd
	switch m.step {
	case nameStep:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case hexStep:
		m.colorInput, cmd = m.colorInput.Update(msg)
	case workStep:
		m.workInput, cmd = m.workInput.Update(msg)
	case breakStep:
		m.breakInput, cmd = m.breakInput.Update(msg)
	}
	return cmd
}

func parseDuration(value string) (time.Duration, error) {
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("Enter a duration such as 25m or 1h")
	}
	return duration, nil
}

func (m *Model) setStep(next step) tea.Cmd {
	m.nameInput.Blur()
	m.colorInput.Blur()
	m.workInput.Blur()
	m.breakInput.Blur()
	m.step = next
	switch next {
	case nameStep:
		return m.nameInput.Focus()
	case hexStep:
		return m.colorInput.Focus()
	case workStep:
		return m.workInput.Focus()
	case breakStep:
		return m.breakInput.Focus()
	}
	return nil
}

func (m *Model) back() tea.Cmd {
	m.err = ""
	switch m.step {
	case taskStep:
		m.cancelled = true
		return tea.Quit
	case nameStep:
		return m.setStep(taskStep)
	case colorStep:
		return m.setStep(nameStep)
	case hexStep:
		return m.setStep(colorStep)
	case workStep:
		if m.creating {
			return m.setStep(colorStep)
		}
		return m.setStep(taskStep)
	case breakStep:
		return m.setStep(workStep)
	case confirmStep:
		return m.setStep(breakStep)
	}
	return nil
}

func (m Model) View() string {
	if m.done || m.cancelled {
		return ""
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("Set up your pomodoro"),
		"",
		m.stepView(),
		"",
		dimStyle.Render(m.help()),
	)
	if m.err != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, content, errorStyle.Render(m.err))
	}
	card := cardStyle.Render(content)
	if m.width == 0 || m.height == 0 {
		return card
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, card)
}

func (m Model) stepView() string {
	switch m.step {
	case taskStep:
		lines := []string{"Choose a task"}
		count := len(m.tasks) + 1
		start := max(0, m.taskCursor-5)
		end := min(count, start+7)
		start = max(0, end-7)
		if start > 0 {
			lines = append(lines, dimStyle.Render(fmt.Sprintf("  ↑ %d more", start)))
		}
		for i := start; i < end; i++ {
			if i == len(m.tasks) {
				lines = append(lines, m.option(i, "+  New task"))
				continue
			}
			task := m.tasks[i]
			lines = append(lines, m.option(i, lipgloss.NewStyle().Foreground(colors.TaskColor(task.Name, task.Color)).Render("●")+" "+task.Name))
		}
		if end < count {
			lines = append(lines, dimStyle.Render(fmt.Sprintf("  ↓ %d more", count-end)))
		}
		return strings.Join(lines, "\n")
	case nameStep:
		return "Name your task\n\n" + m.nameInput.View()
	case colorStep:
		lines := []string{"Choose a color for " + m.selected.Name}
		for i, color := range palette {
			swatch := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render("██")
			lines = append(lines, m.colorOption(i, swatch+"  "+color))
		}
		lines = append(lines, m.colorOption(len(palette), "◇   Custom hex"))
		return strings.Join(lines, "\n")
	case hexStep:
		return "Enter a color for " + m.selected.Name + "\n\n" + m.colorInput.View()
	case workStep:
		return m.taskLabel() + "\n\nWork duration\n\n" + m.workInput.View()
	case breakStep:
		return m.taskLabel() + "\n\nBreak duration\n\n" + m.breakInput.View()
	case confirmStep:
		return lipgloss.JoinVertical(lipgloss.Left,
			"Ready to focus?",
			"",
			m.taskLabel(),
			fmt.Sprintf("%v work  ·  %v break", m.workDuration, m.breakDuration),
		)
	}
	return ""
}

func (m Model) option(index int, label string) string {
	if index == m.taskCursor {
		return titleStyle.Render("❯ ") + label
	}
	return "  " + label
}

func (m Model) colorOption(index int, label string) string {
	if index == m.colorCursor {
		return titleStyle.Render("❯ ") + label
	}
	return "  " + label
}

func (m Model) taskLabel() string {
	color := colors.TaskColor(m.selected.Name, m.selected.Color)
	return lipgloss.NewStyle().Foreground(color).Render("● " + m.selected.Name)
}

func (m Model) help() string {
	switch m.step {
	case taskStep, colorStep:
		return "↑/↓ move  ·  enter select  ·  esc back"
	case confirmStep:
		return "enter start  ·  esc change  ·  ctrl+c cancel"
	default:
		return "enter accept  ·  esc back  ·  ctrl+c cancel"
	}
}

func (m Model) Result() Result {
	return Result{
		Task:          m.selected,
		WorkDuration:  m.workDuration,
		BreakDuration: m.breakDuration,
		Cancelled:     m.cancelled,
	}
}
