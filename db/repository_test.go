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
	if err := repo.CreateSession(time.Now(), 30*time.Minute, WorkSession, "OCP Java"); err != nil {
		t.Fatal(err)
	}

	stats, err := repo.GetTaskStats()
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 || stats[0].Task != "OCP Java" || stats[0].Duration != 30*time.Minute {
		t.Fatalf("unexpected task stats: %#v", stats)
	}
}
