# 運用手順書 (Operations Manual)

本ドキュメントでは、MCPプロキシ・認証基盤のデプロイ、監視、およびトラブルシューティングの手順を記載します。

## 1. 動作要件と環境変数

サーバー側（`docker-compose.yml`）を起動する前に、利用する認可サーバー（OAuth 2.0 / OIDC）の情報を環境変数として設定する必要があります。ルートディレクトリに `.env` ファイルを作成してください。

### サーバー側の環境変数 (`.env`)
```env
# 認可サーバーの Introspection API エンドポイント (※認証なしでテストする場合は空に設定)
OAUTH_INTROSPECT_URL=https://{your-idp-domain}/oauth2/v1/introspect

# サーバー検証用のクライアントIDとシークレット (Basic認証用)
OAUTH_CLIENT_ID=your-server-client-id
OAUTH_CLIENT_SECRET=your-server-client-secret

# --- オプション: AIモデルの設定 ---
# 詳細は docs/MODELS.md を参照してください。
# EMBEDDING_MODEL=intfloat/multilingual-e5-base
# RERANKER_MODEL=cross-encoder/mmarco-mMiniLMv2-L12-H384-v1
```

### クライアント（Bridge）側の環境変数
ローカルでブリッジを利用するユーザーのPC上で設定します。
```bash
# 認可エンドポイントとトークンエンドポイント
export OAUTH_AUTH_URL=https://{your-idp-domain}/oauth2/v1/authorize
export OAUTH_TOKEN_URL=https://{your-idp-domain}/oauth2/v1/token

# ローカルクライアント用の Client ID (PKCEを利用するためSecretは不要)
export OAUTH_CLIENT_ID=your-client-id

# 接続先MCPサーバーのURL
export MCP_REMOTE_URL=https://your-caddy-server-domain
```

## 2. デプロイ手順

本番環境またはステージング環境でのデプロイは、Docker Compose を使用して一括で起動します。

```bash
cd deploy

# イメージのビルドとバックグラウンド起動
docker compose up -d --build
```

### 2.1 コンテナの稼働確認
```bash
docker compose ps
```

## 3. 監視とトラブルシューティング

### 3.1 認証エラー (401 Unauthorized)
クライアントからアクセスできない場合、まずは `auth-helper` のログを確認します。
```bash
cd deploy
docker compose logs auth-helper
```
*   **原因の切り分け**:
    *   `Introspection failed`: 認可サーバーへの通信エラー、または環境変数の設定ミス（Client ID/Secretが間違っている）の可能性があります。
    *   ログが出ない場合: Caddyでリクエストがブロックされているか、クライアントが正しい Authorization ヘッダーを付与できていません。

### 3.2 AIからの応答が途切れる・タイムアウトする
SSEストリーミングが途切れる場合、Caddy のログを確認します。
```bash
cd deploy
docker compose logs caddy
```
*   **原因の切り分け**:
    *   Caddy と `mcp-server` 間の通信が切れている場合は、MCPサーバー側の処理負荷が高すぎる可能性があります。
    *   クライアントネットワーク側で長時間のHTTP接続が切断されている場合は、プロキシ層のタイムアウト設定を見直す必要があります。

## 4. セキュリティ上の留意事項 (重要)

### キャッシュによるアカウント停止のタイムラグ
本システムでは、認可サーバーへの負荷（レートリミット）を軽減するため、認証結果を Redis にキャッシュしています。
そのため、退職等により認可サーバー側で**アカウントを即時停止した場合でも、Redisのキャッシュが有効な期間（デフォルト60秒）はMCPサーバーにアクセスできてしまう**というタイムラグが発生します。

万が一、即時かつ強制的に全セッションを遮断する必要がある重大なインシデントが発生した場合は、Redis のキャッシュをフラッシュしてください。

```bash
cd deploy
docker compose exec redis redis-cli FLUSHALL
```
