package db

import (
	"context"
	"database/sql"
	"embed"
	"entire-dashboard/db/sqlc"
	"entire-dashboard/models"
	"fmt"
	"strings"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type Store struct {
	db      *sql.DB
	queries *sqlc.Queries
}

func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db, queries: sqlc.New(db)}, nil
}

func runMigrations(db *sql.DB) error {
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	// 既存 DB との互換性: goose_db_version テーブルがなく、
	// repositories テーブルが既に存在する場合は、
	// マイグレーション 1 を適用済みとしてマークする
	var gooseTableExists int
	_ = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='goose_db_version'").Scan(&gooseTableExists)
	if gooseTableExists == 0 {
		var repoTableExists int
		_ = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='repositories'").Scan(&repoTableExists)
		if repoTableExists > 0 {
			if err := goose.Up(db, "migrations"); err != nil {
				// goose テーブルが初期化され、001_initial.sql は IF NOT EXISTS なので安全
				return fmt.Errorf("initial goose up: %w", err)
			}
			return nil
		}
	}

	return goose.Up(db, "migrations")
}

// Repository operations

func (s *Store) AddRepo(path, name string) (sqlc.Repository, error) {
	if strings.TrimSpace(path) == "" {
		return sqlc.Repository{}, fmt.Errorf("repo path is required")
	}
	return s.queries.UpsertRepo(context.Background(), sqlc.UpsertRepoParams{
		Path: path,
		Name: name,
	})
}

func (s *Store) GetRepos() ([]sqlc.Repository, error) {
	return s.queries.GetRepos(context.Background())
}

func (s *Store) DeleteRepo(id int64) error {
	ctx := context.Background()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := s.queries.WithTx(tx)
	path, err := qtx.GetRepoPath(ctx, id)
	if err != nil {
		return err
	}
	if err := qtx.DeleteSessionsByRepoPath(ctx, path); err != nil {
		return err
	}
	if err := qtx.DeleteRepo(ctx, id); err != nil {
		return err
	}
	return tx.Commit()
}

// Session operations

func (s *Store) UpsertSession(sess models.Session) error {
	return s.queries.UpsertSession(context.Background(), sqlc.UpsertSessionParams{
		RepoPath:        sess.RepoPath,
		CheckpointID:    sess.CheckpointID,
		SessionID:       sess.SessionID,
		Agent:           sess.Agent,
		Branch:          sess.Branch,
		CreatedAt:       sess.CreatedAt,
		Prompt:          sess.Prompt,
		AgentLines:      int64(sess.AgentLines),
		HumanAdded:      int64(sess.HumanAdded),
		HumanModified:   int64(sess.HumanModified),
		HumanRemoved:    int64(sess.HumanRemoved),
		TotalCommitted:  int64(sess.TotalCommitted),
		AgentPercentage: sess.AgentPercentage,
		InputTokens:     int64(sess.InputTokens),
		OutputTokens:    int64(sess.OutputTokens),
		ApiCallCount:    int64(sess.APICallCount),
	})
}

func (s *Store) GetDailyStats(repoPath string) ([]sqlc.GetDailyStatsRow, error) {
	return s.queries.GetDailyStats(context.Background(), sqlc.GetDailyStatsParams{
		Column1:  repoPath,
		RepoPath: repoPath,
	})
}

func (s *Store) GetSessions(repoPath string) ([]sqlc.Session, error) {
	return s.queries.GetSessions(context.Background(), sqlc.GetSessionsParams{
		Column1:  repoPath,
		RepoPath: repoPath,
	})
}

func (s *Store) SessionExists(sessionID string) (bool, error) {
	count, err := s.queries.SessionExists(context.Background(), sessionID)
	return count > 0, err
}

func (s *Store) Close() error {
	return s.db.Close()
}
