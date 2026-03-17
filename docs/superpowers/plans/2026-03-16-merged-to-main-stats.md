# Merged-to-Main Stats Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show the percentage of AI-generated code that made it into the main branch, with per-session merged/unmerged status.

**Architecture:** Add `merged_to_main` column to sessions table. During sync, run `git branch --merged main` to get merged branch list, then tag each session accordingly. Expose via existing Session schema + new KPI card in frontend.

**Tech Stack:** Go 1.23 (ogen codegen), SQLite, React 19, TypeScript, Vite, Recharts, orval (frontend codegen)

**Code generation:** Backend uses `go generate ./...` with ogen. Frontend uses `npm run generate` with orval. Both read from `api/openapi.yml`. The `generated/` directories are auto-generated and must not be hand-edited.

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `api/openapi.yml` | Modify | Add `merged_to_main` to Session schema |
| `backend/models/models.go` | Modify | Add `MergedToMain` field to Session struct |
| `backend/git/reader.go` | Modify | Add `GetMergedBranches` function |
| `backend/db/db.go` | Modify | Add migration, update UpsertSession, add `GetMergedStats` |
| `backend/handlers/handlers.go` | Modify | Update SyncData to set merged_to_main, update GetSessions mapping |
| `backend/generated/*` | Regenerate | Auto-generated from openapi.yml |
| `frontend/src/api/generated/*` | Regenerate | Auto-generated from openapi.yml |
| `frontend/src/App.tsx` | Modify | Add "Merged AI Lines" KPI card |
| `frontend/src/App.css` | Modify | Add merged badge styles |
| `frontend/src/components/SessionTimeline.tsx` | Modify | Add Merged/Unmerged badge column |

---

## Chunk 1: Backend Data Layer

### Task 1: OpenAPI Schema Update

**Files:**
- Modify: `api/openapi.yml`

- [ ] **Step 1: Add `merged_to_main` to Session schema**

In `api/openapi.yml`, add `merged_to_main` to the Session schema properties and required list:

```yaml
# In components.schemas.Session.required, add:
        - merged_to_main

# In components.schemas.Session.properties, add after api_call_count:
        merged_to_main:
          type: boolean
```

- [ ] **Step 2: Commit**

```bash
git add api/openapi.yml
git commit -m "feat: add merged_to_main field to Session schema in OpenAPI"
```

---

### Task 2: Backend Model Update

**Files:**
- Modify: `backend/models/models.go`

- [ ] **Step 1: Add MergedToMain field to Session struct**

Add to the Session struct after `APICallCount`:

```go
MergedToMain    bool    `json:"merged_to_main"`
```

- [ ] **Step 2: Commit**

```bash
git add backend/models/models.go
git commit -m "feat: add MergedToMain field to Session model"
```

---

### Task 3: Git Merged Branch Detection

**Files:**
- Modify: `backend/git/reader.go`

- [ ] **Step 1: Add GetMergedBranches function**

Add this function to `backend/git/reader.go`:

```go
// GetMergedBranches returns a set of branch names that have been merged into main.
func GetMergedBranches(repoPath string) (map[string]bool, error) {
	out, err := exec.Command("git", "-C", repoPath, "branch", "--merged", "main").Output()
	if err != nil {
		return nil, fmt.Errorf("git branch --merged main: %w", err)
	}
	merged := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		branch := strings.TrimSpace(line)
		// Remove the "* " prefix for the current branch
		branch = strings.TrimPrefix(branch, "* ")
		if branch != "" {
			merged[branch] = true
		}
	}
	return merged, nil
}
```

Note: This is a package-level function (not a method on Reader) since it doesn't need the shadow branch.

- [ ] **Step 2: Verify it compiles**

