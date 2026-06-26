# 開発者ガイド (Development Guide)

本ドキュメントでは、中継ブリッジ (`stdio-to-sse bridge`) や認証ヘルパー (`auth-helper`) の開発およびビルド方法について説明します。

## 1. ローカル開発環境の準備

開発には以下のツールが必要です。
- **Go**: `1.21` 以上 (ブリッジおよび認証ヘルパーの開発用)
- **Docker & Docker Compose**: (ローカルインフラの起動用)

## 2. ブリッジ (Bridge) のビルドと配布

クライアント用中継ブリッジは `client/bridge/` ディレクトリにソースコードがあります。
このツールはCursorやClaude Desktop等のローカルから実行されるため、各OS向けにビルドする必要があります。

```bash
cd client/bridge

# 依存関係のダウンロード
go mod tidy

# ローカル環境（自分のPC）向けのビルド
make build

# 全OS (macOS Intel/ARM, Linux, Windows) 向けのクロスコンパイル
make build-all
```

### Homebrew (Custom Tap) での配布について
組織内で配布する場合、`make build-all` で生成したバイナリ（特に macOS 向けの `remote-rag-bridge-darwin-arm64` 等）をGitHub Releaseにアップロードし、社内用の Homebrew Tap リポジトリを作成してFormulaを定義することで、ユーザーは `brew install your-org/tap/remote-rag-bridge` で簡単に導入できます。

## 3. 認証ヘルパー (Auth Helper) の開発

認証ヘルパーは `server/auth-helper/` ディレクトリにあります。
トークンのハッシュ化やRedis通信のロジックを改修した場合は、Dockerコンテナを再ビルドして検証します。

```bash
cd deploy
# 認証ヘルパーの変更を反映してコンテナを再起動
docker compose up -d --build auth-helper
```

## 4. MCP Server の開発

MCPの検索ロジックやツール定義は `server/core/src/mcp_server/server.py` 等のPythonコードに記述されています。
開発時はDockerコンテナ内で起動するか、ローカルのPython仮想環境（`.venv`）を利用して検証します。

```bash
cd server/core
# ローカル仮想環境のセットアップ
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

# 単体でのテスト起動 (stdioモード)
python -m src.mcp_server.server --transport stdio
```
