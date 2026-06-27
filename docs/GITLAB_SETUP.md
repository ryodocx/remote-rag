# GitLabを用いた設定例

本ドキュメントでは、`RRAG (Remote RAG)` の認証基盤として **GitLab** (gitlab.com または セルフホスト版GitLab) を利用するための詳細な設定手順を解説します。

GitLabを利用する場合、**2つのOAuthアプリケーション**を作成する必要があります。
1. **サーバー側（Auth Proxy）用**: Confidential アプリケーション (APIでのIntrospection検証に使用)
2. **クライアント側（Bridge CLI）用**: Public アプリケーション (AIエージェントからのPKCEフローによるログインに使用)

---

## 1. サーバー側（Auth Proxy）の設定

サーバーの `auth-helper` コンテナが、GitLab に対してトークンの有効性を検証（Introspection）するための設定です。

### GitLab管理画面での操作
1. GitLabの左サイドバーから **Edit Profile > Applications** (または グループ/Adminエリアの Applications) に移動し、「**Add new application**」をクリックします。
2. Name に任意の名前を入力します（例: `RRAG Auth Proxy`）。
3. Redirect URI は使用しないため、ダミーのURL（例: `https://localhost/`）を入力します。
4. **Confidential** のチェックを **入れたまま** にします。
5. Scopes は何も選択しなくて構いません（トークン検証自体には特定のスコープは不要です）。
6. 「Save application」をクリックし、生成された **Application ID** と **Secret** を控えます。

### サーバー環境変数 (`deploy/.env`) の設定
控えた情報を `deploy/.env` に以下のように設定します。

```env
# GitLabの Token Introspection エンドポイント
# セルフホスト版の場合はドメイン部分をご自身の環境に合わせて変更してください
OAUTH_INTROSPECT_URL=https://gitlab.com/oauth/introspect

# 先ほど控えたConfidentialアプリケーションの Application ID と Secret
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

## トラブルシューティング

- **`Introspection failed` エラーが発生する場合**:
  - `deploy/.env` の `OAUTH_INTROSPECT_URL` が正しいドメインになっているか、`OAUTH_CLIENT_SECRET` が正しく設定されているか確認してください。
- **ブラウザでのログイン時、「The redirect URI included is not valid」エラーになる場合**:
  - GitLabに登録した Redirect URI と、クライアントPCで起動しているポート（デフォルト: `http://127.0.0.1:18080/callback`）が完全に一致しているか確認してください。
- **GitLabのトークンが `active: false` になる場合**:
  - サーバー側の `OAUTH_INTROSPECT_URL` への通信が遮断されていないか確認してください。また、GitLab側のアクセストークン有効期限が切れている可能性があります。Bridge CLIを再実行してトークンを取り直してください。
