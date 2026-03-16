package models

// Repository represents a registered git repository.
type Repository struct {
	ID        int    `json:"id"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// Session represents a single AI agent session extracted from Entire's shadow branch.
type Session struct {
	ID              int     `json:"id"`
	RepoPath        string  `json:"repo_path"`
	CheckpointID    string  `json:"checkpoint_id"`
	SessionID       string  `json:"session_id"`
	Agent           string  `json:"agent"`
	Branch          string  `json:"branch"`
	CreatedAt       string  `json:"created_at"`
	Prompt          string  `json:"prompt"`
	AgentLines      int     `json:"agent_lines"`
	HumanAdded      int     `json:"human_added"`
	HumanModified   int     `json:"human_modified"`
	HumanRemoved    int     `json:"human_removed"`
	TotalCommitted  int     `json:"total_committed"`
	AgentPercentage float64 `json:"agent_percentage"`
	InputTokens     int     `json:"input_tokens"`
	OutputTokens    int     `json:"output_tokens"`
	APICallCount    int     `json:"api_call_count"`
	MergedToMain    bool    `json:"merged_to_main"`
}

// DailyStat represents aggregated daily statistics.
type DailyStat struct {
	Date            string  `json:"date"`
	AgentLines      int     `json:"agent_lines"`
	HumanLines      int     `json:"human_lines"`
	TotalLines      int     `json:"total_lines"`
	AgentPercentage float64 `json:"agent_percentage"`
	SessionCount    int     `json:"session_count"`
}

// CheckpointMeta is the top-level metadata.json for a checkpoint.
type CheckpointMeta struct {
	CLIVersion       string            `json:"cli_version"`
	CheckpointID     string            `json:"checkpoint_id"`
	Strategy         string            `json:"strategy"`
	Branch           string            `json:"branch"`
	CheckpointsCount int               `json:"checkpoints_count"`
	FilesTouched     []string          `json:"files_touched"`
	Sessions         []SessionRef      `json:"sessions"`
	TokenUsage       TokenUsage        `json:"token_usage"`
}

// SessionRef is a reference to session files within a checkpoint.
type SessionRef struct {
	Metadata   string `json:"metadata"`
	Transcript string `json:"transcript"`
	Context    string `json:"context"`
	Prompt     string `json:"prompt"`
}

// SessionMeta is the session-level metadata.json.
type SessionMeta struct {
	CLIVersion       string       `json:"cli_version"`
	CheckpointID     string       `json:"checkpoint_id"`
	SessionID        string       `json:"session_id"`
	Strategy         string       `json:"strategy"`
	CreatedAt        string       `json:"created_at"`
	Branch           string       `json:"branch"`
	Agent            string       `json:"agent"`
	FilesTouched     []string     `json:"files_touched"`
	TokenUsage       TokenUsage   `json:"token_usage"`
	Attribution      Attribution  `json:"initial_attribution"`
}

type TokenUsage struct {
	InputTokens          int `json:"input_tokens"`
	CacheCreationTokens  int `json:"cache_creation_tokens"`
	CacheReadTokens      int `json:"cache_read_tokens"`
	OutputTokens         int `json:"output_tokens"`
	APICallCount         int `json:"api_call_count"`
}

type Attribution struct {
	CalculatedAt    string  `json:"calculated_at"`
	AgentLines      int     `json:"agent_lines"`
	HumanAdded      int     `json:"human_added"`
	HumanModified   int     `json:"human_modified"`
	HumanRemoved    int     `json:"human_removed"`
	TotalCommitted  int     `json:"total_committed"`
	AgentPercentage float64 `json:"agent_percentage"`
}
