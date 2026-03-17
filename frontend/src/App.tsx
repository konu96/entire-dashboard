import { useEffect, useState, useCallback } from "react";
import { DailyDashboard } from "./components/DailyDashboard";
import { SessionTimeline } from "./components/SessionTimeline";
import {
  getDailyStats,
  getSessions,
  getRepos,
  addRepo as addRepoApi,
  deleteRepo as deleteRepoApi,
  syncData,
} from "./api/generated";
import type { DailyStat, Repository, Session } from "./api/generated";
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
  const [showRepoManage, setShowRepoManage] = useState(false);
  const [loadError, setLoadError] = useState<string>("");

  const loadRepos = useCallback(async () => {
    try {
      const res = await getRepos();
      if (res.status === 200) {
        setRepos(res.data);
      } else {
        setRepos([]);
        setLoadError(res.data.message);
      }
    } catch (e) {
      setRepos([]);
      setLoadError(e instanceof Error ? e.message : "リポジトリ取得に失敗しました");
    }
  }, []);

  const loadData = useCallback(async () => {
    try {
      setLoadError("");
      const repoFilter = selectedRepo || undefined;
      const [statsRes, sessRes] = await Promise.all([
        getDailyStats(repoFilter ? { repo: repoFilter } : undefined),
        getSessions(repoFilter ? { repo: repoFilter } : undefined),
      ]);
      setDailyStats(statsRes.status === 200 ? statsRes.data : []);
      setSessions(sessRes.status === 200 ? sessRes.data : []);
    } catch (e) {
      setDailyStats([]);
      setSessions([]);
      setLoadError(e instanceof Error ? e.message : "データ取得に失敗しました");
    }
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
      await syncData(repoFilter ? { repo: repoFilter } : undefined);
      setLastSynced(new Date().toLocaleString("ja-JP"));
      await loadData();
    } catch {
      setLastSynced("Sync failed");
    } finally {
      setSyncing(false);
    }
  };

  const handleAddRepo = async () => {
    if (addingRepo || !newRepoPath.trim()) return;
    setAddingRepo(true);
    try {
      const res = await addRepoApi({ path: newRepoPath.trim() });
      if (res.status !== 200) {
        throw new Error("data" in res ? (res.data as { message: string }).message : "Failed to add repo");
      }
      const repo = res.data;
      setNewRepoPath("");
      setShowRepoForm(false);
      await loadRepos();
      setSelectedRepo(repo.path);
      await syncData({ repo: repo.path });
      setLastSynced(new Date().toLocaleString("ja-JP"));
      const [statsRes, sessRes] = await Promise.all([
        getDailyStats({ repo: repo.path }),
        getSessions({ repo: repo.path }),
      ]);
      setDailyStats(statsRes.status === 200 ? statsRes.data : []);
      setSessions(sessRes.status === 200 ? sessRes.data : []);
    } catch (e) {
      alert(`Failed to add repo: ${e instanceof Error ? e.message : e}`);
    } finally {
      setAddingRepo(false);
    }
  };

  const handleDeleteRepo = async (id: number) => {
    if (!confirm("このリポジトリと関連するセッションデータを完全に削除しますか？")) {
      return;
    }
    try {
      const target = repos.find((r) => r.id === id);
      await deleteRepoApi(id);
      await loadRepos();
      if (target?.path === selectedRepo) {
        setSelectedRepo("");
      }
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
  const mergedAgentLines = sessions
    .filter((s) => s.merged_to_main)
    .reduce((sum, s) => sum + s.agent_lines, 0);
  const mergedPct =
    totalAgent > 0
      ? ((mergedAgentLines / totalAgent) * 100).toFixed(1)
      : "0.0";

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
          <button
            className="repo-add-button"
            onClick={() => setShowRepoManage(!showRepoManage)}
          >
            Manage
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

        {showRepoManage && (
          <div className="repo-manage">
            <div className="repo-manage-header">
              <span className="repo-manage-title">リポジトリ管理</span>
            </div>
            {repos.length === 0 && (
              <p className="repo-manage-empty">登録されたリポジトリはありません</p>
            )}
            {repos.map((r) => (
              <div key={r.id} className="repo-manage-row">
                <div className="repo-manage-info">
                  <span className="repo-tag-name">{r.name}</span>
                  <span className="repo-tag-path">{r.path}</span>
                </div>
                <button
                  className="repo-manage-delete"
                  onClick={() => handleDeleteRepo(r.id)}
                >
                  削除
                </button>
              </div>
            ))}
          </div>
        )}
      </div>

      {loadError && (
        <div style={{ color: "#d32f2f", background: "#fdecea", padding: "12px 16px", borderRadius: 8, marginBottom: 16, fontSize: 13 }}>
          {loadError}
        </div>
      )}

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
        <div className="kpi-card">
          <div className="kpi-label">Merged to Main</div>
          <div className="kpi-value">
            {mergedPct}
            <span className="kpi-unit">%</span>
          </div>
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
