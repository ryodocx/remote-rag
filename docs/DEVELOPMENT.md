# 開発者ガイド (Development Guide)

本ドキュメントでは、中継ブリッジ (`stdio-to-sse bridge`) や認証ヘルパー (`auth-helper`) の開発およびビルド方法について説明します。

## 1. ローカル開発環境の準備

開発には以下のツールが必要です。
- **Go**: `1.26.4` 以上 (ブリッジおよび認証ヘルパーの開発用)
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

# 全OS (macOS ARM, Linux, Windows) 向けのクロスコンパイル
make build-all
```

### Homebrewでの配布について
現在、GitHub Actions (`.github/workflows/release.yml`) を用いて、新しいバージョンタグがプッシュされると自動で各OS向けのバイナリがビルドされ、リポジトリ内の `Formula/rrag-bridge.rb` が更新されます。
ユーザーは `brew tap ryodocx/remote-rag` で簡単にインストールできるようになっています。

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

### 4.1 AIモデルの動作テスト

環境変数を用いてAIモデル（EmbeddingやReranker）を差し替えた際の動作確認は、専用のCLIスクリプトを使用すると便利です。

```bash
# 例: 日本語特化モデルのテスト
export EMBEDDING_MODEL="pkshatech/GLuCoSE-base-ja"
export EMBEDDING_PREFIX_QUERY=""
export EMBEDDING_PREFIX_PASSAGE=""

# ダミーデータの取り込みテスト
python scripts/ingest_cli.py dummy

# 検索（ハイブリッド検索 + リランク）のテスト
python scripts/search_cli.py "人工知能"
```
