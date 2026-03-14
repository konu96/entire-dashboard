# Session Context

## User Prompts

### Prompt 1

Base directory for this skill: /Users/unokohei/.claude/skills/pr-review-responder

# PR Review Responder

PRのレビューコメントを確認し、対応要否を分類した上で指摘に対応してコードを修正する。

## ワークフロー

### 1. PR情報を取得

```bash
# ブランチ名
git branch --show-current

# リポジトリ情報（owner/repo を抽出）
git remote get-url origin
```

`git@github.com:owner/repo.git` または `https://github.com/owner/repo.git` から...

### Prompt 2

修正をして

### Prompt 3

対応して

### Prompt 4

[Request interrupted by user]

### Prompt 5

コミットとプッシュして

