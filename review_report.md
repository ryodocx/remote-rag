# Remote RAG システム コードベース & ドキュメント統合レビューレポート

## 1. 概要 (Executive Summary)
本レポートは、`remote-rag` リポジトリに含まれる以下のコンポーネントを対象に実施した、包括的なコードベースおよびドキュメントの監査結果をまとめたものです。
- **クライアント・ブリッジ** (`client/bridge/`, Go言語実装)
- **認証プロキシ・ヘルパー** (`server/auth-helper/`, Go言語実装)
- **RAGコア・サーバー** (`server/core/`, Python/FastAPI/LanceDB実装)
- **デプロイメント設定** (Docker Compose, Helm Chart, Caddyfile)
- **各種ドキュメント** (`README.md`, `README_ja.md`, `docs/*`)

監査の結果、ドキュメントの記載内容と実際のコード実装における重要な乖離（キャッシュTTLの不一致、No-Authモードの仕様など）、Docker Compose起動時におけるJWKS検証フローを破壊する設定不備、およびクライアント/サーバーにおけるアーキテクチャ上の堅牢性・パフォーマンスの課題（送信処理のスレッドブロッキング、トークン失効時のデッドロック、起動時のモデルロード副作用など）が特定されました。

本レポートでは、特定された不整合と課題を整理し、具体的な実装レベルでの解決策および改善ロードマップを提案します。

---

## 2. ドキュメントとコードの不一致・不整合 (Document-to-Code Mismatches and Inconsistencies)

ドキュメントと実際のコード実装の間で確認された主要な不整合および設定エラーを以下に示します。

### 2.1 認証introspection結果のキャッシュ有効期限 (Cache TTL Defaults)
- **ファイルパスと該当行**: `server/auth-helper/main.go` 62〜71行目
- **ドキュメントの記載内容**: `docs/ARCHITECTURE.md` 112行目、および `docs/OPERATIONS.md` 21行目、91〜93行目において、デフォルトのキャッシュ有効期間（TTL）は **60秒**（`AUTH_INTROSPECT_CACHE_TTL_SECONDS=60`）と説明されています。
- **実際のコード実装**:
  ```go
  ttlStr := os.Getenv("AUTH_INTROSPECT_CACHE_TTL_SECONDS")
  if ttlStr != "" {
      // ...
  } else {
      introspectCacheTTL = 600 * time.Second
  }
  ```
  環境変数が未設定の場合、デフォルト値として **600秒（10分）** がハードコードされています。
- **運用上の影響**: IdP側でアカウントやトークンが無効化（リボーク）された場合、環境変数を明示的に設定していない環境では、最大10分間にわたり無効なトークンでのアクセスが許可されてしまい、セキュリティ上の脆弱性となります。

### 2.2 Docker ComposeにおけるCaddy環境変数の不足 (Caddy Environment Variables Mismatch)
- **ファイルパスと該当行**: `deploy/compose.yaml` 19〜30行目、`deploy/Caddyfile` 36行目・44行目
- **ドキュメントの記載内容**: `deploy/Caddyfile` では、`/id_token/*` や `/jwt/*` ルートでの認証時に、環境変数 `{$OAUTH_CLIENT_ID}` や `{$AUTH_EXPECTED_AUD}` をクエリパラメータとして認証ヘルパーに渡す設計になっています。
- **実際のコード実装**: `deploy/compose.yaml` 内の `caddy` コンテナサービス定義の `environment` ブロックには、`OTEL_EXPORTER_OTLP_ENDPOINT` のみが定義されており、`OAUTH_CLIENT_ID` や `AUTH_EXPECTED_AUD` が渡されていません。
- **運用上の影響**: Docker Composeでシステムを起動した際、Caddy側でこれらの環境変数が空文字として評価されます。認証ヘルパーの `/auth/jwks` ハンドラーは `expected_aud` クエリパラメータが空である場合に `400 Bad Request` を返してリクエストを拒否するため、デフォルト設定でのJWKSトークン検証フローが動作しません。

