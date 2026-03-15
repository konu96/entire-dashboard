# sqlc 導入設計

## 概要

手書き SQL（`db/db.go`）を sqlc によるコード生成に全面置換する。合わせて SQLite ドライバーを `modernc.org/sqlite`（Pure Go）に変更し、マイグレーションツールとして goose を導入する。

## 動機

- **型安全性**: 手書きの `rows.Scan` でのカラム不一致やフィールド漏れを防ぐ
- **将来の拡張への備え**: クエリ追加時に `.sql` に書いて `sqlc generate` するだけのワークフローを確立

## 決定事項

| 項目 | 決定 |
|---|---|
| sqlc エンジン | sqlite |
| SQLite ドライバー | `modernc.org/sqlite`（Pure Go、CGO 不要） |
| マイグレーション | goose（SQL ベース） |
| 生成コード配置 | `backend/db/sqlc/` |
| 移行方式 | フル置換（手書き SQL を全て sqlc 生成に置換） |

## ディレクトリ構成

```
backend/
  db/
    db.go              ← Store 構造体（sqlc Queries をラップ、トランザクション処理）
    sqlc/
      db.go            ← sqlc 生成: DBTX インターフェース, Queries 構造体
      models.go        ← sqlc 生成: Repository, Session 構造体
      query.sql.go     ← sqlc 生成: クエリメソッド
    migrations/
      001_initial.sql  ← goose マイグレーション（CREATE TABLE）
    query.sql          ← sqlc 用クエリ定義
    schema.sql         ← sqlc 用スキーマ定義
  sqlc.yaml            ← sqlc 設定ファイル
  models/models.go     ← 非 DB 構造体のみ残す（CheckpointMeta 等）
```

## sqlc 設定

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

## スキーマ定義（db/schema.sql）

現在の `migrate()` 関数と同じスキーマ:

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

## クエリ定義（db/query.sql）

現在の7メソッドに対応する9クエリ（DeleteRepo のトランザクション分割含む）:

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

## Store 層の設計

### 変更前

`db.Store` が `*sql.DB` を直接操作し、手書き SQL を実行。返り値は `models.Repository` 等。

### 変更後

`db.Store` が sqlc 生成の `*sqlc.Queries` をラップ。

```go
type Store struct {
    db      *sql.DB
    queries *sqlc.Queries
}
```

- **単純なクエリ**: `Queries` のメソッドに委譲（GetRepos, UpsertSession）
- **トランザクション**: `WithTx` を使って Store のメソッドとして実装（DeleteRepo）
- **AddRepo**: `RETURNING` により単一クエリに簡素化（現在の TX + INSERT + SELECT が不要に）
- **GetDailyStats / GetSessions**: sqlc が2引数のパラメータ構造体を生成する（`WHERE (? = '' OR repo_path = ?)` の `?` が2つ）。Store ラッパーで `repoPath` を両フィールドにセットする
- **SessionExists**: sqlc は `int64` を返す。Store ラッパーで `count > 0` の `bool` 変換を行う

### 返り値の型

ハンドラ層は `sqlc.Repository` / `sqlc.Session` 等を受け取り、OpenAPI 生成型（`generated.Repository` 等）に変換する。現在の `models.Repository` → `generated.Repository` 変換と同じパターン。

## models/models.go の変更

DB 関連構造体を削除:
- ~~`Repository`~~（sqlc 生成に移行）
- ~~`DailyStat`~~（sqlc 生成に移行）

`Session` 構造体は残す:
- `git/reader.go` が `models.Session` を構築して返す
- `db/sqlc` パッケージへの依存は循環インポートのリスクがあるため、`models.Session` を中間型として維持
- `Store.UpsertSession` は `models.Session` を受け取り、内部で `sqlc.UpsertSessionParams` に変換する

残す構造体（Git reader 用、非 DB）:
- `CheckpointMeta`
- `SessionRef`
- `SessionMeta`
- `TokenUsage`
- `Attribution`

## goose マイグレーション

### db/migrations/001_initial.sql

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

### 既存 DB との互換性

既に `~/.entire-dashboard/dashboard.db` が存在するユーザーへの対応:

- `001_initial.sql` では `CREATE TABLE IF NOT EXISTS` を使用する
- `db.New()` 起動時に `goose_db_version` テーブルが存在しない場合、goose のバージョンを 1 にセットしてから `goose.Up` を実行する（既存テーブルを再作成しない）
- これにより既存ユーザーのデータが保持される

### 起動時の実行

`go:embed` でマイグレーションファイルを埋め込み、`goose.Up(db, embedFS)` で実行する。これにより実行時のカレントディレクトリに依存しない。

```go
//go:embed migrations/*.sql
var embedMigrations embed.FS

func runMigrations(db *sql.DB) error {
    goose.SetBaseFS(embedMigrations)
    goose.SetDialect("sqlite3")
    return goose.Up(db, "migrations")
}
```

### WAL モードの設定

`modernc.org/sqlite` では DSN パラメータが異なる。接続後に `PRAGMA journal_mode=WAL` を実行して WAL モードを有効化する。

```go
db, err := sql.Open("sqlite", path)
if err != nil {
    return nil, err
}
if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
    return nil, fmt.Errorf("set WAL mode: %w", err)
}
```

## 依存関係の変更

### 追加

- `modernc.org/sqlite` — Pure Go SQLite ドライバー
- `github.com/pressly/goose/v3` — マイグレーションツール

### 削除

- `github.com/mattn/go-sqlite3` — CGO 依存 SQLite ドライバー

### 開発ツール

- `github.com/sqlc-dev/sqlc` — `go install` または `brew install sqlc` でインストール

## 影響範囲

| ファイル | 変更内容 |
|---|---|
| `backend/db/db.go` | 全面書き換え: sqlc Queries ラッパー + goose マイグレーション |
| `backend/db/schema.sql` | 新規: スキーマ定義 |
| `backend/db/query.sql` | 新規: クエリ定義 |
| `backend/db/sqlc/` | 新規: sqlc 生成コード |
| `backend/db/migrations/001_initial.sql` | 新規: goose マイグレーション |
| `backend/sqlc.yaml` | 新規: sqlc 設定 |
| `backend/models/models.go` | DB 構造体削除（Repository, DailyStat）。Session は git reader 用に残す |
| `backend/handlers/handlers.go` | `models.*` → `sqlc.*` 型に変更（Session 関連は models.Session → sqlc 変換を Store 内で吸収） |
| `backend/git/reader.go` | 変更なし（引き続き `models.Session` を返す） |
| `backend/go.mod` | 依存追加・削除 |
| `backend/Makefile` | `sqlc generate` コマンド追加 |
