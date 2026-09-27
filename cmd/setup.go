package cmd

import (
	"fmt"

	"github.com/Bahaaio/pomo/config"
	"github.com/Bahaaio/pomo/db"
	"github.com/Bahaaio/pomo/ui/setup"
	tea "github.com/charmbracelet/bubbletea"
)

func configurePomodoro() (bool, error) {
	database, err := db.Connect()
	if err != nil {
		return false, fmt.Errorf("could not load tasks: %w", err)
	}
	defer database.Close()

	repo := db.NewSessionRepo(database)
	tasks, err := repo.GetTasks()
	if err != nil {
		return false, fmt.Errorf("could not load tasks: %w", err)
	}

	program := tea.NewProgram(newSetupModel(tasks), tea.WithAltScreen())
	final, err := program.Run()
	if err != nil {
		return false, err
	}
	result := final.(setup.Model).Result()
	if result.Cancelled {
		return false, nil
	}
	if err := repo.SaveTask(result.Task); err != nil {
		return false, fmt.Errorf("could not save task: %w", err)
	}

	config.C.Work.Title = result.Task.Name
	config.C.Work.Color = result.Task.Color
	config.C.Work.Duration = result.WorkDuration
	config.C.Break.Duration = result.BreakDuration
	return true, nil
}

func newSetupModel(tasks []db.SavedTask) setup.Model {
	return setup.New(tasks, config.C.Work.Title, config.C.Work.Duration, config.C.Break.Duration)
}
