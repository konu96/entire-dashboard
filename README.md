# entire-dashboard

Entire CLI の AI エージェントセッションメトリクスを可視化するダッシュボードアプリケーション。Git リポジトリに記録された Entire チェックポイントデータを読み取り、AI とヒューマンのコード貢献を分析・表示します。

## アーキテクチャ

- **Backend:** Go (net/http + SQLite)
- **Frontend:** React + TypeScript + Vite + Recharts

```
entire-dashboard/
├── backend/
│   ├── main.go              # エントリポイント、サーバー起動
│   ├── handlers/handlers.go # HTTP ルートハンドラ
│   ├── db/db.go             # SQLite ストア (CRUD、マイグレーション)
│   ├── models/models.go     # データモデル定義
│   └── git/reader.go        # Git shadow branch からのデータ読み取り
├── frontend/
│   ├── src/
│   │   ├── App.tsx          # メインコンポーネント
│   │   ├── api.ts           # API クライアント
│   │   ├── types.ts         # TypeScript 型定義
│   │   └── components/
│   │       ├── DailyDashboard.tsx   # 日次チャート
│   │       └── SessionTimeline.tsx  # セッション一覧テーブル
│   └── package.json
└── start.sh                 # ビルド＆起動スクリプト
```

## 機能

- **リポジトリ管理** - Git リポジトリの登録・削除・一覧表示
- **データ同期** - Git の `entire/checkpoints/v1` shadow branch からセッションメタデータを取得
- **KPI カード** - AI 貢献率、AI 行数、ヒューマン行数、セッション数
- **日次チャート** - AI vs ヒューマンのコード貢献トレンド（積み上げ棒グラフ）
- **セッション詳細** - 個別セッションのエージェント名、ブランチ、プロンプト、トークン使用量
- **リポジトリ別フィルタリング** - 全体または特定リポジトリの統計表示

## セットアップ

### 必要なもの

- Go 1.23+
- Node.js (npm)
- Git

### 起動

```bash
./start.sh [REPO_PATH] [PORT]
```

このスクリプトは以下を実行します:

1. Go バックエンドをビルド
2. フロントエンドの依存関係をインストール＆ビルド
3. `http://localhost:PORT` でサーバーを起動（デフォルト: 8080）

```bash
# デフォルト (ポート 8080)
./start.sh

# リポジトリを指定して起動
./start.sh /path/to/my/repo

# リポジトリとポートを指定
./start.sh /path/to/my/repo 3000
```

### 開発

バックエンドとフロントエンドを個別に起動できます。

```bash
# バックエンド
cd backend && go run main.go --port 8080

# フロントエンド (Vite dev server)
cd frontend && npm install && npm run dev
```

## API

| メソッド | エンドポイント | 説明 |
|---------|--------------|------|
| GET | `/api/repos` | 登録済みリポジトリ一覧 |
| POST | `/api/repos` | リポジトリ登録 |
| DELETE | `/api/repos/{id}` | リポジトリ削除 |
| GET | `/api/daily-stats` | 日次統計（`?repo=path` でフィルタ可） |
| GET | `/api/sessions` | セッション一覧（`?repo=path` でフィルタ可） |
| POST | `/api/sync` | Git からデータ同期（`?repo=path` で対象指定可） |

## データ保存先

- **データベース:** `~/.entire-dashboard/dashboard.db`（SQLite、WAL モード）
- 初回起動時に自動作成されます
