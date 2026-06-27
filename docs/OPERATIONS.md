# 運用手順書 (Operations Manual)

本ドキュメントでは、MCPプロキシ・認証基盤のデプロイ、監視、およびトラブルシューティングの手順を記載します。

## 1. 動作要件と環境変数

サーバー側（`deploy/compose.yaml`）を起動する前に、利用する認可サーバー（OAuth 2.0 / OIDC）の情報を環境変数として設定する必要があります。`deploy/` ディレクトリに `.env` ファイルを作成してください。

### サーバー側の環境変数 (`.env`)
```env
# 検証モードの選択 (jwks または introspect)
# デフォルトは jwks です (推奨)
OAUTH_VALIDATION_MODE=jwks

# [Introspectionモードの場合] 認可サーバーの Introspection API エンドポイント
OAUTH_INTROSPECT_URL=https://{your-idp-domain}/oauth2/v1/introspect
# サーバー検証用のクライアントIDとシークレット (Basic認証用)
OAUTH_CLIENT_ID=your-server-client-id
OAUTH_CLIENT_SECRET=your-server-client-secret
# Introspection結果のキャッシュTTL（秒）
AUTH_INTROSPECT_CACHE_TTL_SECONDS=60

# [JWKSモードの場合] 認可サーバーの JWKS URL
# OAUTH_VALIDATION_MODE=jwks の場合は必須です
# OAUTH_JWKS_URL=https://{your-idp-domain}/oauth2/v1/keys

# --- オプション: AIモデルの設定 ---
# 詳細は docs/MODELS.md を参照してください。
# EMBEDDING_MODEL=intfloat/multilingual-e5-base
# RERANKER_MODEL=cross-encoder/mmarco-mMiniLMv2-L12-H384-v1
```

### クライアント（Bridge）側の環境変数
ローカルでブリッジを利用するユーザーのPC上で設定します。
```bash
# OIDC Discoveryを利用して自動設定するための Issuer URL (推奨)
export OAUTH_ISSUER_URL=https://{your-idp-domain}
# ※ OAUTH_ISSUER_URL を指定しない場合は、以下の2つを個別に指定することも可能です
# export OAUTH_AUTH_URL=https://{your-idp-domain}/oauth2/v1/authorize
# export OAUTH_TOKEN_URL=https://{your-idp-domain}/oauth2/v1/token

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

`OAUTH_VALIDATION_MODE=introspect` の場合、認可サーバーへの負荷（レートリミット）を軽減するため、認証結果を Valkey (Redis互換) にキャッシュしています。
そのため、退職等により認可サーバー側で**アカウントを即時停止（Revoke）した場合でも、キャッシュが有効な期間（デフォルト60秒）はMCPサーバーにアクセスできてしまう**というタイムラグが発生します。
このタイムラグは `AUTH_INTROSPECT_CACHE_TTL_SECONDS` で調整可能です。

※ `OAUTH_VALIDATION_MODE=jwks` の場合、トークン自体の検証はローカルで都度行われるためキャッシュによる遅延はありませんが、トークンの有効期限 (`exp`) が切れるまでは無効化を検知できません。即時無効化の影響を小さくするには、IdP側でアクセストークンの有効期限を短く（例: 5〜15分）設定することを推奨します。

万が一、即時かつ強制的に全セッションを遮断する必要がある重大なインシデントが発生した場合は、Valkey(Redis) のキャッシュをフラッシュしてください。

```bash
cd deploy
docker compose exec valkey redis-cli FLUSHALL
```