### 2.3 No-Authモード（ローカルテスト）時の挙動と必要設定 (No-Auth Mode Mock Configuration)
- **ファイルパスと該当行**: `server/auth-helper/main.go` 326〜333行目、`README.md` 194〜203行目（`README_ja.md` 199〜209行目）
- **ドキュメントの記載内容**: 「No-Authモード：`deploy/.env` の認証用変数を空のままにすると、認証ヘルパーはいかなるBearerトークンも有効とみなす（クライアントブリッジはダミートークンで通信が通る）」と記載されています。
- **実際のコード実装**:
  ```go
  if oauthIntrospectURL == "" {
      if os.Getenv("MOCK_AUTH") == "true" {
          return &IntrospectionResponse{Active: true}, nil
      }
      return &IntrospectionResponse{Active: false}, nil
  }
  ```
  `OAUTH_INTROSPECT_URL` が空の場合でも、`MOCK_AUTH` 環境変数が `"true"` に設定されていない限り、認証ヘルパーはトークンを却下（401エラー）します。
- **運用上の影響**: ユーザーがドキュメント通りに認証変数を空にしただけでは通信が拒否され、No-Authモードが機能しません。ローカル開発やテストの開始時に無用な混乱を招きます。

### 2.4 OpenAPI定義ファイル（openapi.json）のパス相違 (OpenAPI Path Discrepancies)
- **ファイルパスと該当行**: `docs/CHATGPT_CUSTOM_GPTS_SETUP.md` 20行目、`docs/COPILOT_STUDIO_SETUP.md` 12行目、`docs/WEB_APP_INTEGRATION.md` 21行目、および `server/core/src/main.py`
- **ドキュメントの記載内容**: カスタムGPTやCopilotのセットアップガイドにおいて、OpenAPIスキーマのURLとして `https://<your-domain>/api/openapi.json` が案内されています。
- **実際のコード実装**: FastAPIはデフォルトのルートパス `/openapi.json` でスキーマを公開しており、`/api/openapi.json` のようなサブパス設定は行われていません。さらに、Caddyのルーティング設定は `/api/*` を直接許可していません。
- **運用上の影響**: 外部連携の登録時にスキーマ取得URLを指定すると `404 Not Found` となり、外部統合の自動セットアップが失敗します。

### 2.5 Scoop/Homebrewセットアップガイドとリポジトリの実態 (Scoop/Homebrew Setup Guides)
- **ファイルパスと該当行**: `README.md` 155〜159行目、およびリポジトリ全体の構成
- **ドキュメントの記載内容**: Windowsユーザー向けに `scoop install rrag-bridge` コマンドを使用したインストールの手順が示されています。また、macOSユーザー向けに `Formula/rrag-bridge.rb` を用いたHomebrewインストール手順があります。
- **実際のコード実装**: リポジトリ内にはScoopのパッケージ定義であるマニフェストファイル（`bucket/rrag-bridge.json` など）が一切存在しません。また、Homebrewフォーミュラ `Formula/rrag-bridge.rb` にはSHA256ハッシュなどに `REPLACE_ME_*` というプレースホルダーが残ったままになっています。
- **運用上の影響**: ユーザーがドキュメントに従ってScoopでのインストールを試みてもエラーで失敗し、導入の障壁となります。

### 2.6 クイックスタートドキュメントにおけるエンドポイントの誤り (Route Path Errors)
- **ファイルパスと該当行**: `README.md` 180行目、`deploy/Caddyfile` 全般
- **ドキュメントの記載内容**: クイックスタートガイドにて、クライアントの設定例として `"args": ["--url", "https://<your-deployed-domain>/mcp/sse"]` が示されています。
- **実際のコード実装**: `deploy/Caddyfile` では、ルートパス `/mcp/sse` のリクエストを処理・中継するディレクティブが存在せず、`/id_token/*` や `/jwt/*` などの認証用プレフィックスのついたパスのみをルーティングの対象としています。
- **運用上の影響**: クイックスタートに記載された設定のまま実行すると、Caddy側でルーティングできず `404` エラーが発生し、初期接続に失敗します。

