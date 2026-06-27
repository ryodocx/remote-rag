# Oktaを用いた設定例

本ドキュメントでは、`RRAG (Remote RAG)` の認証基盤として **Okta** (OAuth 2.0 / OIDC) を利用するための詳細な設定手順を解説します。

Oktaを利用する場合、アプリケーション（App Integration）を作成する必要があります。
基本的には**クライアント側（Bridge CLI）用のアプリケーションを1つ作成するだけ**で稼働しますが、Introspectionモードを利用する場合のみサーバー側用にもう1つアプリケーションが必要です。

---

## 1. サーバー側（Auth Proxy）の設定

サーバーの `auth-helper` コンテナが、Okta が発行したトークンの有効性を検証するための設定です。
RRAGはローカルでの **JWKSモード (推奨)** と、Oktaへ直接問い合わせる **Introspectionモード** の2つをサポートしています。

### パターンA: JWKS モード (推奨・設定が簡単)
JWKSモードを利用する場合、Okta側での追加のアプリケーション作成は**不要**です。
`deploy/.env` に Okta の JWKS URL を設定するだけで完了します。

```env
# 検証モードを JWKS に設定
OAUTH_VALIDATION_MODE=jwks

# OktaのカスタムAuthorization ServerのJWKSエンドポイント
# (デフォルトのAuthorization Serverを使用する場合の例)
OAUTH_JWKS_URL=https://{your-okta-domain}/oauth2/default/v1/keys
```

### パターンB: Introspection モード (オプション)
Opaqueトークンを利用せざるを得ない場合や、強制失効・キャッシュベースの検証を行いたい場合はこちらのモードを利用します。この場合、Okta管理画面で「API Services」アプリケーションを作成し、Client Secretを発行する必要があります。

> [!NOTE]
> **なぜOpaqueトークンを利用するのか？ (Okta特有の制約)**
> OktaでJWKS検証が可能な「JWT形式のアクセストークン」を発行するには、Oktaの有償オプションである **API Access Management (Custom Authorization Server)** が必要です（例: `/oauth2/default` などのエンドポイント）。
> 
> **※ オプション契約の有無の見分け方:**
> 1. **管理画面での確認**: Okta管理画面で **Security > API** を開いた際、**「Authorization Servers」** というタブが存在し、そこに `default` などのサーバーがリストされていれば契約あり（JWKSモード利用可能）です。タブ自体が存在しない場合は未契約です。
> 2. **トークン形式での確認**: 発行されたアクセストークンが `eyJ...` から始まるドット(`.`)区切りの文字列であれば JWT ですが、40文字程度のランダムな文字列であれば Opaqueトークン です。
> 
> もしこのオプションを契約しておらず、標準の **Org Authorization Server**（例: `/oauth2/v1/token` などのルートエンドポイント）を使用する場合、発行されるアクセストークンは必ず **Opaqueトークン** になります。OpaqueトークンはローカルでのJWKS署名検証が不可能なため、この「Introspection モード」を使用して Okta サーバーへ直接有効性を問い合わせる必要があります。

1. **Applications > Applications** に移動し、「**Create App Integration**」をクリック。
2. **API Services** を選択。
3. アプリケーション名を入力して作成。
4. アプリ作成後、**Client ID** と **Client Secret** を控えます。

`deploy/.env` に以下のように設定します。

```env
OAUTH_VALIDATION_MODE=introspect
OAUTH_INTROSPECT_URL=https://{your-okta-domain}/oauth2/default/v1/introspect
OAUTH_CLIENT_ID={API Services App Client ID}
OAUTH_CLIENT_SECRET={API Services App Client Secret}
```

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

# 接続先MCPサーバーのURL
export MCP_REMOTE_URL=https://your-caddy-server-domain
```

*(Cursor 等の AI エージェントは、起動したターミナルの環境変数を引き継ぐか、設定ファイル内で環境変数を指定できます。)*

---

## 3. (オプション) ユーザー属性に基づくフィルタリング設定

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
