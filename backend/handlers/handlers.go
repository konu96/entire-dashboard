package handlers

import (
	"context"
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
			ID:        r.ID,
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
		ID:        repo.ID,
		Path:      repo.Path,
		Name:      repo.Name,
		CreatedAt: repo.CreatedAt,
	}, nil
}

func (h *Handler) DeleteRepo(ctx context.Context, params generated.DeleteRepoParams) (generated.DeleteRepoRes, error) {
	if err := h.store.DeleteRepo(params.ID); err != nil {
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
			AgentLines:      s.AgentLines,
			HumanLines:      s.HumanLines,
			TotalLines:      s.TotalLines,
			AgentPercentage: s.AgentPercentage,
			SessionCount:    s.SessionCount,
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
			ID:              s.ID,
			RepoPath:        s.RepoPath,
			CheckpointID:    s.CheckpointID,
			SessionID:       s.SessionID,
			Agent:           s.Agent,
			Branch:          s.Branch,
			CreatedAt:       s.CreatedAt,
			Prompt:          s.Prompt,
			AgentLines:      s.AgentLines,
			HumanAdded:      s.HumanAdded,
			HumanModified:   s.HumanModified,
			HumanRemoved:    s.HumanRemoved,
			TotalCommitted:  s.TotalCommitted,
			AgentPercentage: s.AgentPercentage,
			InputTokens:     s.InputTokens,
			OutputTokens:    s.OutputTokens,
			APICallCount:    s.APICallCount,
			MergedToMain:    s.MergedToMain,
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

	return &generated.SyncResponse{
		TotalFound: totalFound,
		Inserted:   totalInserted,
		Message:    "sync complete",
	}, nil
}
