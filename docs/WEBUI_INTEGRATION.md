# AIチャット Web UIからの連携ガイド

本ドキュメントでは、ChatGPT、Claude、Gemini、Copilot などの **ブラウザ版 AI チャット（Web UI）** から、本プロジェクト（RRAG MCP Server）と連携する方法について解説します。

---

## 1. 結論：MCPのネイティブ対応状況

結論から言うと、現在 Model Context Protocol (MCP) は **ローカルデスクトップアプリ（Claude Desktop, Cursor, Windsurf, Zed 等）向け** に最適化されたプロトコルです。

そのため、2026年現在の仕様では、**ChatGPT, Claude, Gemini などのブラウザ版 Web UI から、直接 MCP サーバーを登録・接続する標準機能は提供されていません。**

Web UI から本システムの高精度な RAG 検索を利用したい場合は、MCP プロトコルをそのまま使うのではなく、**REST API (OpenAPI)** としてインターフェースを公開し、各プラットフォームの拡張機能を利用する代替アプローチ（ワークアラウンド）が必要になります。

---

## 2. Web UI と連携するための代替アプローチ

各プラットフォームの仕様に合わせた連携方法を以下に示します。

### A. ChatGPT (Custom GPTs / Actions)

ChatGPT Plus / Team / Enterprise ユーザーが利用できる **Custom GPTs の「Actions」機能** を利用することで連携が可能です。

1. **REST API ラッパーの作成**
   現在の `FastMCP` は MCP プロトコル（JSON-RPC 形式）で通信するため、ChatGPT からは直接呼べません。代わりに、`searcher.py` の `WikiSearcher` クラスを呼び出す、シンプルな FastAPI の REST エンドポイントを作成します。
   ```python
   # 例: 検索用RESTエンドポイント
   @app.get("/search")
   def search_endpoint(query: str, limit: int = 5):
       searcher = WikiSearcher()
       return searcher.search(query=query, limit=limit)
   ```
2. **OpenAPI スキーマの発行**
   FastAPI が自動生成する `/openapi.json` を取得し、Custom GPTs の Actions にインポートします。
3. **認証の設定**
   Custom GPTs の Actions 設定画面で `Authentication` を `OAuth` に設定し、お使いの IdP (Okta等) の Client ID / Secret と認可URLを設定します。これにより、ユーザーが GPTs を使う際に OAuth ログインが走り、発行された Bearer トークンで本プロジェクトの `auth-helper`（プロキシ）を通過できるようになります。

### B. Microsoft Copilot (Copilot Studio)

Copilot の企業向け環境で利用できる **Copilot Studio** を経由することで連携可能です。

1. ChatGPT と同様に、REST API (OpenAPI スキーマ) を用意します。
2. Copilot Studio にアクセスし、「カスタム コパイロット」または既存のコパイロットの「アクションを追加」から、OpenAPI スキーマをインポートしてプラグインとして登録します。
3. Entra ID などの OAuth 認証と組み合わせることで、社内のアクセス権限を持ったユーザーのみに検索を許可できます。

### C. Claude Web / Gemini Web

現時点において、Claude Web 版 や Gemini Web 版のコンシューマー向け画面には、ユーザーが任意の OpenAPI エンドポイントをワンクリックで追加する機能（ChatGPT の Custom GPTs に相当するもの）はありません。
*(※ 開発者向け API を用いた Tool Use 機能は存在しますが、ブラウザのチャット画面とは独立しています)*

**推奨される解決策:**
ブラウザベースで Claude や Gemini の強力なモデルと本 RAG システムを組み合わせたい場合は、**Dify**, **LangFlow**, **Streamlit**, **Chatbot UI** などのオープンソースのチャット UI フレームワークを自前でデプロイし、そこに本システムの検索 API (REST) をツールとして組み込むアプローチが最も確実です。

---

## 3. 今後の展望 (MCP-to-OpenAPI ブリッジ)

オープンソースコミュニティでは、既存の MCP サーバーを自動的に OpenAPI 準拠の REST API に変換する「MCP-to-OpenAPI ブリッジ」の開発が進められています。
将来的には、本プロジェクトの MCP サーバーにブリッジツールを被せるだけで、コードを一切書かずに ChatGPT (Custom GPTs) と連携できるようになる見込みです。

また、AI プラットフォーム側（OpenAI, Google 等）が将来的に標準で MCP プロトコルをサポートするようになれば、設定画面にエンドポイントURLを入力するだけで Web UI から直接連携が可能になります。

現時点では、最もシームレスで体験が良いのは **Claude Desktop や Cursor などのネイティブ MCP 対応デスクトップアプリ** での利用となります。