### 2.7 ONNXモデルファイル未指定時のフォールバック動作不良 (ONNX Fallback Configs)
- **ファイルパスと該当行**: `server/core/src/database/schema.py` 66〜70行目、`server/core/src/database/reranker.py` 69〜75行目、および `docs/MODELS.md` 65〜71行目
- **ドキュメントの記載内容**: `EMBEDDING_ONNX_FILE=none` または `RERANKER_ONNX_FILE=none` と設定することで、ONNXランタイムを使用せず、PyTorchやCPUのデフォルト（Hugging Face SentenceTransformersの標準動作）へ安全にフォールバックできると記載されています。
- **実際のコード実装**:
  ```python
  # schema.py の実装例
  self._model = SentenceTransformer(
      self.name,
      backend="onnx",
      model_kwargs=kwargs if kwargs else None
  )
  ```
  `EMBEDDING_ONNX_FILE` が `"none"` の場合であっても、SentenceTransformerおよびCrossEncoderの初期化時に `backend="onnx"` が常にハードコードされて引き渡されます。
- **運用上の影響**: ONNXファイルを `"none"` に指定しても、ライブラリ内部でONNXバックエンドでの読み込みが強制実行され、ONNXモデルが見つからないためにエラーで起動がクラッシュします。PyTorchフォールバックが全く機能しません。

---

## 3. コード品質、堅牢性、およびパフォーマンス分析 (Code Quality, Robustness, and Performance Analysis)

システム全体の堅牢性向上と商用利用に耐えうるパフォーマンス確保のために、解決すべきアーキテクチャおよび実装上の問題点を分析します。

### 3.1 クライアントブリッジ起動時の強制認証クラッシュ (Client Bridge Startup Crash)
- **分析**: クライアントブリッジの `client/bridge/main.go` 起動時に、ローカルのOSキーリングに保存されたトークン有無を確認するために `GetValidToken()` が無条件で呼び出されます。トークンが存在しない場合、自動的にOAuth2フローを開始しようとしますが、ローカル環境でNo-Authモード（あるいは静的トークンモード）として動かしたい場合にもOAuthクライアントID等の定義がチェックされるため、未設定だと `OAuth configuration is incomplete...` エラーで起動前にクラッシュします。
- **影響**: ドキュメント記載のNo-Authモードや静的トークンによるシンプルな動作検証がクライアント側で不可能になっており、開発体験を著しく損ねています。

### 3.2 送信処理（Transmitter）のスレッドブロッキング (Transmitter Blocking Thread)
- **分析**: `client/bridge/transmitter.go` において、標準入力からJSON-RPCメッセージ（MCPメッセージ）を受け取り、それをリモートサーバーへHTTP POST送信するループが同一スレッド（ゴルーチン）上で同期的に実行されています。POST送信を担う `t.sendPostRequest()` は同期的なHTTP処理であり、最大30秒のタイムアウト設定となっています。
- **影響**: MCP（Model Context Protocol）は非同期・双方向フルデュプレックス通信を前提とした規格ですが、この実装ではHTTP POSTリプライを待つ間、標準入力スキャンが完全にフリーズします。検索などの重い処理を実行している最中に、AIクライアント側から別の通知（キャンセルや追加要求など）を送信してもブリッジ側で読み込めず、デッドロックや著しいラグを引き起こす原因になります。

### 3.3 401 Unauthorized受信時のトークン未失効化 (Lack of Keyring Invalidation)
- **分析**: リモートサーバーまたはプロキシから認証エラーを示す `401 Unauthorized` レスポンスが返却された場合、`client/bridge/transmitter.go` および `sse.go` は警告を出力するのみで、OSキーリングに格納されているトークンの削除や無効化処理を行いません。
- **影響**: 認証プロバイダ（IdP）側でトークンが明示的に無効化（リボーク）された場合であっても、ローカルに記録された有効期限（Expiry）に達するまでは、クライアントブリッジが無効なトークンを送信し続けます。これにより、手動でキーリングを削除しない限り、再接続も再認証プロンプトの表示も行われない永続的な切断状態（ハング）に陥ります。

