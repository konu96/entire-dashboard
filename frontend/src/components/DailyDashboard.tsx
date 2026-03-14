import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from "recharts";
import type { DailyStat } from "../types";

interface Props {
  data: DailyStat[];
}

export function DailyDashboard({ data }: Props) {
  if (data.length === 0) {
    return <div className="empty-state">No data yet. Click "Sync" to load.</div>;
  }

  // Format date for display (MM/DD)
  const chartData = data.map((d) => ({
    ...d,
    dateLabel: d.date.slice(5), // "03-12" format
  }));

  return (
    <div className="section">
      <div className="section-title">AI / Human Lines by Day (Stacked Bar)</div>
      <div className="section-subtitle">
        Each bar shows total lines committed per day, split by AI-generated and human-written lines
      </div>
      <div className="chart-container">
        <ResponsiveContainer width="100%" height={340}>
          <BarChart data={chartData} margin={{ top: 8, right: 8, left: 0, bottom: 4 }}>
            <CartesianGrid vertical={false} stroke="#e8e8eb" />
            <XAxis
              dataKey="dateLabel"
              tick={{ fill: "#626264", fontSize: 12 }}
              axisLine={{ stroke: "#e8e8eb" }}
              tickLine={false}
            />
            <YAxis
              tick={{ fill: "#626264", fontSize: 12 }}
              axisLine={false}
              tickLine={false}
              width={60}
              tickFormatter={(v: number) => v.toLocaleString()}
            />
            <Tooltip content={<CustomTooltip />} />
            <Bar dataKey="agent_lines" name="AI Lines" stackId="a" fill="#0031D8" />
            <Bar dataKey="human_lines" name="Human Lines" stackId="a" fill="#22A06B" radius={[3, 3, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
        {/* Inline legend next to chart */}
        <div style={{ display: "flex", gap: 24, paddingLeft: 60, marginTop: 4 }}>
          <LegendItem color="#0031D8" label="AI Lines" />
          <LegendItem color="#22A06B" label="Human Lines" />
        </div>
      </div>
      <div className="meta-info">
        <span>Data source: Entire Checkpoints (Shadow Branch)</span>
        <span>Period: {data[0]?.date} - {data[data.length - 1]?.date}</span>
      </div>
    </div>
  );
}

function LegendItem({ color, label }: { color: string; label: string }) {
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
      <div style={{ width: 12, height: 12, borderRadius: 2, background: color }} />
      <span style={{ fontSize: 12, color: "#626264" }}>{label}</span>
    </div>
  );
}

function CustomTooltip({ active, payload, label }: { active?: boolean; payload?: Array<{ value: number; name: string; color: string }>; label?: string }) {
  if (!active || !payload?.length) return null;

  const ai = payload.find((p) => p.name === "AI Lines")?.value ?? 0;
  const human = payload.find((p) => p.name === "Human Lines")?.value ?? 0;
  const total = ai + human;
  const aiPct = total > 0 ? ((ai / total) * 100).toFixed(1) : "0.0";

  return (
    <div style={{
      background: "#ffffff",
      border: "1px solid #e8e8eb",
      borderRadius: 6,
      padding: "12px 16px",
      boxShadow: "0 2px 8px rgba(0,0,0,0.08)",
      fontSize: 13,
    }}>
      <div style={{ fontWeight: 600, marginBottom: 8, color: "#1a1a1a" }}>{label}</div>
      <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
        <div style={{ display: "flex", justifyContent: "space-between", gap: 24 }}>
          <span style={{ color: "#0031D8" }}>AI Lines</span>
          <span style={{ fontWeight: 600 }}>{ai.toLocaleString()}</span>
        </div>
        <div style={{ display: "flex", justifyContent: "space-between", gap: 24 }}>
          <span style={{ color: "#22A06B" }}>Human Lines</span>
          <span style={{ fontWeight: 600 }}>{human.toLocaleString()}</span>
        </div>
        <div style={{ borderTop: "1px solid #e8e8eb", paddingTop: 4, marginTop: 4, display: "flex", justifyContent: "space-between" }}>
          <span style={{ color: "#626264" }}>AI Ratio</span>
          <span style={{ fontWeight: 700, color: "#0031D8" }}>{aiPct}%</span>
        </div>
      </div>
    </div>
  );
}
