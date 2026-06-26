# AI Models Configuration Guide

Remote RAG MCP Server では、テキストのベクトル化（Embedding）と検索結果の再評価（Reranker）に利用するAIモデルを、環境変数を通じて柔軟に切り替えることができます。

## 設定方法
`docker-compose.yml` 内の `environment` セクション、あるいは環境変数に以下の設定値を追加・上書きすることでモデルを変更できます。

### 環境変数一覧

| 変数名 | 説明 | デフォルト値 |
| :--- | :--- | :--- |
| `EMBEDDING_MODEL` | HuggingFaceのEmbeddingモデルのID | `intfloat/multilingual-e5-base` |
| `EMBEDDING_ONNX_FILE` | ONNXランタイムで使用するファイル名。指定しない場合は自動フォールバック | `onnx/model_qint8_avx512_vnni.onnx` |
| `EMBEDDING_PREFIX_QUERY` | クエリ時に付与する文字列プレフィックス (e5系で必須) | `query: ` |
| `EMBEDDING_PREFIX_PASSAGE` | DB挿入時に付与する文字列プレフィックス (e5系で必須) | `passage: ` |
| `RERANKER_MODEL` | HuggingFaceのReranker(CrossEncoder)のID。`none` または空文字で無効化 | `cross-encoder/mmarco-mMiniLMv2-L12-H384-v1` |
| `RERANKER_ONNX_FILE` | RerankerのONNXファイル名 | `onnx/model_quint8_avx2.onnx` |

---

## ユースケース別 推奨構成 (モデル設定スニペット)

以下は、要件に合わせた `.env` または `docker-compose.yml` への設定例です。

### 1. 現代の標準 (コスパ重視・デフォルト)
*   **特徴**: わずか1.4GB程度のメモリで、IT技術や日本語の文脈を高度に理解するバランス構成。
*   **予想メモリ**: 約 1.4 GB

```dotenv
# デフォルト値のため、設定を省略した場合もこの挙動になります。
EMBEDDING_MODEL=intfloat/multilingual-e5-base
EMBEDDING_ONNX_FILE=onnx/model_qint8_avx512_vnni.onnx
EMBEDDING_PREFIX_QUERY="query: "
EMBEDDING_PREFIX_PASSAGE="passage: "
RERANKER_MODEL=cross-encoder/mmarco-mMiniLMv2-L12-H384-v1
RERANKER_ONNX_FILE=onnx/model_quint8_avx2.onnx
```

### 2. 超軽量化 (速度・省メモリ重視)
*   **特徴**: Rerankerを無効化し、Embeddingもより小さなモデルに変更。メモリ1GB未満で常駐可能ですが、精細な文脈の一致度はやや低下します。
*   **予想メモリ**: 約 0.8 GB

```dotenv
EMBEDDING_MODEL=intfloat/multilingual-e5-small
EMBEDDING_ONNX_FILE=onnx/model_qint8_avx512_vnni.onnx
EMBEDDING_PREFIX_QUERY="query: "
EMBEDDING_PREFIX_PASSAGE="passage: "
RERANKER_MODEL=none  # Rerankerを無効化
```

### 3. 最高峰精度 (クオリティ重視・ハイエンド)
*   **特徴**: オープンソース最強クラスのモデル。複雑な質問にも高精度で答えますが、メモリとCPUを非常に多く消費します。
*   **予想メモリ**: 約 4.0 GB 以上

```dotenv
EMBEDDING_MODEL=BAAI/bge-m3
EMBEDDING_ONNX_FILE=none  # PyTorch等デフォルト設定にフォールバック
EMBEDDING_PREFIX_QUERY=""
EMBEDDING_PREFIX_PASSAGE=""
RERANKER_MODEL=BAAI/bge-reranker-v2-m3
RERANKER_ONNX_FILE=none
```

> [!WARNING]
> **DBの再構築（データの全消去）について**
> 
> 環境変数で `EMBEDDING_MODEL` を切り替えた場合、これまでに作成したベクトルの次元数や意味合いが変わるため、以前のデータベースは利用できなくなります。
> モデルを差し替える際は、常にホスト側の `data/lancedb` フォルダの中身を削除し、再度データの取り込み（Ingest）を実行し直してください。
