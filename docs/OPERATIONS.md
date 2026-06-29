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
AUTH_INTROSPECT_CACHE_TTL_SECONDS=600

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

### 高度な環境変数オプション (Advanced Configurations)
特定のテストやカスタマイズが必要な場合に利用可能です。

| コンポーネント | 環境変数名 | デフォルト値 | 用途 |
| :--- | :--- | :--- | :--- |
| **client** | `RRAG_PROFILE` | `""` | 複数アカウント切り替え時のキーリング名接尾辞として使用。 |
| **server (Go)** | `MOCK_AUTH` | `""` | `true` に設定すると外部IdPへの検証をスキップし全リクエストを許可（ローカルテスト用）。**注意**: 認可サーバーのURL環境変数が空であっても、この変数が `true` でない場合は認証エラーになります。 |
| **server (Python)** | `ENABLE_INGEST_API` | `"false"` | `"true"` でドキュメント取り込み用の `/ingest` API を有効化。 |
| **server (Python)** | `LANCEDB_PATH` | `"server/core/data/lancedb"` | LanceDBのデータベース保存先ディレクトリを指定。 |
| **server (Python)** | `VECTOR_DIM` | 未設定 | ベクトル埋め込み次元数を明示。設定するとモデル事前読み込みを抑止可能。 |
| **server (Python)** | `WIKI_SEARCH_MAX_TOKENS` | `4000` | 検索結果からLLMに渡すコンテキストの最大トークン数を制限。 |
| **server (Python)** | `DEFAULT_RELEVANCE_THRESHOLD` | `-1.0` | リランカースコアの最低基準値。 |
| **server (Python)** | `EXACT_MATCH_RELEVANCE_THRESHOLD`| `-5.0` | 完全一致テキスト判定時の閾値緩和幅。 |

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
    *   Caddy と `rrag-server` 間の通信が切れている場合は、MCPサーバー側の処理負荷が高すぎる可能性があります。
    *   クライアントネットワーク側で長時間のHTTP接続が切断されている場合は、プロキシ層のタイムアウト設定を見直す必要があります。

## 4. セキュリティ上の留意事項 (重要)

### キャッシュによるアカウント停止のタイムラグ

`OAUTH_VALIDATION_MODE=introspect` の場合、認可サーバーへの負荷（レートリミット）を軽減するため、認証結果を Valkey (Redis互換) にキャッシュしています。
そのため、退職等により認可サーバー側で**アカウントを即時停止（Revoke）した場合でも、キャッシュが有効な期間（デフォルト600秒）はMCPサーバーにアクセスできてしまう**というタイムラグが発生します。
このタイムラグは `AUTH_INTROSPECT_CACHE_TTL_SECONDS` で調整可能です。

※ `OAUTH_VALIDATION_MODE=jwks` の場合、トークン自体の検証はローカルで都度行われるためキャッシュによる遅延はありませんが、トークンの有効期限 (`exp`) が切れるまでは無効化を検知できません。即時無効化の影響を小さくするには、IdP側でアクセストークンの有効期限を短く（例: 5〜15分）設定することを推奨します。

万が一、即時かつ強制的に全セッションを遮断する必要がある重大なインシデントが発生した場合は、Valkey(Redis) のキャッシュをフラッシュしてください。

```bash
cd deploy
docker compose exec valkey redis-cli FLUSHALL
```

---

## 5. 属性ベースアクセス制御 (ABAC) の詳細設定

`auth-helper` は、トークンの検証成功後に、トークン内の各種クレーム（Claims）を検証する「属性ベースアクセス制御 (ABAC)」をサポートしています。

### 5.1 評価ロジックルール
- **カテゴリ間 (AND条件)**: 指定した複数のフィルタ環境変数（例: `AUTH_FILTER_EMAIL_DOMAINS` と `AUTH_FILTER_GROUPS` 両方）が存在する場合、**すべての条件を同時に満たす**必要があります。どれか一つでも満たさないリクエストは `403 Forbidden` として拒否されます。
- **カテゴリ内 (OR条件 / リスト指定)**: カンマ区切りで複数の値を指定できる環境変数（例: `AUTH_FILTER_GROUPS=admin,developer`）では、トークン側の値がいずれか**一つでも一致すれば**条件を満たしたとみなされます。

### 5.2 ABAC フィルタ環境変数一覧

| 環境変数名 | 対象クレーム | 指定形式 | 説明 |
| :--- | :--- | :--- | :--- |
| `AUTH_FILTER_ISS` | `iss` | 文字列 (完全一致) | トークンを発行した認可サーバー（Issuer）の識別URLを検証。 |
| `AUTH_FILTER_AUD` | `aud` | 文字列 (完全一致) | トークンの対象読者（Audience）を検証。トークン内の `aud` クレームが配列の場合はその中に一致するものがあるかを検証。 |
| `AUTH_FILTER_CLIENT_ID` | `client_id` | 文字列 (完全一致) | リクエストを行ったOAuthクライアントIDを検証。 |
| `AUTH_FILTER_SCOPES` | `scope` | 文字列 (完全一致) | トークンに付与されているスコープを検証（スペース区切りの中から一致するものを検証）。 |
| `AUTH_FILTER_EMAIL_DOMAINS`| `email` | カンマ区切り文字列 | 許可するメールアドレスのドメイン（例: `example.com,test.org`）。アドレス末尾の `@domain` 部分を前方一致のように後方一致で検証。 |
| `AUTH_FILTER_EMAILS` | `email` | カンマ区切り文字列 | 許可する具体的なメールアドレス（完全一致）。 |
| `AUTH_FILTER_GROUPS` | `groups` | カンマ区切り文字列 | 許可するユーザーグループ（例: `admin,engineers`）。トークン内の `groups` クレームが文字列または配列のいずれであっても適合。 |
| `AUTH_FILTER_SUBJECTS` | `sub` | カンマ区切り文字列 | 許可する特定のユーザー識別子（Subject）。 |

### 5.3 トークンペイロードとフィルタ設定例

#### 適合するIDトークン / JWTアクセストークンのペイロード例
```json
{
  "iss": "https://identity.example.com",
  "sub": "user_123456",
  "aud": "rrag-client-app",
  "client_id": "client_abc123",
  "exp": 1719669600,
  "scope": "openid email profile mcp:access",
  "email": "developer@example.com",
  "groups": ["developers", "r-and-d"]
}
```

#### フィルタ設定例 (.env)
上記のトークンを持つユーザーのみアクセスを許可する場合の、極めて厳格な設定例です。

```dotenv
# 発行元とAudienceの制限 (Caddyfile側から渡される aud との一致)
AUTH_FILTER_ISS=https://identity.example.com
AUTH_FILTER_AUD=rrag-client-app

# メールドメインと所属グループの絞り込み (AND条件で評価)
AUTH_FILTER_EMAIL_DOMAINS=example.com
AUTH_FILTER_GROUPS=developers,administrators
```

この設定の場合、メールアドレスが `@example.com` で終わり、かつ所属グループ（`groups`）に `developers` または `administrators` のいずれかを含むトークンのみが認証を通過します。

