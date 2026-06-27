# Oktaを用いた設定例

本ドキュメントでは、`RRAG (Remote RAG)` の認証基盤として **Okta** (OAuth 2.0 / OIDC) を利用するための詳細な設定手順を解説します。

Oktaを利用する場合、アプリケーション（App Integration）を作成する必要があります。
基本的には**クライアント側（Bridge CLI）用のアプリケーションを1つ作成するだけ**で稼働しますが、Introspectionモードを利用する場合のみサーバー側用にもう1つアプリケーションが必要です。

---

## 1. サーバー側（Auth Proxy）の設定

サーバーの `auth-helper` コンテナは、JWKSモードとIntrospectionモードの両方を**同時**に提供します。用途に応じて環境変数を設定してください。

`deploy/.env` に以下の環境変数を設定します。

```env
# --- JWKS 共通設定 ---
# OktaのAuthorization ServerのJWKSエンドポイント
OAUTH_JWKS_URL=https://{your-okta-domain}/oauth2/v1/keys

# --- /id_token/* ルート用の設定 (ローカルツール向け) ---
# ローカルツール用には Client ID の一致確認 (aud) が必須です
OAUTH_CLIENT_ID={Native App Client ID}

# --- /jwt/* ルート用の設定 (JWTアクセストークン向け) ---
# APIのResource URIを指定します (例: api://default)
AUTH_EXPECTED_AUD=api://default

# --- /introspect/* ルート用の設定 (Opaqueトークン向け) ---
OAUTH_INTROSPECT_URL=https://{your-okta-domain}/oauth2/v1/introspect
OAUTH_CLIENT_SECRET={API Services App Client Secret}
```

> [!IMPORTANT]
> **トークンの種類に応じたURLの使い分け**
> CaddyはアクセスするURLのプレフィックスによって検証方法を動的に切り替えます。
> - **`/id_token/*`**: ローカルツール (Bridge CLI) から送信される **IDトークン** をJWKSで検証します (`aud` は `OAUTH_CLIENT_ID` と一致すること)。
> - **`/jwt/*`**: API Access Management等で発行された **JWTアクセストークン** をJWKSで検証します (`aud` は `AUTH_EXPECTED_AUD` と一致すること)。
> - **`/introspect/*`**: Org Authorization Serverで発行された **Opaqueトークン** をOktaへ問い合わせて検証します。

---

## 2. クライアント側（Bridge CLI）の設定

AIエージェントを実行するローカルPCから、ブラウザを通じてOktaにログインしてトークンを取得するための設定です。

### Okta管理画面での操作
1. **Applications > Applications** に移動し、「**Create App Integration**」をクリックします。
2. Sign-in method で **OIDC - OpenID Connect** を選択します。
3. Application type で **Native Application** を選択し、「Next」をクリックします。
4. App integration name に任意の名前を入力します（例: `RRAG Bridge Client`）。
5. Grant type で **Authorization Code** がチェックされていることを確認します（PKCEが自動適用されます）。
6. **Sign-in redirect URIs** に `http://127.0.0.1:18080/callback` を追加します。
   *(※Oktaは完全一致のRedirect URIを要求するため、ポート番号を固定する必要があります)*
7. Assignments で、利用を許可するユーザーやグループをアサインし、「Save」をクリックします。
8. 作成されたアプリの **General** タブから **Client ID** を控えます。
   *(※Native Appのため、Client Secretは発行されません)*

### クライアントPC側の環境変数設定
AIエージェントを実行するPCの環境変数、またはエージェントの起動スクリプト内で以下を設定します。

```bash
# OktaのOIDC Discovery (Issuer) URL
# これを指定することで認可エンドポイントとトークンエンドポイントが自動で設定されます。
export OAUTH_ISSUER_URL=https://{your-okta-domain}/oauth2/default

# 先ほど控えたNative ApplicationのClient ID
export OAUTH_CLIENT_ID={Native App Client ID}

# (任意) Oktaで設定したRedirect URIのポート番号が 18080 以外の場合は指定してください。
# デフォルトで 18080 が使用されるため、18080を設定した場合はこの環境変数の指定は不要です。
# export OAUTH_REDIRECT_PORT=18080

# IDトークンを送信するための設定（/id_token/* ルート用）
export USE_ID_TOKEN=true

# 接続先MCPサーバーのURL (/id_token/mcp/* プレフィックスを指定)
export MCP_REMOTE_URL=https://your-caddy-server-domain/id_token/mcp/sse
```