### 3.4 SSEの複数行イベントパースの設計不備 (Multi-line SSE Printing Issues)
- **分析**: `client/bridge/sse.go` の `readStream` 内において、SSE（Server-Sent Events）の仕様に基づかない簡略的なパースが行われています。`data: ` プレフィックスを検知した段階で即座に標準出力（`stdout`）へ出力し、改行コードを末尾に付与しています。
- **影響**: 本来のSSE規格では、1つのイベントに対して複数行の `data: ` が返ってくる可能性があり、これらは改行コードで結合した1つのJSONペイロードとして構築されるべきです。現行実装では複数行のJSONデータが途中で分割され、個別に改行されて `stdout` に送られるため、AIクライアント側のJSON-RPCパーサーが不正なフォーマットとしてエラー検知し、通信が切断されます。

### 3.5 Go auth-helperにおけるMemoryCacheクリーンアップ遅延 (Go auth-helper MemoryCache Cleanup)
- **分析**: `server/auth-helper/cache.go` に実装されている `MemoryCache` は、`Get()` メソッド内でデータが期限切れ（Expired）であることを検知し `ErrCacheMiss` を返しますが、その期限切れデータを内部マップ（`m.items`）から削除（エビクション）していません。実際に削除が行われるのは、5分周期で裏で動いている `cleanupLoop` の実行時のみです。
- **影響**: 大量のユーザーや短期トークンが頻繁に行き交う高スループットな環境下において、無効化された大量のキャッシュデータが最大5分間メモリ上に残留し、不要なメモリ圧迫を引き起こします。

### 3.6 Python RAG Coreモジュールインポート時のモデル誤ロード (Core Python Model Loading Side-effects)
- **分析**: `server/core/scripts/count_chars.py` や各種ユーティリティ、FastAPIの初期設定などにおいて、データベース接続用の `DB_PATH` をインポートしようと `src.database.client` をインポートすると、芋づる式に `src.database.schema` のモジュールロードが走ります。その際、テーブル定義クラス `WikiChunk` の初期化プロセス内で `embed_func.ndims()` が呼び出されます。この関数は、環境変数 `VECTOR_DIM` が定義されていない場合、正しい次元数を取得するために実際に `SentenceTransformer` モデルを初期化しテストデータをエンコードする処理（`generate_embeddings(["test"])`）を実行します。
- **影響**: 単なる文字カウントやマイグレーション、ヘルスチェックなどの極めて軽量なスクリプトを実行する場合でも、裏で数GBにおよぶAIモデルの読み込みが発生し、起動までに5〜10秒以上の遅延が生じるほか、余分なメモリ消費が発生します。これによりFastAPIサーバー自体のコールドスタート時のヘルスチェックタイムアウト（FastAPI起動ラグ）のリスクも高まっています。

### 3.7 テストポータビリティの欠如 (Test Portability Issues)
- **分析**: `server/core/tests/test_mcp_client.py` の10行目において、テスト用Pythonインタープリターのパスが Windows 環境専用の `os.path.join(..., ".venv", "Scripts", "python.exe")` に固定されています。
- **影響**: Linux や macOS といった POSIX 互換システム（CI/CD環境含む）では、仮想環境のPythonバイナリは `.venv/bin/python` に配置されるため、当該テストがスキップされるか、存在しないパスの実行を試みてエラーとなります。

---

## 4. 未ドキュメントの環境変数一覧 (Undocumented Environment Variables)

コードベース内で利用されているものの、公式の運用ガイド（`docs/OPERATIONS.md` 等）に仕様やデフォルト値が記載されていない環境変数は以下の通りです。

