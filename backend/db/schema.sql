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
