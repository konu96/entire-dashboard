package handlers

import (
	"encoding/json"
	"entire-dashboard/db"
	gitreader "entire-dashboard/git"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
)

type Handler struct {
	store *db.Store
}

func New(store *db.Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) GetRepos(w http.ResponseWriter, r *http.Request) {
	repos, err := h.store.GetRepos()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, repos)
}

func (h *Handler) AddRepo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Path == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}

	absPath, err := filepath.Abs(req.Path)
	if err != nil {
		http.Error(w, "invalid path: "+err.Error(), http.StatusBadRequest)
		return
	}

	name := filepath.Base(absPath)
	repo, err := h.store.AddRepo(absPath, name)
	if err != nil {
		http.Error(w, "failed to add repo: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, repo)
}

func (h *Handler) DeleteRepo(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.store.DeleteRepo(id); err != nil {
		http.Error(w, "failed to delete repo: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func (h *Handler) GetDailyStats(w http.ResponseWriter, r *http.Request) {
	repoPath := r.URL.Query().Get("repo")
	stats, err := h.store.GetDailyStats(repoPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, stats)
}

func (h *Handler) GetSessions(w http.ResponseWriter, r *http.Request) {
	repoPath := r.URL.Query().Get("repo")
	sessions, err := h.store.GetSessions(repoPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, sessions)
}

// Sync reads from the git shadow branch and upserts into SQLite.
// If ?repo= is specified, syncs that repo only. Otherwise syncs all registered repos.
func (h *Handler) Sync(w http.ResponseWriter, r *http.Request) {
	repoPath := r.URL.Query().Get("repo")

	var repos []string
	if repoPath != "" {
		repos = append(repos, repoPath)
	} else {
		repoList, err := h.store.GetRepos()
		if err != nil {
			http.Error(w, "failed to list repos: "+err.Error(), http.StatusInternalServerError)
			return
		}
		for _, r := range repoList {
			repos = append(repos, r.Path)
		}
	}

	totalFound := 0
	totalInserted := 0

	for _, rp := range repos {
		reader := gitreader.NewReader(rp)
		sessions, err := reader.ReadAllSessions()
		if err != nil {
			log.Printf("sync error for %s: %v", rp, err)
			continue
		}

		inserted := 0
		for _, sess := range sessions {
			sess.RepoPath = rp
			if sess.SessionID == "" {
				log.Printf("skip session with empty session_id in %s", rp)
				continue
			}
			exists, err := h.store.SessionExists(sess.SessionID)
			if err != nil {
				log.Printf("check exists error: %v", err)
				continue
			}
			if exists {
				continue
			}
			if err := h.store.UpsertSession(sess); err != nil {
				log.Printf("upsert error: %v", err)
				continue
			}
			inserted++
		}
		totalFound += len(sessions)
		totalInserted += inserted
	}

	writeJSON(w, map[string]any{
		"total_found": totalFound,
		"inserted":    totalInserted,
		"message":     "sync complete",
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