| コンポーネント | 環境変数名 | デフォルト値 | 用途 / コード参照箇所 |
| :--- | :--- | :--- | :--- |
| **client** | `RRAG_PROFILE` | `""` | クライアントブリッジで複数アカウントや接続先プロファイルを切り替える際の、キーリングのサービス名/アカウントの接尾辞として使用されます。 (`auth.go` 29行目) |
| **server (Go)** | `MOCK_AUTH` | `""` | 認証プロキシでローカルテストを実行する際、`true` に設定することで外部IdPへのトークン検証（Introspect）をスキップし、全リクエストを通します。 (`main.go` 327行目) |
| **server (Python)** | `ENABLE_INGEST_API` | `"false"` | `"true"` に設定すると、データ取り込み用のインジェストルーター（`/ingest`）を有効にし、Webhook経由でドキュメントを取り込めるようにします。 (`main.py` 39行目) |
| **server (Python)** | `LANCEDB_PATH` | `"server/core/data/lancedb"` | LanceDBのデータベース保存先ディレクトリを指定します。 (`client.py` 12行目) |
| **server (Python)** | `VECTOR_DIM` | `None` | ベクトル埋め込み次元数を明示的に指定します。設定すると、モジュールインポート時のモデル事前読み込みを抑止できます。 (`schema.py` 32行目) |
| **server (Python)** | `WIKI_SEARCH_MAX_TOKENS` | `4000` | RAG検索結果からLLMに渡すコンテキストの最大トークン数を制限し、API利用料金を抑制します。 (`searcher.py` 32行目) |
| **server (Python)** | `DEFAULT_RELEVANCE_THRESHOLD` | `-1.0` | 検索で取得したチャンクをRerankした後、有効とみなす最低リランカースコアの基準値。 (`searcher.py` 10行目) |
| **server (Python)** | `EXACT_MATCH_RELEVANCE_THRESHOLD` | `-5.0` | クエリ内容がデータベース内の完全一致テキストと判定された場合の一時的な閾値緩和幅。 (`searcher.py` 12行目) |

---

## 5. 改善提案とロードマップ (Actionable Improvement Proposals)

特定された課題への具体的かつ動作可能な修正アプローチを示します。

### 5.1 Docker ComposeおよびCaddy設定の修正
`deploy/compose.yaml` の `caddy` サービス定義を修正し、Caddyfileが必要とする環境変数を正しく引き渡します。
```yaml
  caddy:
    image: caddy:2-alpine
    container_name: caddy-proxy
    ports:
      - "8080:8080"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
    environment:
      - OTEL_EXPORTER_OTLP_ENDPOINT=${OTEL_EXPORTER_OTLP_ENDPOINT:-}
      - OAUTH_CLIENT_ID=${OAUTH_CLIENT_ID:-}
      - AUTH_EXPECTED_AUD=${AUTH_EXPECTED_AUD:-}
    depends_on:
      - auth-helper
      - rrag-server
```

### 5.2 クライアントブリッジにおけるNo-Authモード（バイパス用CLIフラグ）の追加
OAuth情報を省略して起動し、ダミートークン通信を行えるよう、`client/bridge/main.go` に `--no-auth` フラグを追加し、`GetValidToken()` をスキップできる経路を用意します。
```go
// client/bridge/main.go の修正イメージ
var noAuth = flag.Bool("no-auth", false, "Bypass local keyring and OAuth checking")

func main() {
    flag.Parse()
    config := ParseConfig()

    var token TokenData
    var err error
    if !*noAuth {
        token, err = GetValidToken()
        if err != nil {
            fmt.Fprintf(os.Stderr, "Failed to get initial auth token: %v\n", err)
            os.Exit(1)
        }
    } else {
        fmt.Fprintf(os.Stderr, "Running in No-Auth mode bypass.\n")
    }
    // sseClient 起動時にも no-auth フラグの状態を引き渡す
}
```

### 5.3 送信処理（Transmitter）の非同期化（マルチスレッド送信）
標準入力の読み取りループをブロックしないよう、POSTリクエスト送信処理を別のゴルーチンで非同期に実行します。
```go
// client/bridge/transmitter.go の修正イメージ
for {
    select {
    case msg, ok := <-t.StdinChan:
        if !ok {
            return
        }
        if strings.TrimSpace(msg) == "" {
            continue
        }
        
        // 排他制御付きで最新の postURL を安全に取得するヘルパー
        u := t.getSafePostURL()
        if u == "" {
            continue
        }
        
        // ゴルーチンを起動し、非同期でリクエストを投げる
        go t.sendPostRequest(u, msg)
    }
}
```

### 5.4 401 Unauthorizedレスポンス受信時のローカルトークン削除
認証期限切れやアカウント無効化に対処するため、401エラーを受信した時点でキーリングのトークンを自動で破棄する機構を追加します。
```go
// client/bridge/auth.go に追加する関数
func InvalidateToken() error {
    return keyring.Delete(serviceName, getAccountName())
}

// client/bridge/transmitter.go のレスポンス評価時
if resp.StatusCode == http.StatusUnauthorized {
    fmt.Fprintf(os.Stderr, "Received 401 Unauthorized. Invalidating local cached credentials...\n")
    InvalidateToken()
}
```

