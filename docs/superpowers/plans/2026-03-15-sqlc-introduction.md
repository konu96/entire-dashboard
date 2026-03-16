# sqlc 導入 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 手書き SQL を sqlc コード生成に全面置換し、SQLite ドライバーを Pure Go に変更、goose マイグレーションを導入する

**Architecture:** `db/schema.sql` と `db/query.sql` から sqlc が型安全な Go コードを `db/sqlc/` に生成。`db/db.go` の Store が sqlc Queries をラップし、トランザクションや型変換を担当。goose が `go:embed` 経由でマイグレーションを実行。

**Tech Stack:** sqlc, modernc.org/sqlite, goose v3, Go 1.25

**Spec:** `docs/superpowers/specs/2026-03-15-sqlc-introduction-design.md`

---

## File Structure

| File | Action | Responsibility |
|---|---|---|
| `backend/sqlc.yaml` | Create | sqlc 設定 |
| `backend/db/schema.sql` | Create | スキーマ定義（sqlc 用） |
| `backend/db/query.sql` | Create | クエリ定義（sqlc 用） |
| `backend/db/sqlc/db.go` | Generate | DBTX インターフェース, Queries 構造体 |
| `backend/db/sqlc/models.go` | Generate | Repository, Session 構造体 |
| `backend/db/sqlc/query.sql.go` | Generate | クエリメソッド |
| `backend/db/migrations/001_initial.sql` | Create | goose マイグレーション |
| `backend/db/db.go` | Rewrite | Store（sqlc Queries ラッパー + goose マイグレーション） |
| `backend/models/models.go` | Modify | Repository, DailyStat 削除。Session は残す |
| `backend/handlers/handlers.go` | Modify | sqlc 生成型への変換に更新 |
| `backend/main.go` | Modify | Store.AddRepo の返り値型変更に対応 |
| `backend/go.mod` | Modify | 依存追加・削除 |
| `Makefile` | Modify | sqlc generate コマンド追加 |

---

## Chunk 1: 基盤セットアップ（sqlc 設定, SQL ファイル, 依存変更）

### Task 1: sqlc インストール確認と依存変更

**Files:**
- Modify: `backend/go.mod`

- [ ] **Step 1: sqlc がインストールされているか確認**

```bash
cd backend && sqlc version
```

インストールされていない場合:
```bash
brew install sqlc
```

- [ ] **Step 2: SQLite ドライバーを modernc.org/sqlite に変更し、goose を追加**

```bash
cd backend && go get modernc.org/sqlite && go get github.com/pressly/goose/v3
```

- [ ] **Step 3: mattn/go-sqlite3 を削除**

```bash
cd backend && go mod edit -droprequire github.com/mattn/go-sqlite3 && go mod tidy
```

- [ ] **Step 4: コミット**

```bash
git add backend/go.mod backend/go.sum
git commit -m "chore: switch to modernc/sqlite and add goose dependency"
```

---

### Task 2: sqlc 設定ファイルとスキーマ定義を作成

**Files:**
- Create: `backend/sqlc.yaml`
- Create: `backend/db/schema.sql`

- [ ] **Step 1: sqlc.yaml を作成**

```yaml
version: "2"
sql:
  - engine: "sqlite"
    queries: "db/query.sql"
    schema: "db/schema.sql"
    gen:
      go:
        package: "sqlc"
        out: "db/sqlc"
```

- [ ] **Step 2: db/schema.sql を作成**

```sql
CREATE TABLE repositories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE sessions (
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

CREATE INDEX idx_sessions_created_at ON sessions(created_at);
CREATE INDEX idx_sessions_repo_path ON sessions(repo_path);
```

- [ ] **Step 3: コミット**

```bash
git add backend/sqlc.yaml backend/db/schema.sql
git commit -m "feat: add sqlc config and schema definition"
```

---

### Task 3: クエリ定義を作成し sqlc generate を実行

**Files:**
- Create: `backend/db/query.sql`
- Generate: `backend/db/sqlc/db.go`, `backend/db/sqlc/models.go`, `backend/db/sqlc/query.sql.go`

- [ ] **Step 1: db/query.sql を作成**

