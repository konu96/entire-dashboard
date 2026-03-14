package db

import (
	"database/sql"
	"entire-dashboard/models"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS repositories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);

		CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			repo_path TEXT NOT NULL DEFAULT '',
			checkpoint_id TEXT NOT NULL,
			session_id TEXT NOT NULL UNIQUE,
			agent TEXT NOT NULL DEFAULT '',
			branch TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT '',
			prompt TEXT NOT NULL DEFAULT '',
			agent_lines INTEGER NOT NULL DEFAULT 0,
			human_added INTEGER NOT NULL DEFAULT 0,
			human_modified INTEGER NOT NULL DEFAULT 0,
			human_removed INTEGER NOT NULL DEFAULT 0,
			total_committed INTEGER NOT NULL DEFAULT 0,
			agent_percentage REAL NOT NULL DEFAULT 0,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			api_call_count INTEGER NOT NULL DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS idx_sessions_created_at ON sessions(created_at);
		CREATE INDEX IF NOT EXISTS idx_sessions_repo_path ON sessions(repo_path);
	`)
	return err
}

// Repository CRUD

func (s *Store) AddRepo(path, name string) (models.Repository, error) {
	if strings.TrimSpace(path) == "" {
		return models.Repository{}, fmt.Errorf("repo path is required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return models.Repository{}, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO repositories (path, name) VALUES (?, ?) ON CONFLICT(path) DO UPDATE SET name=excluded.name`,
		path, name,
	)
	if err != nil {
		return models.Repository{}, err
	}

	var repo models.Repository
	err = tx.QueryRow(
		`SELECT id, path, name, created_at FROM repositories WHERE path = ?`, path,
	).Scan(&repo.ID, &repo.Path, &repo.Name, &repo.CreatedAt)
	if err != nil {
		return models.Repository{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.Repository{}, err
	}
	return repo, nil
}

func (s *Store) GetRepos() ([]models.Repository, error) {
	rows, err := s.db.Query(`SELECT id, path, name, created_at FROM repositories ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []models.Repository
	for rows.Next() {
		var r models.Repository
		if err := rows.Scan(&r.ID, &r.Path, &r.Name, &r.CreatedAt); err != nil {
			return nil, err
		}
		repos = append(repos, r)
	}
	return repos, rows.Err()
}

func (s *Store) DeleteRepo(id int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var path string
	if err := tx.QueryRow(`SELECT path FROM repositories WHERE id = ?`, id).Scan(&path); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM sessions WHERE repo_path = ?`, path); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM repositories WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// Session CRUD

func (s *Store) UpsertSession(sess models.Session) error {
	_, err := s.db.Exec(`
		INSERT INTO sessions (
			repo_path, checkpoint_id, session_id, agent, branch, created_at, prompt,
			agent_lines, human_added, human_modified, human_removed,
			total_committed, agent_percentage,
			input_tokens, output_tokens, api_call_count
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
			repo_path=excluded.repo_path,
			agent=excluded.agent, branch=excluded.branch,
			created_at=excluded.created_at, prompt=excluded.prompt,
			agent_lines=excluded.agent_lines, human_added=excluded.human_added,
			human_modified=excluded.human_modified, human_removed=excluded.human_removed,
			total_committed=excluded.total_committed, agent_percentage=excluded.agent_percentage,
			input_tokens=excluded.input_tokens, output_tokens=excluded.output_tokens,
			api_call_count=excluded.api_call_count
	`,
		sess.RepoPath, sess.CheckpointID, sess.SessionID, sess.Agent, sess.Branch, sess.CreatedAt, sess.Prompt,
		sess.AgentLines, sess.HumanAdded, sess.HumanModified, sess.HumanRemoved,
		sess.TotalCommitted, sess.AgentPercentage,
		sess.InputTokens, sess.OutputTokens, sess.APICallCount,
	)
	return err
}

func (s *Store) GetDailyStats(repoPath string) ([]models.DailyStat, error) {
	rows, err := s.db.Query(`
		SELECT
			substr(created_at, 1, 10) AS date,
			SUM(agent_lines) AS agent_lines,
			SUM(human_added) AS human_lines,
			SUM(agent_lines) + SUM(human_added) AS total_lines,
			CASE WHEN SUM(agent_lines) + SUM(human_added) > 0
				THEN ROUND(CAST(SUM(agent_lines) AS REAL) / (SUM(agent_lines) + SUM(human_added)) * 100, 1)
				ELSE 0
			END AS agent_percentage,
			COUNT(*) AS session_count
		FROM sessions
		WHERE (? = '' OR repo_path = ?)
		GROUP BY substr(created_at, 1, 10)
		ORDER BY date
	`, repoPath, repoPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []models.DailyStat
	for rows.Next() {
		var s models.DailyStat
		if err := rows.Scan(&s.Date, &s.AgentLines, &s.HumanLines, &s.TotalLines, &s.AgentPercentage, &s.SessionCount); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

func (s *Store) GetSessions(repoPath string) ([]models.Session, error) {
	rows, err := s.db.Query(`
		SELECT id, repo_path, checkpoint_id, session_id, agent, branch, created_at, prompt,
			agent_lines, human_added, human_modified, human_removed,
			total_committed, agent_percentage,
			input_tokens, output_tokens, api_call_count
		FROM sessions
		WHERE (? = '' OR repo_path = ?)
		ORDER BY created_at DESC
	`, repoPath, repoPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []models.Session
	for rows.Next() {
		var s models.Session
		if err := rows.Scan(
			&s.ID, &s.RepoPath, &s.CheckpointID, &s.SessionID, &s.Agent, &s.Branch, &s.CreatedAt, &s.Prompt,
			&s.AgentLines, &s.HumanAdded, &s.HumanModified, &s.HumanRemoved,
			&s.TotalCommitted, &s.AgentPercentage,
			&s.InputTokens, &s.OutputTokens, &s.APICallCount,
		); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (s *Store) SessionExists(sessionID string) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE session_id = ?", sessionID).Scan(&count)
	return count > 0, err
}

func (s *Store) Close() error {
	return s.db.Close()
}
