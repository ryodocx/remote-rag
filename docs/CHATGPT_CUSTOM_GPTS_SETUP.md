# Custom GPTs / Actions 連携セットアップガイド

本ドキュメントでは、ChatGPT の Custom GPTs から **RRAG MCP Server** が提供する REST API（ハイブリッド検索など）を呼び出すための設定手順を解説します。

---

## 前提条件
- RRAG サーバーが HTTPS（例: `https://rrag.example.com`）でデプロイされ、外部からアクセス可能であること。
- IdP (Okta, Auth0, GitLab 等) に、OAuth アプリケーションが登録済みであること。
- ChatGPT Plus / Team / Enterprise アカウントを保持していること。

---

## 1. OpenAPI スキーマの取得

Custom GPTs の Actions に設定するためのスキーマ（`openapi.json`）を取得します。

サーバーがデプロイされたら、ブラウザまたは `curl` で以下にアクセスし、JSONファイルを保存します。
```bash
curl -O https://rrag.example.com/api/openapi.json
```
*(FastAPIは `/api/openapi.json` に OpenAPI スキーマを公開します)*

---

## 2. Custom GPTs の作成と Actions の設定

1. ChatGPT を開き、左下のメニューから **Explore GPTs** > **+ Create**（または右上の **Create**）を選択します。
2. **Configure** タブを開き、GPT の名前と説明を入力します。
3. 画面下部の **Create new action** をクリックします。

### Schema の設定
取得した `openapi.json` の内容を **Schema** エリアに貼り付けるか、「Import from URL」に `https://rrag.example.com/api/openapi.json` を入力してインポートします。

### Authentication の設定
スキーマの下にある **Authentication** の歯車アイコンをクリックし、以下のように設定します。

| 設定項目 | 入力内容 | 備考 |
|---|---|---|
| **Authentication Type** | `OAuth` | |
| **Client ID** | IdP のクライアントID | |
| **Client Secret** | IdP のクライアントシークレット | |
| **Authorization URL** | IdP の認可エンドポイント | 例: `https://<your-idp>/oauth2/v1/authorize` |
| **Token URL** | IdP のトークンエンドポイント | 例: `https://<your-idp>/oauth2/v1/token` |
| **Scope** | `openid profile email` など | IdP で必要なスコープを指定 |
| **Token Exchange Method** | `Default (POST)` | |

設定を保存すると、画面下部に **Callback URL**（例: `https://chat.openai.com/aip/<GPT_ID>/oauth/callback`）が表示されます。これをコピーしてください。

---

## 3. IdP への Redirect URI 登録

コピーした Callback URL を、IdP (Okta など) の OAuth アプリケーション設定画面で **Allowed Redirect URIs (Sign-in redirect URIs)** に追加します。

> **参考:** Okta をご利用の場合は、[OKTA_SETUP.md](OKTA_SETUP.md) も併せてご参照ください。

---

## 4. プロンプト (Instructions) の設定例

GPT が適切に検索 API を使用できるように、**Instructions** に以下のような指示を追加することをおすすめします。

```text
あなたは社内ナレッジベースのアシスタントです。
ユーザーからの質問に対して、必ず提供されている「Search」アクション（/api/search）を使って社内情報を検索し、その結果に基づいて回答してください。
検索する際は、適切なキーワードや自然言語のクエリを用いてください。
もしユーザーが認証（ログイン）を求められた場合は、ログインを行ってから再度質問するように案内してください。
```

---

## 5. 動作テスト

1. 画面右側のプレビューウィンドウで、「社内の◯◯について教えて」と質問します。
2. 初回は「**Sign in to [あなたのAPI名]**」というボタンが表示されます。
3. ボタンをクリックすると IdP のログイン画面に遷移し、認証を完了します。
4. ログイン後、GPT が自動的に `search_api` を呼び出し、検索結果に基づいた回答を返せば成功です。
