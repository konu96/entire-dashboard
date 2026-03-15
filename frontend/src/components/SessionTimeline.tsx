import { useState } from "react";
import type { Session } from "../api/generated";

interface Props {
  sessions: Session[];
}

export function SessionTimeline({ sessions }: Props) {
  if (sessions.length === 0) {
    return <div className="empty-state">No sessions yet. Click "Sync" to load.</div>;
  }

  return (
    <div className="section">
      <div className="section-title">Session Details</div>
      <div className="section-subtitle">
        Individual AI agent sessions, sorted by most recent. Click a row to expand the full prompt.
      </div>
      <div className="session-list">
        <div className="session-row session-row--header">
          <div>Date</div>
          <div>Branch / Prompt</div>
          <div style={{ textAlign: "right" }}>AI Lines</div>
          <div style={{ textAlign: "right" }}>Human Lines</div>
          <div style={{ textAlign: "right" }}>Total</div>
          <div style={{ textAlign: "right" }}>AI Ratio</div>
        </div>
        {sessions.map((s) => (
          <SessionRow key={s.session_id} session={s} />
        ))}
      </div>
      <div className="meta-info">
        <span>{sessions.length} sessions total</span>
        <span>Agent lines = lines attributed to AI agent during checkpoint</span>
      </div>
    </div>
  );
}

function SessionRow({ session: s }: { session: Session }) {
  const [expanded, setExpanded] = useState(false);
  const date = s.created_at.slice(5, 10);
  const time = s.created_at.slice(11, 16);
  const pct = s.agent_percentage;

  return (
    <div>
      <div className="session-row session-row--clickable" onClick={() => setExpanded(!expanded)}>
        <div className="session-date">
          <div className="session-date-main">{date}</div>
          <div style={{ fontSize: 11, color: "#626264" }}>{time}</div>
        </div>
        <div style={{ minWidth: 0 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <span className="agent-badge">{s.agent}</span>
            <span className="session-branch" title={s.branch}>{s.branch}</span>
            {s.prompt && (
              <span style={{ fontSize: 11, color: "#626264", flexShrink: 0 }}>
                {expanded ? "▲" : "▼"}
              </span>
            )}
          </div>
          {!expanded && s.prompt && (
            <div className="session-prompt">{s.prompt}</div>
          )}
        </div>
        <div className="session-lines session-lines--ai">{s.agent_lines.toLocaleString()}</div>
        <div className="session-lines session-lines--human">{s.human_added.toLocaleString()}</div>
        <div className="session-lines session-lines--total">{s.total_committed.toLocaleString()}</div>
        <div className="session-ratio">
          <div className="session-ratio-bar">
            <div className="session-ratio-fill" style={{ width: `${pct}%` }} />
          </div>
          <div className="session-ratio-text">{pct.toFixed(1)}%</div>
        </div>
      </div>
      {expanded && s.prompt && (
        <div
          style={{
            background: "#f8f8fb",
            padding: "12px 20px 12px 136px",
            fontSize: 13,
            lineHeight: 1.6,
            color: "#1a1a1a",
            whiteSpace: "pre-wrap",
            overflowWrap: "break-word",
            borderTop: "1px solid #e8e8eb",
            maxHeight: 300,
            overflowY: "auto",
          }}
        >
          {s.prompt}
        </div>
      )}
    </div>
  );
}