```sql
-- name: GetRepos :many
SELECT id, path, name, created_at FROM repositories ORDER BY created_at;

-- name: UpsertRepo :one
INSERT INTO repositories (path, name) VALUES (?, ?)
ON CONFLICT(path) DO UPDATE SET name=excluded.name
RETURNING id, path, name, created_at;

-- name: GetRepoPath :one
SELECT path FROM repositories WHERE id = ?;

-- name: DeleteRepo :exec
DELETE FROM repositories WHERE id = ?;

-- name: DeleteSessionsByRepoPath :exec
DELETE FROM sessions WHERE repo_path = ?;

-- name: UpsertSession :exec
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
    api_call_count=excluded.api_call_count;

-- name: GetDailyStats :many
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
ORDER BY date;

-- name: GetSessions :many
SELECT id, repo_path, checkpoint_id, session_id, agent, branch, created_at, prompt,
    agent_lines, human_added, human_modified, human_removed,
    total_committed, agent_percentage,
    input_tokens, output_tokens, api_call_count
FROM sessions
WHERE (? = '' OR repo_path = ?)
ORDER BY created_at DESC;

-- name: SessionExists :one
SELECT COUNT(*) FROM sessions WHERE session_id = ?;
```

- [ ] **Step 2: sqlc generate を実行**

```bash
cd backend && sqlc generate
```

Expected: `db/sqlc/` ディレクトリに `db.go`, `models.go`, `query.sql.go` が生成される

- [ ] **Step 3: 生成されたコードを確認**

生成された `db/sqlc/models.go` を確認し、`Repository` と `Session` の構造体フィールドが期待通りか検証する。特に:
- `Repository` に `ID`, `Path`, `Name`, `CreatedAt` フィールドがあること
- `Session` に全16カラムのフィールドがあること

- [ ] **Step 4: コミット**

```bash
git add backend/db/query.sql backend/db/sqlc/
git commit -m "feat: add sqlc query definitions and generate Go code"
```

---

### Task 4: goose マイグレーションファイルを作成

**Files:**
- Create: `backend/db/migrations/001_initial.sql`

- [ ] **Step 1: migrations ディレクトリを作成し、001_initial.sql を作成**

```sql
-- +goose Up
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

-- +goose Down
DROP INDEX IF EXISTS idx_sessions_repo_path;
DROP INDEX IF EXISTS idx_sessions_created_at;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS repositories;
```

- [ ] **Step 2: コミット**

```bash
git add backend/db/migrations/001_initial.sql
git commit -m "feat: add goose migration for initial schema"
```

---

## Chunk 2: Store 層の書き換え

### Task 5: db/db.go を sqlc Queries ラッパーに書き換え

**Files:**
- Rewrite: `backend/db/db.go`

- [ ] **Step 1: db/db.go を全面書き換え**

```go
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
			// 既存 DB: goose バージョンテーブルを作成し、バージョン 1 をスタンプ
			if _, err := goose.EnsureDBVersion(db); err != nil {
				return fmt.Errorf("ensure db version: %w", err)
			}
			if err := goose.UpTo(db, "migrations", 0); err != nil {
				// goose テーブル初期化のみ
			}
			// バージョン 1 を適用済みとしてマーク
			if _, err := db.Exec("INSERT INTO goose_db_version (version_id, is_applied) VALUES (1, true)"); err != nil {
				return fmt.Errorf("stamp version: %w", err)
			}
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

func (s *Store) GetSessions(repoPath string) ([]sqlc.GetSessionsRow, error) {
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
```

**注意:** sqlc が生成する型名（`UpsertRepoParams`, `GetDailyStatsParams`, `GetDailyStatsRow`, `GetSessionsRow` 等）は `sqlc generate` の出力を確認して正確な名前に合わせること。特に:
- `GetDailyStatsParams` の `Column1` フィールド名は sqlc が自動命名するため異なる可能性がある
- `int64` vs `int` の変換が必要（sqlc は SQLite の INTEGER を `int64` に変換する）

- [ ] **Step 2: コンパイルが通るか確認**

```bash
cd backend && go build ./...
```

この時点ではまだ `handlers/handlers.go` と `main.go` が古い型を参照しているためエラーになる。次の Task で修正する。

- [ ] **Step 3: コミット**

```bash
git add backend/db/db.go
git commit -m "feat: rewrite db.Store as sqlc Queries wrapper with goose migrations"
```

---

## Chunk 3: ハンドラ層と models の更新

### Task 6: models/models.go から DB 構造体を削除

**Files:**
- Modify: `backend/models/models.go`

- [ ] **Step 1: Repository 構造体と DailyStat 構造体を削除**

`models/models.go` から以下を削除:
- `Repository` 構造体（行 4-9）
- `DailyStat` 構造体（行 33-40）

`Session` 構造体は `git/reader.go` が使うため残す。

- [ ] **Step 2: コミット**

```bash
git add backend/models/models.go
git commit -m "refactor: remove Repository and DailyStat from models (now generated by sqlc)"
```

---

### Task 7: handlers/handlers.go を sqlc 生成型に更新

