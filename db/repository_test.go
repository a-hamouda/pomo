package db

import (
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

func TestTaskStatsWithLegacyDatabase(t *testing.T) {
	database := sqlx.MustOpen("sqlite", ":memory:")
	defer database.Close()
	database.MustExec(`CREATE TABLE sessions(
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL,
		duration INTEGER NOT NULL,
		started_at TEXT NOT NULL
	)`)

	if err := createSchema(database); err != nil {
		t.Fatal(err)
	}

	repo := NewSessionRepo(database)
	now := time.Now()
	now = time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
	if err := repo.CreateSession(now, 30*time.Minute, WorkSession, "Write report", "#5A56E0"); err != nil {
		t.Fatal(err)
	}

	stats, err := repo.GetTaskStats()
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 || stats[0].Task != "Write report" || stats[0].Color != "#5A56E0" || stats[0].Duration != 30*time.Minute {
		t.Fatalf("unexpected task stats: %#v", stats)
	}

	if err := repo.CreateSession(now, 15*time.Minute, WorkSession, "Other", ""); err != nil {
		t.Fatal(err)
	}
	weekly, err := repo.GetDailyStats(now, now, "")
	if err != nil {
		t.Fatal(err)
	}
	today := weekly[len(weekly)-1]
	if today.WorkDuration != 45*time.Minute || len(today.Tasks) != 2 {
		t.Fatalf("unexpected daily task stats: %#v", today)
	}

	filtered, err := repo.GetDailyStats(now, now, "Other")
	if err != nil {
		t.Fatal(err)
	}
	if filtered[0].WorkDuration != 15*time.Minute || len(filtered[0].Tasks) != 1 || filtered[0].Tasks[0].Task != "Other" {
		t.Fatalf("unexpected filtered stats: %#v", filtered)
	}

	if err := repo.SaveTask(SavedTask{Name: "Write report", Color: "#F25D94"}); err != nil {
		t.Fatal(err)
	}
	task, err := repo.GetTask("Write report")
	if err != nil || task.Color != "#F25D94" {
		t.Fatalf("unexpected saved task: %#v, %v", task, err)
	}
	stats, err = repo.GetTaskStats()
	if err != nil || stats[0].Color != "#F25D94" {
		t.Fatalf("task stats did not use saved color: %#v, %v", stats, err)
	}
}
