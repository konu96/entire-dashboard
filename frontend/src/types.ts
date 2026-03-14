export interface Repository {
  id: number;
  path: string;
  name: string;
  created_at: string;
}

export interface DailyStat {
  date: string;
  agent_lines: number;
  human_lines: number;
  total_lines: number;
  agent_percentage: number;
  session_count: number;
}

export interface Session {
  id: number;
  repo_path: string;
  checkpoint_id: string;
  session_id: string;
  agent: string;
  branch: string;
  created_at: string;
  prompt: string;
  agent_lines: number;
  human_added: number;
  human_modified: number;
  human_removed: number;
  total_committed: number;
  agent_percentage: number;
  input_tokens: number;
  output_tokens: number;
  api_call_count: number;
}
