# アーキテクチャ設計書

本システムは、ローカル環境のAIエージェント（Claude Desktop, Cursorなど）から、セキュアな社内ネットワークやクラウド上のMCPサーバー（Model Context Protocol Server）へ接続するための認証・プロキシ基盤です。特定のベンダーに依存しない汎用的な **OAuth 2.0 / OIDC (RFC 7662: Token Introspection)** に準拠しています。

## 1. コンポーネント構成図

```mermaid
graph TD
    subgraph Client Environment [Mac / Claude Desktop]
        Agent[Claude Desktop / Cursor]
        Bridge[Bridge CLI<br>Go]
        KeyChain[(OS Keychain)]
    end

    subgraph Server Environment [Remote Linux / Windows]
        Caddy[Caddy Reverse Proxy<br>HTTPS / TLS]
        AuthHelper[Auth Helper<br>Go / Redis Cache]
        MCPServer[MCP Server<br>Python / FastMCP]
        LanceDB[(LanceDB<br>Vector + FTS)]
    end

    Agent -- stdio --> Bridge
    Bridge -- "1. Get/Refresh Token" --> KeyChain
    Bridge -- "2. HTTPS / SSE (Token Header)" --> Caddy
    
    Caddy -- "3. Forward Auth Request" --> AuthHelper
    AuthHelper -- "4. Introspection" --> IdentityProvider[Identity Provider<br>Okta / Auth0]
    
    Caddy -- "5. Proxy if Valid" --> MCPServer
    MCPServer -- "6. Hybrid Search" --> LanceDB
```

## 2. 認証シーケンス図

以下の構成により、アプリケーション（MCPサーバー）から認証の責務を完全に切り離した「多層防御アーキテクチャ」を実現しています。

```mermaid
sequenceDiagram
    participant User as ユーザー / ブラウザ
    participant AI as AIクライアント (Cursor等)
    participant Bridge as stdio-to-sse Bridge
    participant Caddy as Caddy (Proxy)
    participant Auth as Auth Helper
    participant Redis as Redis (Cache)
    participant IdP as 認可サーバー (OAuth/OIDC)
    participant MCP as MCP Server

    Note over Bridge: 1. 起動・トークン確認
    alt トークンなし or 期限切れ
        Bridge->>User: ブラウザ起動 (PKCE フロー)
        User->>IdP: ログインと認可
        IdP-->>Bridge: 認可コード
        Bridge->>IdP: コードとトークンの交換
        IdP-->>Bridge: アクセストークン (JWT/Opaque)
        Note over Bridge: OS Keychainに保存
    end

    Note over AI, Bridge: 2. MCP通信 (stdio)
    AI->>Bridge: JSON-RPC (stdin)
    
    Note over Bridge, MCP: 3. MCP通信 (HTTP/SSE)
    Bridge->>Caddy: POST /messages<br/>Authorization: Bearer <token>
    
    Caddy->>Auth: 認証委譲 (forward_auth)
    Auth->>Auth: トークンのハッシュ値(SHA-256)算出
    Auth->>Redis: GET <hash>
    
    alt キャッシュミス
        Auth->>IdP: POST /introspect (Basic Auth)
        IdP-->>Auth: active: true
        Auth->>Redis: SETEX <hash> 60 "valid"
    end
    
    Auth-->>Caddy: 200 OK
    Caddy->>MCP: プロキシリクエスト
    MCP-->>Caddy: JSON-RPC 応答
    Caddy-->>Bridge: SSE ストリーム
    Bridge-->>AI: JSON-RPC (stdout)
```

## 2. コンポーネント詳細

### 2.1 stdio-to-sse Bridge (ローカルクライアント)
*   **役割**: 標準入出力（stdio）しかサポートしていないAIクライアントに対し、HTTP POSTおよびSSE（Server-Sent Events）プロトコルへの透過的な変換を提供します。
*   **認証UX**: 起動時に有効なトークンが無い場合、自動的にOSのブラウザを開いて認可サーバーへ誘導する PKCE (Proof Key for Code Exchange) フローを実行します。
*   **セキュリティ**: 取得したトークンは平文ファイルではなく、OSが提供するネイティブのシークレットマネージャ（macOS Keychain, Windows Credential Manager等）に暗号化して保存されます。

### 2.2 Caddy (フロントプロキシ)
*   **役割**: 外部からのトラフィックの入り口。HTTPS化（自動証明書）やリバースプロキシを担います。
*   **認証の関所**: `forward_auth` ディレクティブを用い、すべてのリクエストをバックエンドの Auth Helper に問い合わせます。
*   **ストリーミング最適化**: MCPのSSE通信が途切れないよう、バッファリングを完全に無効化（`flush_interval -1`）しています。

### 2.3 Auth Helper (認証サイドカー)
*   **役割**: 渡された Bearer トークンが有効かどうかを検証します。
*   **オンライン検証**: RFC 7662 の Introspection エンドポイントを利用し、トークンが実際に有効（`active: true`）かを認可サーバーに直接問い合わせます。これにより Opaque Token でも安全に検証可能です。

### 2.4 Redis (キャッシュストア)
*   **役割**: 認可サーバーへの問い合わせによる API レートリミット枯渇や通信遅延を防ぐため、検証結果を一定時間（TTL: 1〜3分）保持します。
*   **セキュリティ**: トークン自体を保存すると漏洩時にリスクとなるため、トークンの **SHA-256ハッシュ値** をキーとして `"valid"` という状態のみを保存します。

### 2.5 MCP Server (アプリケーション)
*   **役割**: 実際のRAG検索やデータ処理を行うコアロジックです。
*   **責務の分離**: 認証に関するコードを一切持ちません。「Caddyを通過したリクエストはすべて安全である」という前提で動作します。
