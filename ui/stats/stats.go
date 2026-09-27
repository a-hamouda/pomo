// Package stats implements the statistics view for pomo.
package stats

import (
	"errors"
	"fmt"
	"time"

	"github.com/Bahaaio/pomo/db"
	"github.com/Bahaaio/pomo/ui/colors"
	"github.com/Bahaaio/pomo/ui/stats/components"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	barChartHeight     = 12
	durationRatioWidth = 30
)

var errStyle = lipgloss.NewStyle().
	Foreground(colors.ErrorMessageFg).
	AlignHorizontal(lipgloss.Center)

type Model struct {
	// components
	durationRatio components.DurationRatio
	barChart      components.BarChart
	heatMap       components.HeatMap
	streak        components.Streak

	// error message
	err error

	// stats
	allTimeStats db.AllTimeStats
	weeklyStats  []db.DailyStat
	monthlyStats []db.DailyStat
	streakStats  db.StreakStats
	taskStats    []db.TaskStat

	// state
	width, height int
	help          help.Model
	quitting      bool
	periodEnd     time.Time
	taskIndex     int
}

func New() Model {
	now := time.Now()
	return Model{
		durationRatio: components.NewDurationRatio(durationRatioWidth),
		barChart:      components.NewBarChart(barChartHeight),
		heatMap:       components.NewHeatMap(),
		streak:        components.NewStreak(),
		help:          help.New(),
		periodEnd:     time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
	}
}

type statsMsg struct {
	allTimeStats db.AllTimeStats
	weeklyStats  []db.DailyStat
	monthlyStats []db.DailyStat
	streakStats  db.StreakStats
	taskStats    []db.TaskStat
	periodEnd    time.Time
	task         string
}

type errMsg struct {
	err       error
	periodEnd time.Time
	task      string
}

func fetchStats(periodEnd time.Time, task string) tea.Cmd {
	return func() tea.Msg {
		database, err := db.Connect()
		if err != nil {
			return errMsg{err: errors.New("failed to connect to the database"), periodEnd: periodEnd, task: task}
		}
		defer database.Close()

		repo := db.NewSessionRepo(database)

		stats, err := repo.GetAllTimeStats()
		if err != nil {
			return errMsg{err: errors.New("failed to fetch all-time stats"), periodEnd: periodEnd, task: task}
		}

		firstMonth := time.Date(periodEnd.Year(), periodEnd.Month(), 1, 0, 0, 0, 0, periodEnd.Location()).AddDate(0, -components.NumberOfMonths+1, 0)
		dailyStats, err := repo.GetDailyStats(firstMonth, periodEnd, task)
		if err != nil {
			return errMsg{err: errors.New("failed to fetch daily stats"), periodEnd: periodEnd, task: task}
		}
		weeklyStats := dailyStats[max(0, len(dailyStats)-7):]

		streakStats, err := repo.GetStreakStats(task)
		if err != nil {
			return errMsg{err: errors.New("failed to fetch streak stats"), periodEnd: periodEnd, task: task}
		}

		taskStats, err := repo.GetTaskStats()
		if err != nil {
			return errMsg{err: errors.New("failed to fetch task stats"), periodEnd: periodEnd, task: task}
		}

		return statsMsg{
			allTimeStats: stats,
			weeklyStats:  weeklyStats,
			monthlyStats: dailyStats,
			streakStats:  streakStats,
			taskStats:    taskStats,
			periodEnd:    periodEnd,
			task:         task,
		}
	}
}

func (m Model) Init() tea.Cmd {
	return fetchStats(m.periodEnd, m.selectedTask())
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	if m.err != nil {
		return m.buildErrorMessage()
	}

	title := "Pomodoro statistics"
	periodStart := m.periodEnd.AddDate(0, 0, -6)
	filter := m.selectedTask()
	if filter == "" {
		filter = "All tasks"
	} else {
		filter = lipgloss.NewStyle().Foreground(colors.TaskColor(filter)).Render(filter)
	}
	period := fmt.Sprintf("%s - %s  ·  %s", periodStart.Format("Jan 2"), m.periodEnd.Format("Jan 2, 2006"), filter)

	durationRatio := m.durationRatio.View(
		m.allTimeStats.TotalWorkDuration,
		m.allTimeStats.TotalBreakDuration,
	)

	streak := m.streak.View(m.streakStats)
	tasks := renderTaskStats(m.taskStats, m.selectedTask())

	chart := m.barChart.View(m.weeklyStats)
	hMap := m.heatMap.View(m.monthlyStats, m.periodEnd)

	charts := lipgloss.JoinHorizontal(lipgloss.Bottom, chart, "   ", hMap)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(
			lipgloss.Center,
			title,
			period,
			"\n",
			"All-time work / break",
			durationRatio,
			"",
			streak,
			"",
			tasks,
			"\n",
			charts,
			"",
			m.help.View(Keys),
		),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statsMsg:
		if !sameDay(msg.periodEnd, m.periodEnd) || msg.task != m.selectedTask() {
			return m, nil
		}
		m.allTimeStats = msg.allTimeStats
		m.weeklyStats = msg.weeklyStats
		m.monthlyStats = msg.monthlyStats
		m.streakStats = msg.streakStats
		m.taskStats = msg.taskStats
		return m, nil
	case errMsg:
		if !sameDay(msg.periodEnd, m.periodEnd) || msg.task != m.selectedTask() {
			return m, nil
		}
		m.err = msg.err
		return m, nil
	case tea.KeyMsg:
		return m, m.handleKeys(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	default:
		return m, nil
	}
}

func renderTaskStats(stats []db.TaskStat, selected string) string {
	if len(stats) == 0 {
		return ""
	}

	lines := []string{"All-time tasks"}
	for _, stat := range stats {
		prefix := "  "
		if stat.Task == selected {
			prefix = "› "
		}
		label := lipgloss.NewStyle().Foreground(colors.TaskColor(stat.Task)).Render(prefix + "■ " + stat.Task)
		lines = append(lines, fmt.Sprintf("%s  %v", label, stat.Duration))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *Model) buildErrorMessage() string {
	title := "An error occurred while fetching statistics."
	message := m.err.Error()

	help := m.help.View(Keys)

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		"",
		message,
		"",
		help,
	)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		errStyle.Render(content),
	)
}

func (m Model) selectedTask() string {
	if m.taskIndex == 0 || m.taskIndex > len(m.taskStats) {
		return ""
	}
	return m.taskStats[m.taskIndex-1].Task
}

func (m *Model) handleKeys(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, Keys.Quit):
		return tea.Quit
	case key.Matches(msg, Keys.PreviousPeriod):
		m.periodEnd = m.periodEnd.AddDate(0, 0, -7)
	case key.Matches(msg, Keys.NextPeriod):
		today := startOfDay(time.Now())
		m.periodEnd = m.periodEnd.AddDate(0, 0, 7)
		if m.periodEnd.After(today) {
			m.periodEnd = today
		}
	case key.Matches(msg, Keys.Today):
		m.periodEnd = startOfDay(time.Now())
	case key.Matches(msg, Keys.PreviousTask):
		m.taskIndex--
		if m.taskIndex < 0 {
			m.taskIndex = len(m.taskStats)
		}
	case key.Matches(msg, Keys.NextTask):
		m.taskIndex = (m.taskIndex + 1) % (len(m.taskStats) + 1)
	default:
		return nil
	}
	m.err = nil
	return fetchStats(m.periodEnd, m.selectedTask())
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func sameDay(a, b time.Time) bool {
	return a.Format(db.DateFormat) == b.Format(db.DateFormat)
}
