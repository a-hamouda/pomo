package db

import (
	"time"

	"github.com/jmoiron/sqlx"
)

const DateFormat = "2006-01-02"

type SessionRepo struct {
	db *sqlx.DB
}

func NewSessionRepo(db *sqlx.DB) *SessionRepo {
	return &SessionRepo{db: db}
}

// CreateSession inserts a new session record into the database.
func (r *SessionRepo) CreateSession(startedAt time.Time, duration time.Duration, sessionType SessionType, task, color string) error {
	startedAtStr := startedAt.Format(time.RFC3339)
	if sessionType == WorkSession && task != "" {
		if _, err := r.db.Exec("INSERT OR IGNORE INTO tasks (name, color) VALUES (?, ?)", task, color); err != nil {
			return err
		}
	}

	if _, err := r.db.Exec(
		"insert into sessions (started_at, duration, type, task) values (?, ?, ?, ?);",
		startedAtStr,
		duration,
		sessionType,
		task,
	); err != nil {
		return err
	}

	return nil
}

func (r *SessionRepo) GetTasks() ([]SavedTask, error) {
	var tasks []SavedTask
	err := r.db.Select(&tasks, `
		SELECT tasks.name, tasks.color
		FROM tasks
		LEFT JOIN sessions ON sessions.type = 'work' AND COALESCE(NULLIF(sessions.task, ''), 'work') = tasks.name
		GROUP BY tasks.name, tasks.color
		ORDER BY MAX(sessions.started_at) DESC, tasks.name;
	`)
	return tasks, err
}

func (r *SessionRepo) GetTask(name string) (SavedTask, error) {
	var task SavedTask
	err := r.db.Get(&task, "SELECT name, color FROM tasks WHERE name = ?", name)
	return task, err
}

func (r *SessionRepo) SaveTask(task SavedTask) error {
	_, err := r.db.Exec(`INSERT INTO tasks (name, color) VALUES (?, ?)
		ON CONFLICT(name) DO UPDATE SET color = excluded.color`, task.Name, task.Color)
	return err
}

// GetTaskStats retrieves total work duration grouped by task.
func (r *SessionRepo) GetTaskStats() ([]TaskStat, error) {
	var stats []TaskStat
	err := r.db.Select(
		&stats,
		`SELECT
			COALESCE(NULLIF(sessions.task, ''), 'work') AS task,
			COALESCE(tasks.color, '') AS color,
			SUM(sessions.duration) AS duration
		FROM sessions
		LEFT JOIN tasks ON tasks.name = COALESCE(NULLIF(sessions.task, ''), 'work')
		WHERE sessions.type = 'work'
		GROUP BY COALESCE(NULLIF(sessions.task, ''), 'work'), tasks.color
		ORDER BY duration DESC, task;`,
	)
	return stats, err
}

// GetAllTimeStats retrieves aggregate statistics across all sessions.
func (r *SessionRepo) GetAllTimeStats() (AllTimeStats, error) {
	var totalStats AllTimeStats

	// sqlite treats (type = 'work') as 1 or 0
	if err := r.db.Get(
		&totalStats,
		`
		SELECT
			COUNT(*) AS total_sessions,
			COALESCE(SUM(duration * (type = 'work')), 0)  AS total_work_duration,
			COALESCE(SUM(duration * (type = 'break')), 0) AS total_break_duration
		FROM sessions;
		`,
	); err != nil {
		return AllTimeStats{}, err
	}

	return totalStats, nil
}

// GetStreakStats calculates the current and best streaks of consecutive work days.
// A streak is consecutive days with at least one 'work' session.
func (r *SessionRepo) GetStreakStats(task string) (StreakStats, error) {
	var dates []string

	if err := r.db.Select(
		&dates,
		`
		SELECT DISTINCT date(started_at) AS day
		FROM sessions
		WHERE type = 'work'
			AND (? = '' OR COALESCE(NULLIF(task, ''), 'work') = ?)
		ORDER BY day DESC;
		`,
		task, task,
	); err != nil {
		return StreakStats{}, err
	}

	return calculateStreak(dates), nil
}

// GetDailyStats retrieves task-filtered daily work durations for an inclusive range.
func (r *SessionRepo) GetDailyStats(from, to time.Time, task string) ([]DailyStat, error) {
	fromStr := from.Format(DateFormat)
	toStr := to.Format(DateFormat)

	var stats []dailyTaskStat

	if err := r.db.Select(
		&stats,
		`
		SELECT
			date(started_at) AS day,
			COALESCE(NULLIF(sessions.task, ''), 'work') AS task,
			COALESCE(tasks.color, '') AS color,
			SUM(sessions.duration) AS duration
		FROM sessions
		LEFT JOIN tasks ON tasks.name = COALESCE(NULLIF(sessions.task, ''), 'work')
		WHERE sessions.type = 'work'
			AND date(started_at) BETWEEN ? AND ?
			AND (? = '' OR COALESCE(NULLIF(sessions.task, ''), 'work') = ?)
		GROUP BY day, COALESCE(NULLIF(sessions.task, ''), 'work'), tasks.color
		ORDER BY day, task;
		`,
		fromStr, toStr, task, task,
	); err != nil {
		return nil, err
	}

	return r.normalizeStats(from, to, stats), nil
}

type dailyTaskStat struct {
	Date     string        `db:"day"`
	Task     string        `db:"task"`
	Color    string        `db:"color"`
	Duration time.Duration `db:"duration"`
}

// ensures that there is a DailyStat entry for each day
func (r *SessionRepo) normalizeStats(from, to time.Time, stats []dailyTaskStat) []DailyStat {
	m := make(map[string]DailyStat)

	for _, stat := range stats {
		day := m[stat.Date]
		day.WorkDuration += stat.Duration
		day.Tasks = append(day.Tasks, TaskStat{Task: stat.Task, Color: stat.Color, Duration: stat.Duration})
		m[stat.Date] = day
	}

	var normalized []DailyStat
	current := from
	for !current.After(to) {
		day := current.Format(DateFormat)
		stat := m[day]
		stat.Date = day
		normalized = append(normalized, stat)

		current = current.AddDate(0, 0, 1) // next day
	}

	return normalized
}