**Files:**
- Modify: `backend/handlers/handlers.go`

- [ ] **Step 1: import を更新**

```go
import (
	"context"
	"entire-dashboard/db"
	"entire-dashboard/db/sqlc"  // 必須: Store の返り値型が sqlc.Repository 等のため
	"entire-dashboard/generated"
	gitreader "entire-dashboard/git"
	"log"
	"path/filepath"
)
```

`db.Store` の返り値型が `sqlc.Repository`, `sqlc.GetDailyStatsRow`, `sqlc.GetSessionsRow` 等のため、`entire-dashboard/db/sqlc` の import は必須。

- [ ] **Step 2: GetRepos を更新**

`r.ID` の型が `int64` に変わるため、`generated.Repository` への変換を調整:

```go
func (h *Handler) GetRepos(ctx context.Context) (generated.GetReposRes, error) {
	repos, err := h.store.GetRepos()
	if err != nil {
		return &generated.ErrorResponse{Message: err.Error()}, nil
	}
	result := make(generated.GetReposOKApplicationJSON, 0, len(repos))
	for _, r := range repos {
		result = append(result, generated.Repository{
			ID:        int(r.ID),
			Path:      r.Path,
			Name:      r.Name,
			CreatedAt: r.CreatedAt,
		})
	}
	return &result, nil
}
```

- [ ] **Step 3: AddRepo を更新**

`store.AddRepo` の返り値が `sqlc.Repository` に変わる:

```go
func (h *Handler) AddRepo(ctx context.Context, req *generated.AddRepoRequest) (generated.AddRepoRes, error) {
	if req.Path == "" {
		return &generated.AddRepoBadRequest{Message: "path is required"}, nil
	}

	absPath, err := filepath.Abs(req.Path)
	if err != nil {
		return &generated.AddRepoBadRequest{Message: "invalid path: " + err.Error()}, nil
	}

	name := filepath.Base(absPath)
	repo, err := h.store.AddRepo(absPath, name)
	if err != nil {
		return &generated.AddRepoInternalServerError{Message: "failed to add repo: " + err.Error()}, nil
	}
	return &generated.Repository{
		ID:        int(repo.ID),
		Path:      repo.Path,
		Name:      repo.Name,
		CreatedAt: repo.CreatedAt,
	}, nil
}
```

- [ ] **Step 4: DeleteRepo を更新**

`params.ID` が `int` だが `store.DeleteRepo` が `int64` を受ける:

```go
func (h *Handler) DeleteRepo(ctx context.Context, params generated.DeleteRepoParams) (generated.DeleteRepoRes, error) {
	if err := h.store.DeleteRepo(int64(params.ID)); err != nil {
		return &generated.DeleteRepoInternalServerError{Message: "failed to delete repo: " + err.Error()}, nil
	}
	return &generated.DeleteRepoResponse{Status: "ok"}, nil
}
```

- [ ] **Step 5: GetDailyStats を更新**

返り値が `[]sqlc.GetDailyStatsRow` に変わる。フィールド名は sqlc の生成結果に合わせる:

```go
func (h *Handler) GetDailyStats(ctx context.Context, params generated.GetDailyStatsParams) (generated.GetDailyStatsRes, error) {
	repoPath := params.Repo.Or("")
	stats, err := h.store.GetDailyStats(repoPath)
	if err != nil {
		return &generated.ErrorResponse{Message: err.Error()}, nil
	}
	result := make(generated.GetDailyStatsOKApplicationJSON, 0, len(stats))
	for _, s := range stats {
		result = append(result, generated.DailyStat{
			Date:            s.Date,
			AgentLines:      int(s.AgentLines),
			HumanLines:      int(s.HumanLines),
			TotalLines:      int(s.TotalLines),
			AgentPercentage: s.AgentPercentage,
			SessionCount:    int(s.SessionCount),
		})
	}
	return &result, nil
}
```

**注意:** sqlc 生成の `GetDailyStatsRow` のフィールド名とフィールド型は生成結果を確認すること。`SUM()` や `COUNT()` の結果が `interface{}` になる場合は、`sqlc.yaml` に型オーバーライドを追加する:

```yaml
version: "2"
sql:
  - engine: "sqlite"
    queries: "db/query.sql"
    schema: "db/schema.sql"
    gen:
      go:
        package: "sqlc"
        out: "db/sqlc"
        overrides:
          - column: "GetDailyStats.agent_lines"
            go_type: "int64"
          - column: "GetDailyStats.human_lines"
            go_type: "int64"
          - column: "GetDailyStats.total_lines"
            go_type: "int64"
          - column: "GetDailyStats.agent_percentage"
            go_type: "float64"
          - column: "GetDailyStats.session_count"
            go_type: "int64"
```