### 5.5 SSEにおける複数行イベントのバッファリング対応
`readStream` 内において、空行（`\n`）のセパレーターを受信するまでデータをバッファリングするようパース処理を改善します。
```go
// client/bridge/sse.go の読み込み部修正
var dataBuffer bytes.Buffer
for {
    line, err := reader.ReadBytes('\n')
    if err != nil {
        return err
    }
    line = bytes.TrimSuffix(line, []byte("\n"))
    line = bytes.TrimSuffix(line, []byte("\r"))

    if bytes.HasPrefix(line, []byte("event: ")) {
        currentEvent = string(bytes.TrimPrefix(line, []byte("event: ")))
    } else if bytes.HasPrefix(line, []byte("data: ")) {
        data := bytes.TrimPrefix(line, []byte("data: "))
        if dataBuffer.Len() > 0 {
            dataBuffer.WriteByte('\n')
        }
        dataBuffer.Write(data)
    } else if len(line) == 0 {
        // 空行（イベントの終了）を受信した時点でまとめて出力する
        if dataBuffer.Len() > 0 {
            if currentEvent == "endpoint" {
                c.handleEndpointEvent(dataBuffer.String())
            } else {
                stdout.Write(dataBuffer.Bytes())
                stdout.Write([]byte("\n"))
            }
            dataBuffer.Reset()
        }
        currentEvent = ""
    }
}
```

### 5.6 認証ヘルパーにおけるExpiredキャッシュの即時削除
`MemoryCache` から値を取得した際、無効（期限切れ）と判定されたデータを即座にマップから削除してメモリリークを防ぎます。
```go
// server/auth-helper/cache.go の Get メソッド内
func (m *MemoryCache) Get(key string) (string, error) {
    m.mu.RLock()
    item, exists := m.items[key]
    m.mu.RUnlock()

    if !exists {
        return "", ErrCacheMiss
    }

    if time.Now().After(item.expiration) {
        // 期限切れデータを即時削除
        m.mu.Lock()
        delete(m.items, key)
        m.mu.Unlock()
        return "", ErrCacheMiss
    }

    return item.value, nil
}
```

### 5.7 Python RAG Coreのモデル遅延読み込みと次元数固定
スキーマ定義ロード時のモデル起動を避けるため、`VECTOR_DIM` 環境変数の使用を標準化するか、エンコーダーモデルの初期化を必要とされる瞬間まで遅延させます。
```python
# server/core/src/database/schema.py の修正
class WikiChunk(LanceModel):
    # VECTOR_DIM 環境変数を優先し、設定されていなければモデルロードを行う遅延評価にする
    vector: Vector(int(os.getenv("VECTOR_DIM", 384))) 
    # ...
```

### 5.8 ONNXフォールバック設定時のバックエンド条件分岐の追加
`EMBEDDING_ONNX_FILE` または `RERANKER_ONNX_FILE` に `"none"` が指定されている場合、`backend="onnx"` パラメータを取り除き、PyTorchで読み込むように分岐処理を導入します。
```python
# server/core/src/database/schema.py
onnx_disabled = not EMBEDDING_ONNX_FILE or str(EMBEDDING_ONNX_FILE).lower() == "none"

self._model = SentenceTransformer(
    self.name,
    backend=None if onnx_disabled else "onnx",
    model_kwargs=kwargs if kwargs and not onnx_disabled else None
)
```

### 5.9 テストコードにおけるマルチプラットフォーム対応のPythonパス解決
Windows以外のOSでもテストを正しく走らせるため、プラットフォームに応じてパスを切り分けるロジックを追加します。
```python
# server/core/tests/test_mcp_client.py
bin_dir = "Scripts" if os.name == "nt" else "bin"
exe_name = "python.exe" if os.name == "nt" else "python"
venv_python = os.path.join(
    os.path.dirname(os.path.dirname(__file__)), 
    ".venv", 
    bin_dir, 
    exe_name
)
```
