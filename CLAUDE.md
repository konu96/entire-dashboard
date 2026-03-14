# CLAUDE.md

このファイルは、Claude Code (claude.ai/code) がこのリポジトリで作業する際のガイダンスを提供します。

## ビルド・起動

```bash
# 本番: 全体をビルドして起動（デフォルトポート 8080）
./start.sh [REPO_PATH] [PORT]

# バックエンドのみ
cd backend && go run main.go --port 8080

# フロントエンド開発サーバー（Vite、ポート 5173）
cd frontend && npm install && npm run dev

# フロントエンド lint
cd frontend && npm run lint
```

バックエンドの CLI フラグ: `--repo <path>`（起動時にリポジトリを自動登録）、`--port <port>`。

自動テストスイートはまだ存在しません。

## アーキテクチャ

Entire CLI の AI エージェントセッションメトリクスを可視化するフルスタックアプリ。Go バックエンドが React SPA と SQLite ベースの REST API を提供する。

**データフロー:** ユーザーが Git リポジトリを登録 → 「Sync」で `entire/checkpoints/v1` shadow branch を `git ls-tree`/`git show` で読み取り → チェックポイント JSON をパース → SQLite にセッションを upsert → フロントエンドが REST API で統計を取得 → チャート（Recharts）とテーブルを描画。

**バックエンド (Go 1.23, `backend/`):**
- `main.go` — サーバー設定、ルート登録、`frontend/dist/` からの静的ファイル配信
- `handlers/handlers.go` — `*db.Store` をラップした `Handler` 構造体上の全 HTTP ハンドラ
- `db/db.go` — WAL モードの SQLite ストア、自動マイグレーション、CRUD + 集計クエリ
- `models/models.go` — データ構造体（Repository, Session, DailyStat, CheckpointMeta）
- `git/reader.go` — Git shadow branch から Entire チェックポイントメタデータを読み取り

**フロントエンド (React 19 + TypeScript + Vite, `frontend/`):**
- `src/App.tsx` — メインコンポーネント、useState/useEffect による状態管理
- `src/api.ts` — `BASE = "http://localhost:8080"` がハードコードされた API クライアント
- `src/components/DailyDashboard.tsx` — 積み上げ棒グラフ（AI vs ヒューマンの行数）
- `src/components/SessionTimeline.tsx` — セッション詳細テーブル

## API ルート

```
GET    /api/repos          — リポジトリ一覧
POST   /api/repos          — リポジトリ追加（body: {path}）
DELETE /api/repos/{id}     — リポジトリ削除（関連セッションも削除）
GET    /api/daily-stats    — 日次集計統計（?repo=path でフィルタ可）
GET    /api/sessions       — セッション一覧（?repo=path でフィルタ可）
POST   /api/sync           — Git からデータ同期（?repo=path でフィルタ可）
```

## 主要パターン

- **ハンドラメソッド** は `(w http.ResponseWriter, r *http.Request)` を受け取り、パスパラメータに `r.PathValue()`、クエリパラメータに `r.URL.Query().Get()` を使用
- **データベース** は `INSERT ... ON CONFLICT DO UPDATE` でセッションの冪等な upsert を実現
- **CORS ミドルウェア** は全オリジンを許可（開発向け設定）
- **フロントエンドの API 呼び出し** は `(await res.json()) ?? []` で null フォールバック
- **TypeScript** は strict モード有効、`noUnusedLocals` と `noUnusedParameters` を設定
- **SQLite** は `~/.entire-dashboard/dashboard.db` に保存、初回起動時に自動作成

## スタイリング

素の CSS（フレームワークなし）。日本語フォールバック付きシステムフォント（Hiragino Kaku Gothic ProN）。コード表示用に等幅フォント（SF Mono, Fira Code）。カラーパレット: 青系 (#0031D8)、緑系 (#22A06B)、グレー系。
