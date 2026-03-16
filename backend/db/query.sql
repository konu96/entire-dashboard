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
