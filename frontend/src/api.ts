import type { DailyStat, Repository, Session } from "./types";

const BASE = "http://localhost:8080";

export async function fetchRepos(): Promise<Repository[]> {
  const res = await fetch(`${BASE}/api/repos`);
  if (!res.ok) {
    throw new Error(`Failed to fetch repos: ${res.status}`);
  }
  return (await res.json()) ?? [];
}

export async function addRepo(path: string): Promise<Repository> {
  const res = await fetch(`${BASE}/api/repos`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ path }),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text);
  }
  return res.json();
}

export async function deleteRepo(id: number): Promise<void> {
  const res = await fetch(`${BASE}/api/repos/${id}`, { method: "DELETE" });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text);
  }
}

export async function fetchDailyStats(repoPath?: string): Promise<DailyStat[]> {
  const params = repoPath ? `?repo=${encodeURIComponent(repoPath)}` : "";
  const res = await fetch(`${BASE}/api/daily-stats${params}`);
  if (!res.ok) {
    throw new Error(`Failed to fetch daily stats: ${res.status}`);
  }
  return (await res.json()) ?? [];
}

export async function fetchSessions(repoPath?: string): Promise<Session[]> {
  const params = repoPath ? `?repo=${encodeURIComponent(repoPath)}` : "";
  const res = await fetch(`${BASE}/api/sessions${params}`);
  if (!res.ok) {
    throw new Error(`Failed to fetch sessions: ${res.status}`);
  }
  return (await res.json()) ?? [];
}

export async function syncData(repoPath?: string): Promise<{ total_found: number; inserted: number }> {
  const params = repoPath ? `?repo=${encodeURIComponent(repoPath)}` : "";
  const res = await fetch(`${BASE}/api/sync${params}`, { method: "POST" });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text);
  }
  return res.json();
}