型オーバーライドが必要かは `sqlc generate` の出力結果を見て判断する。`int64` や `float64` が生成されていれば追加不要。

- [ ] **Step 6: GetSessions を更新**

返り値が `[]sqlc.GetSessionsRow` に変わる:

```go
func (h *Handler) GetSessions(ctx context.Context, params generated.GetSessionsParams) (generated.GetSessionsRes, error) {
	repoPath := params.Repo.Or("")
	sessions, err := h.store.GetSessions(repoPath)
	if err != nil {
		return &generated.ErrorResponse{Message: err.Error()}, nil
	}
	result := make(generated.GetSessionsOKApplicationJSON, 0, len(sessions))
	for _, s := range sessions {
		result = append(result, generated.Session{
			ID:              int(s.ID),
			RepoPath:        s.RepoPath,
			CheckpointID:    s.CheckpointID,
			SessionID:       s.SessionID,
			Agent:           s.Agent,
			Branch:          s.Branch,
			CreatedAt:       s.CreatedAt,
			Prompt:          s.Prompt,
			AgentLines:      int(s.AgentLines),
			HumanAdded:      int(s.HumanAdded),
			HumanModified:   int(s.HumanModified),
			HumanRemoved:    int(s.HumanRemoved),
			TotalCommitted:  int(s.TotalCommitted),
			AgentPercentage: s.AgentPercentage,
			InputTokens:     int(s.InputTokens),
			OutputTokens:    int(s.OutputTokens),
			APICallCount:    int(s.ApiCallCount),
		})
	}
	return &result, nil
}
```

- [ ] **Step 7: SyncData を更新**

`GetRepos` の返り値型変更に対応:

```go
// SyncData 内の repos ループ部分
repoList, err := h.store.GetRepos()
// ...
for _, r := range repoList {
    repos = append(repos, r.Path)
}
```

この部分は元々 `r.Path` でアクセスしているので、`sqlc.Repository` でも同じフィールド名なら変更不要。

- [ ] **Step 8: コミット**

```bash
git add backend/handlers/handlers.go
git commit -m "refactor: update handlers to use sqlc generated types"
```

---

### Task 8: main.go を更新

**Files:**
- Modify: `backend/main.go:43`

- [ ] **Step 1: AddRepo の返り値型に対応**

`main.go` の行 43 で `store.AddRepo` を呼んでいるが、返り値の型が `models.Repository` → `sqlc.Repository` に変わる。使っている箇所は `_` で捨てているだけなので、コード変更は不要のはず。ただしコンパイルを確認する。

- [ ] **Step 2: ビルド確認**

```bash
cd backend && go build ./...
```

Expected: エラーなし

- [ ] **Step 3: コミット（変更がある場合のみ）**

```bash
git add backend/main.go
git commit -m "refactor: update main.go for sqlc type changes"
```

---

## Chunk 4: Makefile 更新と最終確認

### Task 9: Makefile に sqlc generate を追加

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: generate-sqlc ターゲットを追加**

```makefile
.PHONY: generate generate-backend generate-frontend generate-sqlc

generate: generate-sqlc generate-backend generate-frontend

generate-sqlc:
	cd backend && sqlc generate

generate-backend:
	cd backend && go generate ./...

generate-frontend:
	cd frontend && npm run generate
```

**注意:** `generate-sqlc` を最初に実行する。`generate-backend`（`go generate`）は sqlc 生成コードに依存する可能性があるため。

- [ ] **Step 2: コミット**

```bash
git add Makefile
git commit -m "chore: add sqlc generate to Makefile"
```

---

### Task 10: 全体ビルド・起動確認

- [ ] **Step 1: go mod tidy**

```bash
cd backend && go mod tidy
```

- [ ] **Step 2: ビルド**

```bash
cd backend && go build ./...
```

Expected: エラーなし

- [ ] **Step 3: 起動テスト**

```bash
cd backend && go run main.go --port 8081 &
sleep 2
curl -s http://localhost:8081/api/repos | head -c 200
kill %1
```

Expected: `[]` または登録済みリポジトリの JSON が返る

- [ ] **Step 4: sqlc generate の冪等性確認**

```bash
cd backend && sqlc generate && git diff --stat
```

Expected: 差分なし（生成コードが既にコミット済みと一致）

- [ ] **Step 5: 最終コミット（go.mod/go.sum の変更がある場合）**

```bash
cd backend && go mod tidy
git add backend/go.mod backend/go.sum
git commit -m "chore: tidy go modules after sqlc migration"
```