```bash
cd backend && go build ./...
```
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add backend/git/reader.go
git commit -m "feat: add GetMergedBranches function for detecting merged branches"
```

---

### Task 4: Database Migration and Queries

**Files:**
- Modify: `backend/db/db.go`

- [ ] **Step 1: Add ALTER TABLE migration in migrate function**

Add after the existing `CREATE INDEX` statements, still inside the same `db.Exec` call:

```sql
-- SQLite allows ALTER TABLE ADD COLUMN if the column doesn't exist yet.
-- We use a separate Exec to handle the "duplicate column" error gracefully.
```

Actually, since SQLite's `ALTER TABLE ADD COLUMN` will error if the column already exists, add a separate migration call after the existing `migrate` function's `db.Exec`. Add this at the end of the `migrate` function, before the `return`:

```go
// Add merged_to_main column (idempotent: ignore error if already exists)
_, _ = db.Exec(`ALTER TABLE sessions ADD COLUMN merged_to_main INTEGER NOT NULL DEFAULT 0`)
```

- [ ] **Step 2: Update UpsertSession to include merged_to_main**

Update the `UpsertSession` method. Add `merged_to_main` to the INSERT columns, VALUES placeholders, and ON CONFLICT SET clause:

```go
func (s *Store) UpsertSession(sess models.Session) error {
	mergedInt := 0
	if sess.MergedToMain {
		mergedInt = 1
	}
	_, err := s.db.Exec(`
		INSERT INTO sessions (
			repo_path, checkpoint_id, session_id, agent, branch, created_at, prompt,
			agent_lines, human_added, human_modified, human_removed,
			total_committed, agent_percentage,
			input_tokens, output_tokens, api_call_count,
			merged_to_main
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
			repo_path=excluded.repo_path,
			agent=excluded.agent, branch=excluded.branch,
			created_at=excluded.created_at, prompt=excluded.prompt,
			agent_lines=excluded.agent_lines, human_added=excluded.human_added,
			human_modified=excluded.human_modified, human_removed=excluded.human_removed,
			total_committed=excluded.total_committed, agent_percentage=excluded.agent_percentage,
			input_tokens=excluded.input_tokens, output_tokens=excluded.output_tokens,
			api_call_count=excluded.api_call_count,
			merged_to_main=excluded.merged_to_main
	`,
		sess.RepoPath, sess.CheckpointID, sess.SessionID, sess.Agent, sess.Branch, sess.CreatedAt, sess.Prompt,
		sess.AgentLines, sess.HumanAdded, sess.HumanModified, sess.HumanRemoved,
		sess.TotalCommitted, sess.AgentPercentage,
		sess.InputTokens, sess.OutputTokens, sess.APICallCount,
		mergedInt,
	)
	return err
}
```

- [ ] **Step 3: Update GetSessions to read merged_to_main**

Update the SELECT query and Scan call in `GetSessions`:

```go
func (s *Store) GetSessions(repoPath string) ([]models.Session, error) {
	rows, err := s.db.Query(`
		SELECT id, repo_path, checkpoint_id, session_id, agent, branch, created_at, prompt,
			agent_lines, human_added, human_modified, human_removed,
			total_committed, agent_percentage,
			input_tokens, output_tokens, api_call_count,
			merged_to_main
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
		var mergedInt int
		if err := rows.Scan(
			&s.ID, &s.RepoPath, &s.CheckpointID, &s.SessionID, &s.Agent, &s.Branch, &s.CreatedAt, &s.Prompt,
			&s.AgentLines, &s.HumanAdded, &s.HumanModified, &s.HumanRemoved,
			&s.TotalCommitted, &s.AgentPercentage,
			&s.InputTokens, &s.OutputTokens, &s.APICallCount,
			&mergedInt,
		); err != nil {
			return nil, err
		}
		s.MergedToMain = mergedInt != 0
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}
```

- [ ] **Step 4: Add UpdateMergedStatus method**

This method bulk-updates `merged_to_main` for all sessions of a repo based on merged branches. Add to `db.go`:

```go
func (s *Store) UpdateMergedStatus(repoPath string, mergedBranches map[string]bool) error {
	rows, err := s.db.Query(
		`SELECT id, branch FROM sessions WHERE repo_path = ?`, repoPath,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	type sessionBranch struct {
		id     int
		branch string
	}
	var items []sessionBranch
	for rows.Next() {
		var sb sessionBranch
		if err := rows.Scan(&sb.id, &sb.branch); err != nil {
			return err
		}
		items = append(items, sb)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, sb := range items {
		merged := 0
		if mergedBranches[sb.branch] {
			merged = 1
		}
		if _, err := s.db.Exec(
			`UPDATE sessions SET merged_to_main = ? WHERE id = ?`, merged, sb.id,
		); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 5: Verify it compiles**

```bash
cd backend && go build ./...
```
Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add backend/db/db.go
git commit -m "feat: add merged_to_main column, migration, and query updates"
```

---

### Task 5: Handler Updates

**Files:**
- Modify: `backend/handlers/handlers.go`

- [ ] **Step 1: Update SyncData to detect merged branches and update status**

In the `SyncData` method, after the existing sync loop, add merged branch detection. Insert this code right before the `return &generated.SyncResponse{...}` line:

```go
	// Update merged_to_main status for all synced repos
	for _, rp := range repos {
		mergedBranches, err := gitreader.GetMergedBranches(rp)
		if err != nil {
			log.Printf("get merged branches error for %s: %v", rp, err)
			continue
		}
		if err := h.store.UpdateMergedStatus(rp, mergedBranches); err != nil {
			log.Printf("update merged status error for %s: %v", rp, err)
		}
	}
```

- [ ] **Step 2: Update GetSessions to map merged_to_main field**

In the `GetSessions` method, add the `MergedToMain` field to the generated Session mapping. Add this line inside the loop where generated.Session is constructed:

```go
MergedToMain:    s.MergedToMain,
```

- [ ] **Step 3: Verify it compiles**

```bash
cd backend && go build ./...
```
Expected: Will fail because `generated.Session` doesn't have `MergedToMain` yet. This is expected — we'll regenerate in Task 6.

- [ ] **Step 4: Commit**

```bash
git add backend/handlers/handlers.go
git commit -m "feat: update SyncData and GetSessions for merged_to_main"
```

---

### Task 6: Regenerate Code

**Files:**
- Regenerate: `backend/generated/*`
- Regenerate: `frontend/src/api/generated/*`

- [ ] **Step 1: Regenerate backend code**

```bash
cd /Users/unokohei/ghq/github.com/konu96/entire-dashboard && make generate-backend
```
Expected: ogen regenerates all files in `backend/generated/`

- [ ] **Step 2: Verify backend compiles**

```bash
cd backend && go build ./...
```
Expected: no errors (generated.Session now has MergedToMain field)

- [ ] **Step 3: Regenerate frontend code**

```bash
cd /Users/unokohei/ghq/github.com/konu96/entire-dashboard && make generate-frontend
```
Expected: orval regenerates frontend API client

- [ ] **Step 4: Commit all generated code**

```bash
git add backend/generated/ frontend/src/api/generated/
git commit -m "chore: regenerate API client code for merged_to_main field"
```

---

## Chunk 2: Frontend UI

### Task 7: KPI Card for Merged AI Lines

**Files:**
- Modify: `frontend/src/App.tsx`

- [ ] **Step 1: Add merged AI stats computation**

After the existing KPI computation block (line ~131 area), add:

```tsx
  // Compute merged KPIs
  const mergedAgentLines = sessions
    .filter((s) => s.merged_to_main)
    .reduce((sum, s) => sum + s.agent_lines, 0);
  const mergedPct =
    totalAgent > 0
      ? ((mergedAgentLines / totalAgent) * 100).toFixed(1)
      : "0.0";
```

- [ ] **Step 2: Add KPI card to the grid**

Add a new KPI card after the existing "Sessions" card, inside the `.kpi-row` div:

```tsx
        <div className="kpi-card">
          <div className="kpi-label">Merged to Main</div>
          <div className="kpi-value">
            {mergedPct}
            <span className="kpi-unit">%</span>
          </div>
        </div>
```

- [ ] **Step 3: Update the grid to accommodate 5 cards**

In `frontend/src/App.css`, update `.kpi-row` grid-template-columns:

```css
.kpi-row {
  display: grid;
  grid-template-columns: 2fr 1fr 1fr 1fr 1fr;
  gap: 16px;
  margin-bottom: 32px;
}
```

- [ ] **Step 4: Verify frontend compiles**

```bash
cd frontend && npm run build
```
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add frontend/src/App.tsx frontend/src/App.css
git commit -m "feat: add Merged to Main KPI card"
```

---

### Task 8: Session Timeline Merged Badge

**Files:**
- Modify: `frontend/src/components/SessionTimeline.tsx`
- Modify: `frontend/src/App.css`

- [ ] **Step 1: Add CSS styles for merged badge**

Add to the end of `frontend/src/App.css`:

```css
/* Merged Badge */
.merged-badge {
  display: inline-block;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 4px;
}

.merged-badge--merged {
  color: #22a06b;
  background: #e6f4ed;
}

.merged-badge--unmerged {
  color: #626264;
  background: #f0f0f3;
}
```

- [ ] **Step 2: Add Status column header**

In `SessionTimeline.tsx`, update the header row to add a "Status" column. Change the header div:

```tsx
        <div className="session-row session-row--header">
          <div>Date</div>
          <div>Branch / Prompt</div>
          <div style={{ textAlign: "center" }}>Status</div>
          <div style={{ textAlign: "right" }}>AI Lines</div>
          <div style={{ textAlign: "right" }}>Human Lines</div>
          <div style={{ textAlign: "right" }}>Total</div>
          <div style={{ textAlign: "right" }}>AI Ratio</div>
        </div>
```

- [ ] **Step 3: Add merged badge to SessionRow**

In the `SessionRow` component, add the badge between the branch/prompt div and the AI Lines div:

```tsx
        <div style={{ textAlign: "center" }}>
          <span className={`merged-badge ${s.merged_to_main ? "merged-badge--merged" : "merged-badge--unmerged"}`}>
            {s.merged_to_main ? "Merged" : "Unmerged"}
          </span>
        </div>
```

- [ ] **Step 4: Update session-row grid to 7 columns**

In `frontend/src/App.css`, update `.session-row` grid-template-columns to include the new Status column:

```css
.session-row {
  background: #ffffff;
  padding: 16px 20px;
  display: grid;
  grid-template-columns: 100px 1fr 90px 100px 100px 100px 80px;
  align-items: center;
  gap: 16px;
}
```

- [ ] **Step 5: Verify frontend compiles and lint passes**

```bash
cd frontend && npm run build && npm run lint
```
Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/SessionTimeline.tsx frontend/src/App.css
git commit -m "feat: add Merged/Unmerged badge to session timeline"
```

---

## Chunk 3: Integration Verification

### Task 9: End-to-End Verification

- [ ] **Step 1: Build and start the full app**

```bash
cd /Users/unokohei/ghq/github.com/konu96/entire-dashboard && ./start.sh
```

- [ ] **Step 2: Manual verification checklist**

1. Open the dashboard in the browser
2. Register a repository (if none exists)
3. Click "Sync" — verify sync completes without errors in terminal
4. Verify the new "Merged to Main" KPI card appears with a percentage
5. Verify the session table has a "Status" column with "Merged"/"Unmerged" badges
6. Verify sessions on the `main` branch show "Merged"
7. Verify the existing KPI cards still display correctly

- [ ] **Step 3: Final commit if any fixes needed**

If any fixes were needed during verification, commit them.