*(Cursor 等の AI エージェントは、起動したターミナルの環境変数を引き継ぐか、設定ファイル内で環境変数を指定できます。)*

---

## 3. Custom GPTs / Actions を利用する場合の設定 (Web UI連携)

ChatGPT の **Custom GPTs (Actions)** 経由で RRAG にアクセスさせたい場合、Okta 上で以下の設定を追加で行う必要があります。

### 3.1. Web アプリケーションの作成 (または既存アプリへの Redirect URI 追加)
Custom GPTs の OAuth 連携は **Web アプリケーション (Confidential Client)** として動作します。

1. **Applications > Applications** に移動し、「**Create App Integration**」をクリックします。
2. Sign-in method で **OIDC - OpenID Connect** を選択します。
3. Application type で **Web Application** を選択し、「Next」をクリックします。
   *(※ 既存のWebアプリがある場合は、その設定を開いて Redirect URI を追加するだけでも構いません)*
4. **Sign-in redirect URIs** に、OpenAI から提供される Callback URL を追加します。
   - 例: `https://chat.openai.com/aip/g-xxxxxxxx/oauth/callback`
5. 保存後、発行された **Client ID** と **Client Secret** を控えます。これを Custom GPTs の Authentication 画面に設定します。

> **参考:** Custom GPTs 側の詳細な設定手順については、[CHATGPT_CUSTOM_GPTS_SETUP.md](CHATGPT_CUSTOM_GPTS_SETUP.md) を参照してください。

---

## 4. (オプション) ユーザー属性に基づくフィルタリング設定

サーバー側(`auth-helper`)で、特定の「メールドメイン」や「グループ」に所属するユーザーのみアクセスを許可するフィルタリングを行いたい場合、Oktaのトークン（または Introspection レスポンス）に `email` や `groups` を含める必要があります。

> [!NOTE]
> 複数の環境変数（例: `AUTH_FILTER_EMAIL_DOMAINS` と `AUTH_FILTER_GROUPS`）を同時に設定した場合、それらは **AND条件** として評価されます。つまり、ユーザーは指定されたメールドメインを持ち、かつ指定されたグループのいずれかに所属している必要があります。

### Authorization Server の設定 (カスタムクレームの追加)
1. **Security > API > Authorization Servers** に移動し、使用しているサーバー（例: `default`）をクリックします。
2. **Claims** タブを開き、「**Add Claim**」をクリックします。
3. `email` クレームの追加:
   - **Name**: `email`
   - **Include in token type**: `Access Token` / `Always` (または条件指定)
   - **Value type**: `Expression`
   - **Value**: `user.email`
4. `groups` クレームの追加:
   - **Name**: `groups`
   - **Include in token type**: `Access Token` / `Always`
   - **Value type**: `Groups`
   - **Filter**: `Matches regex` `.*` (すべてのグループを含める場合)
5. 保存後、`auth-helper` の環境変数（`AUTH_FILTER_EMAIL_DOMAINS` や `AUTH_FILTER_GROUPS` など）を設定することでフィルタリングが有効になります。

---

## トラブルシューティング

- **`Introspection failed` エラーが発生する場合**:
  - `deploy/.env` の `OAUTH_INTROSPECT_URL` が正しいか確認してください。Oktaでは、APIアクセス保護用に Custom Authorization Server (`/oauth2/default/...` など) を使うのが一般的ですが、Okta orgのルート (`/oauth2/v1/introspect`) を指定すると動作しない場合があります。
- **ブラウザでのログイン後、「Redirect URI mismatch」エラーになる場合**:
  - クライアント環境変数 `OAUTH_REDIRECT_PORT=18080` が正しくBridge CLIに読み込まれているか、Okta側に登録したURI `http://127.0.0.1:18080/callback` と一字一句一致しているか確認してください。
