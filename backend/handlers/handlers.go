package handlers

import (
	"context"
	"database/sql"
	"entire-dashboard/db"
	"entire-dashboard/generated"
	gitreader "entire-dashboard/git"
	"log"
	"path/filepath"
)

type Handler struct {
	store *db.Store
}

var _ generated.Handler = (*Handler)(nil)

func New(store *db.Store) *Handler {
	return &Handler{store: store}
}

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

func (h *Handler) DeleteRepo(ctx context.Context, params generated.DeleteRepoParams) (generated.DeleteRepoRes, error) {
	if err := h.store.DeleteRepo(int64(params.ID)); err != nil {
		return &generated.DeleteRepoInternalServerError{Message: "failed to delete repo: " + err.Error()}, nil
	}
	return &generated.DeleteRepoResponse{Status: "ok"}, nil
}

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
			AgentLines:      int(nullFloat64ToInt64(s.AgentLines)),
			HumanLines:      int(nullFloat64ToInt64(s.HumanLines)),
			TotalLines:      int(s.TotalLines),
			AgentPercentage: float64(s.AgentPercentage),
			SessionCount:    int(s.SessionCount),
		})
	}
	return &result, nil
}

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

func (h *Handler) SyncData(ctx context.Context, params generated.SyncDataParams) (generated.SyncDataRes, error) {
	repoPath := params.Repo.Or("")

	var repos []string
	if repoPath != "" {
		repos = append(repos, repoPath)
	} else {
		repoList, err := h.store.GetRepos()
		if err != nil {
			return &generated.ErrorResponse{Message: "failed to list repos: " + err.Error()}, nil
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

	return &generated.SyncResponse{
		TotalFound: totalFound,
		Inserted:   totalInserted,
		Message:    "sync complete",
	}, nil
}

// nullFloat64ToInt64 converts sql.NullFloat64 to int64, returning 0 if not valid.
func nullFloat64ToInt64(n sql.NullFloat64) int64 {
	if !n.Valid {
		return 0
	}
	return int64(n.Float64)
}
