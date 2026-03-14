import { useEffect, useState, useCallback } from "react";
import { DailyDashboard } from "./components/DailyDashboard";
import { SessionTimeline } from "./components/SessionTimeline";
import {
  fetchDailyStats,
  fetchSessions,
  fetchRepos,
  addRepo,
  deleteRepo,
  syncData,
} from "./api";
import type { DailyStat, Repository, Session } from "./types";
import "./App.css";

function App() {
  const [repos, setRepos] = useState<Repository[]>([]);
  const [selectedRepo, setSelectedRepo] = useState<string>("");
  const [dailyStats, setDailyStats] = useState<DailyStat[]>([]);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [syncing, setSyncing] = useState(false);
  const [lastSynced, setLastSynced] = useState<string>("");
  const [newRepoPath, setNewRepoPath] = useState("");
  const [addingRepo, setAddingRepo] = useState(false);
  const [showRepoForm, setShowRepoForm] = useState(false);

  const loadRepos = useCallback(async () => {
    const r = await fetchRepos();
    setRepos(r);
  }, []);

  const loadData = useCallback(async () => {
    const repoFilter = selectedRepo || undefined;
    const [stats, sess] = await Promise.all([
      fetchDailyStats(repoFilter),
      fetchSessions(repoFilter),
    ]);
    setDailyStats(stats);
    setSessions(sess);
  }, [selectedRepo]);

  useEffect(() => {
    loadRepos();
  }, [loadRepos]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const handleSync = async () => {
    setSyncing(true);
    try {
      const repoFilter = selectedRepo || undefined;
      await syncData(repoFilter);
      setLastSynced(new Date().toLocaleString("ja-JP"));
      await loadData();
    } catch {
      setLastSynced("Sync failed");
    } finally {
      setSyncing(false);
    }
  };

  const handleAddRepo = async () => {
    if (!newRepoPath.trim()) return;
    setAddingRepo(true);
    try {
      await addRepo(newRepoPath.trim());
      setNewRepoPath("");
      setShowRepoForm(false);
      await loadRepos();
    } catch (e) {
      alert(`Failed to add repo: ${e instanceof Error ? e.message : e}`);
    } finally {
      setAddingRepo(false);
    }
  };

  const handleDeleteRepo = async (id: number) => {
    try {
      await deleteRepo(id);
      await loadRepos();
      setSelectedRepo("");
    } catch (e) {
      alert(`Failed to delete repo: ${e instanceof Error ? e.message : e}`);
    }
  };

  // Compute KPIs
  const totalAgent = dailyStats.reduce((sum, d) => sum + d.agent_lines, 0);
  const totalHuman = dailyStats.reduce((sum, d) => sum + d.human_lines, 0);
  const totalAll = totalAgent + totalHuman;
  const overallPct =
    totalAll > 0 ? ((totalAgent / totalAll) * 100).toFixed(1) : "0.0";
  const totalSessions = sessions.length;

  return (
    <div className="dashboard">
      <header className="header">
        <h1 className="header-title">Entire Dashboard</h1>
        <div className="header-actions">
          {lastSynced && (
            <span className="sync-status">Last synced: {lastSynced}</span>
          )}
          <button
            onClick={handleSync}
            disabled={syncing}
            className="sync-button"
          >
            {syncing ? "Syncing..." : "Sync"}
          </button>
        </div>
      </header>

      {/* Repository Selector */}
      <div className="repo-bar">
        <div className="repo-selector">
          <select
            value={selectedRepo}
            onChange={(e) => setSelectedRepo(e.target.value)}
            className="repo-select"
          >
            <option value="">All Repositories</option>
            {repos.map((r) => (
              <option key={r.id} value={r.path}>
                {r.name} ({r.path})
              </option>
            ))}
          </select>

          <button
            className="repo-add-button"
            onClick={() => setShowRepoForm(!showRepoForm)}
          >
            + Add
          </button>
        </div>

        {showRepoForm && (
          <div className="repo-form">
            <input
              type="text"
              value={newRepoPath}
              onChange={(e) => setNewRepoPath(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleAddRepo()}
              placeholder="/path/to/git/repository"
              className="repo-input"
              disabled={addingRepo}
            />
            <button
              onClick={handleAddRepo}
              disabled={addingRepo || !newRepoPath.trim()}
              className="sync-button"
            >
              {addingRepo ? "Adding..." : "Add"}
            </button>
          </div>
        )}

        {repos.length > 0 && (
          <div className="repo-list">
            {repos.map((r) => (
              <div key={r.id} className="repo-tag">
                <span className="repo-tag-name">{r.name}</span>
                <span className="repo-tag-path">{r.path}</span>
                <button
                  className="repo-tag-delete"
                  onClick={() => handleDeleteRepo(r.id)}
                  title="Remove repository"
                >
                  &times;
                </button>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* KPI: Overview indicators */}
      <div className="kpi-row">
        <div className="kpi-card kpi-card--primary">
          <div className="kpi-label">AI Contribution Ratio</div>
          <div className="kpi-value">
            {overallPct}
            <span className="kpi-unit">%</span>
          </div>
        </div>
        <div className="kpi-card">
          <div className="kpi-label">AI Lines</div>
          <div className="kpi-value">{totalAgent.toLocaleString()}</div>
        </div>
        <div className="kpi-card">
          <div className="kpi-label">Human Lines</div>
          <div className="kpi-value">{totalHuman.toLocaleString()}</div>
        </div>
        <div className="kpi-card">
          <div className="kpi-label">Sessions</div>
          <div className="kpi-value">{totalSessions}</div>
        </div>
      </div>

      {/* Daily Chart: Trend over time */}
      <DailyDashboard data={dailyStats} />

      {/* Session Detail: Individual session breakdown */}
      <SessionTimeline sessions={sessions} />
    </div>
  );
}

export default App;
