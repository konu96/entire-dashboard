package git

import (
	"encoding/json"
	"entire-dashboard/models"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

const shadowBranch = "entire/checkpoints/v1"

// Reader reads Entire checkpoint data from a git repository's shadow branch.
type Reader struct {
	repoPath string
}

func NewReader(repoPath string) *Reader {
	return &Reader{repoPath: repoPath}
}

// ReadAllSessions extracts all session metadata from the shadow branch.
func (r *Reader) ReadAllSessions() ([]models.Session, error) {
	// List top-level directories (2-char prefixes)
	prefixes, err := r.lsTree(shadowBranch)
	if err != nil {
		return nil, fmt.Errorf("list prefixes: %w", err)
	}

	var sessions []models.Session
	for _, prefix := range prefixes {
		// List checkpoint IDs under each prefix
		checkpointIDs, err := r.lsTree(shadowBranch + ":" + prefix + "/")
		if err != nil {
			continue
		}
		for _, cpID := range checkpointIDs {
			path := prefix + "/" + cpID
			ss, err := r.readCheckpointSessions(path, prefix+cpID)
			if err != nil {
				continue
			}
			sessions = append(sessions, ss...)
		}
	}
	return sessions, nil
}

func (r *Reader) readCheckpointSessions(treePath, checkpointID string) ([]models.Session, error) {
	// List entries in this checkpoint (numbered session dirs + metadata.json)
	entries, err := r.lsTree(shadowBranch + ":" + treePath + "/")
	if err != nil {
		return nil, err
	}

	var sessions []models.Session
	for _, entry := range entries {
		// Only process numbered directories (session indices)
		if _, err := strconv.Atoi(entry); err != nil {
			continue
		}

		metaPath := treePath + "/" + entry + "/metadata.json"
		meta, err := r.readSessionMeta(metaPath)
		if err != nil {
			continue
		}

		prompt := r.readFileContent(treePath + "/" + entry + "/prompt.txt")

		sessions = append(sessions, models.Session{
			CheckpointID:    meta.CheckpointID,
			SessionID:       meta.SessionID,
			Agent:           meta.Agent,
			Branch:          meta.Branch,
			CreatedAt:       meta.CreatedAt,
			Prompt:          truncate(prompt, 500),
			AgentLines:      meta.Attribution.AgentLines,
			HumanAdded:      meta.Attribution.HumanAdded,
			HumanModified:   meta.Attribution.HumanModified,
			HumanRemoved:    meta.Attribution.HumanRemoved,
			TotalCommitted:  meta.Attribution.TotalCommitted,
			AgentPercentage: meta.Attribution.AgentPercentage,
			InputTokens:     meta.TokenUsage.InputTokens,
			OutputTokens:    meta.TokenUsage.OutputTokens,
			APICallCount:    meta.TokenUsage.APICallCount,
		})
	}
	return sessions, nil
}

func (r *Reader) readSessionMeta(path string) (models.SessionMeta, error) {
	var meta models.SessionMeta
	content := r.readFileContent(path)
	if content == "" {
		return meta, fmt.Errorf("empty content at %s", path)
	}
	err := json.Unmarshal([]byte(content), &meta)
	return meta, err
}

func (r *Reader) readFileContent(path string) string {
	out, err := r.gitShow(shadowBranch + ":" + path)
	if err != nil {
		return ""
	}
	return out
}

func (r *Reader) lsTree(ref string) ([]string, error) {
	out, err := exec.Command("git", "-C", r.repoPath, "ls-tree", "--name-only", ref).Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var result []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			result = append(result, l)
		}
	}
	return result, nil
}

func (r *Reader) gitShow(ref string) (string, error) {
	out, err := exec.Command("git", "-C", r.repoPath, "show", ref).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
