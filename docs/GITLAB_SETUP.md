# GitLabを用いた設定例

本ドキュメントでは、`RRAG (Remote RAG)` の認証基盤として **GitLab** (gitlab.com または セルフホスト版GitLab) を利用するための詳細な設定手順を解説します。

GitLabを利用する場合、OAuthアプリケーションを作成する必要があります。
基本的には**クライアント側（Bridge CLI）用のアプリケーションを1つ作成するだけ**で稼働しますが、Introspectionモードを利用する場合のみサーバー側用にもう1つアプリケーションが必要です。

---

## 1. サーバー側（Auth Proxy）の設定

サーバーの `auth-helper` コンテナが、GitLab が発行したトークンの有効性を検証するための設定です。
RRAGはローカルでの **JWKSモード (推奨)** と、GitLabへ直接問い合わせる **Introspectionモード** の2つをサポートしています。

### パターンA: JWKS モード (推奨・設定が簡単)
JWKSモードを利用する場合、GitLab側での追加のアプリケーション作成は**不要**です。
`deploy/.env` に GitLab の JWKS URL を設定するだけで完了します。

```env
# 検証モードを JWKS に設定
OAUTH_VALIDATION_MODE=jwks

# GitLabの JWKS エンドポイント
# セルフホスト版の場合はドメイン部分をご自身の環境に合わせて変更してください
OAUTH_JWKS_URL=https://gitlab.com/oauth/discovery/keys
```

### パターンB: Introspection モード (オプション)
Opaqueトークンを利用したい場合や、キャッシュベースの検証を行いたい場合はこちらのモードを利用します。この場合、GitLab上で「Confidential アプリケーション」を作成し、Secretを発行する必要があります。

1. GitLabの左サイドバーから **Edit Profile > Applications** に移動し、「**Add new application**」をクリックします。
2. Name に任意の名前を入力し、Redirect URI にダミーのURL（例: `https://localhost/`）を入力します。
3. **Confidential** のチェックを **入れたまま** にし、Scopes は未選択のまま保存します。
4. 生成された **Application ID** と **Secret** を控えます。

`deploy/.env` に以下のように設定します。

```env
OAUTH_VALIDATION_MODE=introspect
OAUTH_INTROSPECT_URL=https://gitlab.com/oauth/introspect
OAUTH_CLIENT_ID={Auth Proxy Application ID}
OAUTH_CLIENT_SECRET={Auth Proxy Secret}
```

---

## 2. クライアント側（Bridge CLI）の設定

AIエージェントを実行するローカルPCから、ブラウザを通じてGitLabにログインしてトークンを取得するための設定です。

### GitLab管理画面での操作
1. 同様に **Applications** に移動し、「**Add new application**」をクリックします。
2. Name に任意の名前を入力します（例: `RRAG Bridge Client`）。
3. **Redirect URI** に `http://127.0.0.1:18080/callback` を入力します。
4. **Confidential のチェックを外します** (非常に重要: PKCEを利用するネイティブアプリのため、Secretを持たないパブリッククライアントとして登録します)。
5. Scopes として、最低限ユーザーを識別できる `read_user` または `api` を選択します。
6. 「Save application」をクリックし、生成された **Application ID** を控えます。

### クライアントPC側の環境変数設定
AIエージェントを実行するPCの環境変数、またはエージェントの起動スクリプト内で以下を設定します。

```bash
# GitLabの OIDC Discovery (Issuer) URL
# これを指定することで認可エンドポイントとトークンエンドポイントが自動で設定されます。
# セルフホスト版の場合はドメイン部分を変更してください。
export OAUTH_ISSUER_URL=https://gitlab.com

# 先ほど控えたPublicアプリケーションの Application ID
export OAUTH_CLIENT_ID={Bridge Client Application ID}

# (任意) ポート番号が 18080 以外の場合は指定してください。
# デフォルトで 18080 が使用されます。
# export OAUTH_REDIRECT_PORT=18080

# 接続先MCPサーバーのURL
export MCP_REMOTE_URL=https://your-caddy-server-domain
```

*(Cursor 等の AI エージェントは、起動したターミナルの環境変数を引き継ぐか、設定ファイル内で環境変数を指定できます。)*

---

## 3. (オプション) ユーザー属性に基づくフィルタリング設定

サーバー側(`auth-helper`)で、特定の「ユーザーID(`sub`)」や「メールドメイン」に基づいたアクセス制御を行うことができます。

> [!NOTE]
> 複数の環境変数（例: `AUTH_FILTER_EMAIL_DOMAINS` と `AUTH_FILTER_SUBJECTS`）を同時に設定した場合、それらは **AND条件** として評価されます。つまり、ユーザーは指定された条件をすべて満たしている必要があります。

### GitLabにおけるクレームの制約について
GitLabの Token Introspection API は仕様上、デフォルトでは `client_id`、`username`、`sub` などの情報を返却します。
もし `email` によるフィルタリング（`AUTH_FILTER_EMAIL_DOMAINS` など）を行いたい場合は、クライアント側（Bridge CLI）がトークンを取得する際の要求スコープに `email` を含める必要があります。（GitLab側のApplication設定でも `email` スコープにチェックを入れてください）

GitLabの場合、グループ情報をIntrospectionレスポンスに直接含めることは標準ではサポートされていないケースが多いため、エンタープライズ版でのSAML/OIDCグループ同期を利用するか、より確実な **`AUTH_FILTER_SUBJECTS`** (一意なユーザーID `sub` による制御) の利用を推奨します。

---

## トラブルシューティング

- **`Introspection failed` エラーが発生する場合**:
  - `deploy/.env` の `OAUTH_INTROSPECT_URL` が正しいドメインになっているか、`OAUTH_CLIENT_SECRET` が正しく設定されているか確認してください。
- **ブラウザでのログイン時、「The redirect URI included is not valid」エラーになる場合**:
  - GitLabに登録した Redirect URI と、クライアントPCで起動しているポート（デフォルト: `http://127.0.0.1:18080/callback`）が完全に一致しているか確認してください。
- **GitLabのトークンが `active: false` になる場合**:
  - サーバー側の `OAUTH_INTROSPECT_URL` への通信が遮断されていないか確認してください。また、GitLab側のアクセストークン有効期限が切れている可能性があります。Bridge CLIを再実行してトークンを取り直してください。
