# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
# Production: build and start everything (default port 8080)
./start.sh [REPO_PATH] [PORT]

# Backend only
cd backend && go run main.go --port 8080

# Frontend dev server (Vite, port 5173)
cd frontend && npm install && npm run dev

# Frontend lint
cd frontend && npm run lint
```

CLI flags for backend: `--repo <path>` (auto-register repo on startup), `--port <port>`.

No automated test suite exists yet.

## Architecture

Full-stack app that visualizes AI agent session metrics from Entire CLI. Go backend serves a React SPA and a REST API backed by SQLite.

**Data flow:** User registers Git repos → "Sync" reads the `entire/checkpoints/v1` shadow branch via `git ls-tree`/`git show` → parses checkpoint JSON → upserts sessions into SQLite → frontend fetches stats via REST API → renders charts (Recharts) and tables.

**Backend (Go 1.23, `backend/`):**
- `main.go` — server setup, route registration, static file serving from `frontend/dist/`
- `handlers/handlers.go` — all HTTP handlers on a `Handler` struct wrapping `*db.Store`
- `db/db.go` — SQLite store with WAL mode, auto-migration, CRUD + aggregation queries
- `models/models.go` — data structs (Repository, Session, DailyStat, CheckpointMeta)
- `git/reader.go` — reads Entire checkpoint metadata from Git shadow branch

**Frontend (React 19 + TypeScript + Vite, `frontend/`):**
- `src/App.tsx` — main component, all state management via useState/useEffect
- `src/api.ts` — API client with hardcoded `BASE = "http://localhost:8080"`
- `src/components/DailyDashboard.tsx` — stacked bar chart (AI vs Human lines)
- `src/components/SessionTimeline.tsx` — session detail table

## API Routes

```
GET    /api/repos          — list repositories
POST   /api/repos          — add repository (body: {path})
DELETE /api/repos/{id}     — delete repository + associated sessions
GET    /api/daily-stats    — daily aggregated stats (?repo=path filter)
GET    /api/sessions       — session list (?repo=path filter)
POST   /api/sync           — sync from Git (?repo=path filter)
```

## Key Patterns

- **Handler methods** receive `(w http.ResponseWriter, r *http.Request)`, use `r.PathValue()` for path params and `r.URL.Query().Get()` for query params
- **Database** uses `INSERT ... ON CONFLICT DO UPDATE` for idempotent session upserts
- **CORS middleware** allows all origins (development-oriented)
- **Frontend API calls** return `(await res.json()) ?? []` with null fallback
- **TypeScript** strict mode enabled with `noUnusedLocals` and `noUnusedParameters`
- **SQLite** stored at `~/.entire-dashboard/dashboard.db`, auto-created on first run

## Styling

Pure CSS (no framework). System fonts with Japanese fallback (Hiragino Kaku Gothic ProN). Monospace fonts (SF Mono, Fira Code) for code-like content. Color palette: blues (#0031D8), greens (#22A06B), grays.
