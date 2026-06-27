# Webアプリケーション / REST API 連携ガイド

本ドキュメントでは、ブラウザ版の AI チャット UI や各種 Web アプリケーション、ワークフロー自動化ツールから、本プロジェクト（RRAG MCP Server）の検索機能へ連携する方法について解説します。

---

## 1. MCPと REST API (OpenAPI) の使い分け

現在、Model Context Protocol (MCP) は主に **ローカルデスクトップアプリ（Claude Desktop, Cursor, Windsurf 等）向け** に先行して普及しているプロトコルです。
そのため、一般的な Web アプリケーションやワークフロー自動化ツール（SaaS 等）は、現時点では MCP プロトコル自体にネイティブ対応していません。

この課題を解決するため、本プロジェクトでは **REST API (OpenAPI 3.0) エンドポイントを標準搭載** しています。
これにより、外部の Web アプリケーションや自動化ツールが、標準的な **REST API リクエストと OAuth 2.0 認証** をサポートしていれば、社内のナレッジ検索をシームレスに統合できます。

---

## 2. AI チャット UI プラットフォームとの連携

### A. ChatGPT (Custom GPTs / Actions)
RRAG が AI チャット環境において最もスムーズに連携できるのが、ChatGPT Plus / Team / Enterprise ユーザー向けの **Custom GPTs** です。
- **設定方法**: RRAG サーバーの `/v1/api/openapi.json` を GPT Builder にインポートし、Authentication で `OAuth` を選択して IdP の情報を設定します。
- **詳細手順**: 専用のセットアップガイド [CHATGPT_CUSTOM_GPTS_SETUP.md](CHATGPT_CUSTOM_GPTS_SETUP.md) をご参照ください。

### B. Microsoft Copilot (Copilot Studio)
Microsoft の企業向け環境である **Copilot Studio** でも、ChatGPT Actions と同様のアーキテクチャで連携が可能です。
- Copilot Studio の管理画面から「カスタム コネクタ」を作成し、OpenAPI スキーマをインポートします。
- セキュリティ設定で「OAuth 2.0」を選択し、Entra ID (Azure AD) 等のクレデンシャルを設定することでエージェントから検索可能になります。
- **詳細手順**: 専用のセットアップガイド [COPILOT_STUDIO_SETUP.md](COPILOT_STUDIO_SETUP.md) をご参照ください。

### C. 制約と代替案 (Claude Web / Gemini Web)
> [!WARNING]
> **Claude Web 版 や Gemini Web 版 では、現在のところ一般ユーザー向けの外部 API 連携（OpenAPI インポート等）機能が提供されていません。**

Claude や Gemini の強力なモデルを利用して社内ナレッジを検索したい場合、解決策は以下の2つです。

1. **ローカルアプリ (MCP) の利用（推奨）**:
   ブラウザ版にこだわらない場合、**Claude Desktop アプリケーション** や **gemini CLI** など、MCP にネイティブ対応したローカルツールを利用するのが最も簡単でスムーズな方法です（本プロジェクトの基本アーキテクチャである `rrag-bridge` をそのまま利用できます）。
2. **オープンソースチャット UI の自社構築**:
   どうしてもブラウザベースの Web UI で利用したい場合は、後述する **Dify** や **Open WebUI** などのオープンソースチャット UI を自前で構築し、そこに RRAG の REST API を組み込むアプローチが推奨されます。

---

## 3. ワークフロー自動化ツールとの連携

以下のサービスのような **iPaaS (Integration Platform as a Service)** を利用することで、特定のトリガー（例: Slack のメッセージ、メール受信など）に応じて RRAG のナレッジ検索を自動実行するワークフローをノーコードで構築できます。

### A. Zapier / Make (Integromat)
- **Zapier**: 「API by Zapier」機能を利用し、Authentication タイプとして OAuth 2.0 (または Bearer Token) を設定して RRAG への HTTP リクエストを行います。
- **Make**: 「Custom App」を作成するか、または汎用の「HTTP > Make an OAuth 2.0 request」モジュールを利用することで、IdP と連携したセキュアなリクエストが可能です。

### B. Microsoft Power Automate
- Copilot Studio と同様に、Power Automate 内で **「カスタム コネクタ」** を作成し、RRAG の OpenAPI (`/v1/api/openapi.json`) をインポートします。
- セキュリティタブで OAuth 2.0 認証 (Azure AD または Generic OAuth) を構成することで、フロー内のステップとして検索アクションを利用できます。

### C. n8n
- オープンソースのワークフローエンジンである n8n では、「HTTP Request」ノードを利用し、Credentials 設定で `OAuth2 API` を作成することで RRAG サーバーと安全に連携できます。

---

## 4. Web アプリ・社内ポータル構築ツールとの連携

### A. Dify / LangFlow 等の生成 AI プラットフォーム
これらのプラットフォームは、外部の OpenAPI スキーマを読み込んで独自の「ツール」として定義する機能を備えています。RRAG の `/v1/api/openapi.json` をインポートし、OAuth2 または API Key 認証を設定することで、自作のチャットボットやエージェントフロー内に検索を組み込めます。

### B. Open WebUI / LibreChat 等のオープンソースチャット
Open WebUI の Functions 機能などを利用して、RRAG の REST API (`/v1/api/search`) を呼び出す Python スクリプトを記述することで、検索結果をチャットコンテキストに統合できます。

### C. Retool / Appsmith / ToolJet / Budibase 等のローコード基盤
これらの「社内業務ツール構築プラットフォーム (Internal Tool Builders)」を利用して、カスタムナレッジ検索 UI やダッシュボードを構築できます。
外部 REST API データソースの追加を標準でサポートしており、設定画面から OAuth 2.0 認証を通じたセキュアなエンドポイントへのリクエストが容易に行えます。

### D. カスタムフロントエンド / Backstage
- **自社開発のフロントエンド (React / Vue 等)**: フロントエンド側で IdP に対するログインを行い、取得した JWT (アクセストークン) を `Authorization: Bearer <token>` ヘッダーに付与して RRAG の API を呼び出します（※CORS の設定が必要です）。
- **Backstage**: Spotify が開発する開発者ポータルから、バックエンドプラグインを通じて OAuth 2.0 認証（または Client Credentials フロー）を行い、プロキシ経由で RRAG サーバーへセキュアにアクセスするアーキテクチャが構築できます。
