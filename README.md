# Remote RAG MCP Server & Auth Proxy

このリポジトリは、社内ナレッジベース（Wikipedia等）を検索するためのRAGエンジンを、Model Context Protocol (MCP) を通じてAIエージェント（Cursor, Claude Desktop等）に提供するための基盤です。汎用的な OAuth 2.0 / OIDC 認証とプロキシ機構を統合し、安全なアクセスを実現しています。

## 📚 ドキュメント (Documentation)

アーキテクチャや運用手順に関する詳細な資料は `docs/` ディレクトリを参照してください。

- **[アーキテクチャ設計書 (ARCHITECTURE.md)](docs/ARCHITECTURE.md)**: システム全体の構成図、Caddy・Auth Helper・Bridgeの役割と連携フロー。
- **[運用手順書 (OPERATIONS.md)](docs/OPERATIONS.md)**: 環境変数の設定方法、デプロイ手順、トラブルシューティング、セキュリティ上の注意点。
- **[開発者ガイド (DEVELOPMENT.md)](docs/DEVELOPMENT.md)**: ブリッジのクロスコンパイル手法（Homebrew配布等）やローカル開発環境の構築方法。

# Remote RAG (WikiSearcher)

Wikipediaのデータからテキストチャンクをベクトル化し、LanceDBを用いたハイブリッド検索（フルテキスト検索 + ベクトル検索）と、CrossEncoder（Reranker）を用いた高度な再評価を提供するRAGエンジンです。

## Dockerでの実行方法（推奨）

DockerとDocker Composeがインストールされている環境であれば、以下の手順ですぐに試すことができます。

### 1. コンテナのビルドと起動
```bash
docker-compose up -d
```

### 2. コンテナに入る
```bash
docker exec -it remote-rag bash
```

### 3. データの取り込み（Ingestion）
コンテナ内で以下のコマンドを実行し、WikipediaのデータをLanceDBに取り込みます。
```bash
# 100件のデータをサンプリングして取り込む場合
python -m src.ingestion.wikipedia_ingest_100
```
*(※初回実行時は、HuggingFaceからEmbeddingモデルとRerankerモデルが自動でダウンロードされます。)*

### 4. 検索CLIのテスト
取り込んだデータをCLIからテスト検索できます。
```bash
python search_cli.py "人工知能の歴史"
```

### 5. MCPサーバーとしての起動
外部のエージェントからMCPツールとして呼び出す場合、以下のコマンドでサーバーを起動します（標準入出力で通信します）。
```bash
python -m src.mcp_server.server
```

## ホストマシンのキャッシュについて
`docker-compose.yml` では、HuggingFaceのモデルキャッシュをDockerボリューム（`huggingface_cache`）に保存する設定になっています。これにより、コンテナを再起動しても数GBのAIモデルを毎回ダウンロードし直す必要はありません。また、取り込んだデータベースはホスト側の `data/` フォルダに同期されます。
