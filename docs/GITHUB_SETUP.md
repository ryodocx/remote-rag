# GitHub を認証基盤として利用する場合の設定 (GitHub OAuth)

RRAGは標準で **OAuth 2.0 / OIDC (Token Introspection または JWKS)** に対応していますが、**GitHub が提供する標準の OAuth Apps および GitHub Apps は、完全な OIDC に準拠していません**。

このドキュメントでは、GitHubアカウントを用いてRRAGの認証を行うためのアーキテクチャ上の注意点と、それを実現するための回避策（IdPブローカーの利用）について解説します。

---

## ⚠️ アーキテクチャ上の制限

GitHubのOAuth実装には以下の特徴があり、RRAGの `auth-helper` (RFC 7662準拠) と直接通信することができません。

1. **JWT 形式ではない**: GitHubが発行するアクセストークン（`gho_...` など）はOpaque（不透明）な文字列であり、JWTではないため、ローカルでの署名検証（JWKSモード）が不可能です。
2. **RFC 7662 非対応**: GitHub API には、標準的な `/introspect` エンドポイントが存在しません。（代わりに `/user` エンドポイントを叩くか、Basic認証を用いた独自のトークン検証APIを叩く必要があります）
3. **OIDC Discovery 非対応**: `.well-known/openid-configuration` エンドポイントが提供されていません。

---

## 🛠️ 回避策: IdPブローカー（Auth0 / Dex 等）の利用

GitHubアカウントでログインさせるためには、GitHubとRRAGの間に「OIDCに完全に準拠したIdP（Identity Provider）」を挟む（フェデレーションする）方法が最も確実かつ安全です。

ここでは、最も導入が容易な **Auth0** を例に解説します。

### ステップ 1: GitHub側で OAuth App を作成する

1. GitHubにログインし、右上のアイコンから **Settings** > **Developer settings** > **OAuth Apps** を開きます。
2. **New OAuth App** をクリックします。
3. 以下のように入力して登録します。
   * **Application name**: `RRAG Auth0 Broker` (任意)
   * **Homepage URL**: `https://YOUR_AUTH0_DOMAIN.auth0.com`
   * **Authorization callback URL**: `https://YOUR_AUTH0_DOMAIN.auth0.com/login/callback` (Auth0のドメインを指定します)
4. 作成後、表示された **Client ID** と **Client Secret** を控えます。

### ステップ 2: Auth0 側で GitHub 連携を設定する

1. Auth0ダッシュボードにログインします。
2. 左メニューの **Authentication** > **Social** を開きます。
3. **Create Connection** をクリックし、**GitHub** を選択します。
4. ステップ1で控えた **Client ID** と **Client Secret** を入力し、必要な属性（Emailなど）にチェックを入れて保存します。

### ステップ 3: Auth0 側で RRAG 用の API と Application を作成する

この手順は、標準的な OIDC IdP の設定と同じです。

1. **Applications** > **Applications** から、RRAG Client (CLI) 用の **Native** アプリケーションを作成します。
   * Callback URL に `http://localhost:8080/callback` 等（Bridge CLI用）を設定します。
2. **Applications** > **APIs** から、RRAG Server 用の API を作成します。
   * Identifier に `https://rrag.your-company.com` などを設定します。
3. **Settings** から、Token Introspection エンドポイントのURL（または JWKS URL）を確認します。

### ステップ 4: RRAG サーバー（docker-compose）の環境変数設定

Auth0が発行するトークンは完全なJWT（または標準のIntrospection対応）であるため、RRAGの `auth-helper` はAuth0を向くように設定します。

```env
# 例: JWKS モードを利用する場合 (推奨)
OAUTH_VALIDATION_MODE=jwks
OAUTH_JWKS_URL=https://YOUR_AUTH0_DOMAIN.auth0.com/.well-known/jwks.json

# または Introspection モードを利用する場合
# OAUTH_VALIDATION_MODE=introspect
# OAUTH_INTROSPECT_URL=https://YOUR_AUTH0_DOMAIN.auth0.com/oauth/token/info
# OAUTH_CLIENT_ID=auth0-api-client-id
# OAUTH_CLIENT_SECRET=auth0-api-client-secret

# 特定のGitHub Organizationのメンバーのみを許可したい場合、
# Auth0のActions/RulesでトークンにカスタムクレームとしてOrganization情報を埋め込み、
# 以下のフィルタを利用することができます。
# AUTH_FILTER_GROUPS=your-github-org-name
```

### ステップ 5: ローカルPC（Bridge CLI）の環境変数設定

Bridge CLI も Auth0 に向けて設定します。

```bash
export OAUTH_ISSUER_URL=https://YOUR_AUTH0_DOMAIN.auth0.com/
export OAUTH_CLIENT_ID=auth0-native-app-client-id
export OAUTH_AUDIENCE=https://rrag.your-company.com
export MCP_REMOTE_URL=https://your-caddy-server-domain
```

以上の設定により、ユーザーがAIエージェントから検索を実行すると、Auth0のログイン画面が立ち上がり、「Continue with GitHub」ボタンからセキュアにログイン・アクセス制御を行うことが可能になります。
