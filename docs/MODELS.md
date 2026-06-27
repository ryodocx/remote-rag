# AI Models Configuration Guide

RRAG MCP Server では、テキストのベクトル化（Embedding）と検索結果の再評価（Reranker）に利用するAIモデルを、環境変数を通じて柔軟に切り替えることができます。

## 設定方法
`compose.yaml` 内の `environment` セクション、あるいは環境変数に以下の設定値を追加・上書きすることでモデルを変更できます。

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

## ユースケース別 推奨構成

要件に合わせた推奨構成の比較表です。用途に合う構成を選択し、`.env` または `compose.yaml` に設定してください。

| 構成名 | 検索品質 | 処理速度 | 予想メモリ | 特徴 | `EMBEDDING_MODEL` | `RERANKER_MODEL` |
| :--- | :---: | :---: | :--- | :--- | :--- | :--- |
| **1. 現代の標準** (デフォルト) | ⭐⭐⭐⭐ | ⚡⚡⚡⚡ | 約 1.4 GB | わずかなメモリでIT技術や日本語の文脈を高度に理解するバランス構成。 | `intfloat/multilingual-e5-base` | `cross-encoder/mmarco-mMiniLMv2-L12-H384-v1` |
| **2. 超軽量化** | ⭐⭐⭐ | ⚡⚡⚡⚡⚡ | 約 0.8 GB | 極限まで軽く常駐に最適。ただし複雑な文脈の一致度はやや低下。 | `intfloat/multilingual-e5-small` | `none` (無効化) |
| **3. 最高峰精度** | ⭐⭐⭐⭐⭐ | ⚡⚡ | 約 4.0 GB+ | オープンソース最強クラスの精度。複雑な質問にも高精度で答えるが非常に重い。 | `BAAI/bge-m3` | `BAAI/bge-reranker-v2-m3` |
| **4. 日本語特化** | ⭐⭐⭐⭐ | ⚡⚡⚡ | 約 1.8 GB | 社内文書が完全に日本語のみの場合に最適化されたモデル。プレフィックス不要。 | `pkshatech/GLuCoSE-base-ja` | `cross-encoder/mmarco-mMiniLMv2-L12-H384-v1` |
| **5. プレフィックス不要・多言語** | ⭐⭐⭐⭐ | ⚡⚡⚡⚡ | 約 1.4 GB | e5系の `query:` 指定が面倒な場合や、一般的な文章検索に適した安定板。 | `sentence-transformers/paraphrase-multilingual-mpnet-base-v2` | `cross-encoder/mmarco-mMiniLMv2-L12-H384-v1` |

### 設定スニペット (.env)

<details>
<summary>1. 現代の標準 (コスパ重視・デフォルト) の設定例</summary>

```dotenv
# デフォルト値のため、設定を省略した場合もこの挙動になります。
EMBEDDING_MODEL=intfloat/multilingual-e5-base
EMBEDDING_ONNX_FILE=onnx/model_qint8_avx512_vnni.onnx
EMBEDDING_PREFIX_QUERY="query: "
EMBEDDING_PREFIX_PASSAGE="passage: "
RERANKER_MODEL=cross-encoder/mmarco-mMiniLMv2-L12-H384-v1
RERANKER_ONNX_FILE=onnx/model_quint8_avx2.onnx
```
</details>

<details>
<summary>2. 超軽量化 (速度・省メモリ重視) の設定例</summary>

```dotenv
EMBEDDING_MODEL=intfloat/multilingual-e5-small
EMBEDDING_ONNX_FILE=onnx/model_qint8_avx512_vnni.onnx
EMBEDDING_PREFIX_QUERY="query: "
EMBEDDING_PREFIX_PASSAGE="passage: "
RERANKER_MODEL=none  # Rerankerを無効化
```
</details>

<details>
<summary>3. 最高峰精度 (クオリティ重視・ハイエンド) の設定例</summary>

```dotenv
EMBEDDING_MODEL=BAAI/bge-m3
EMBEDDING_ONNX_FILE=none  # PyTorch等デフォルト設定にフォールバック
EMBEDDING_PREFIX_QUERY=""
EMBEDDING_PREFIX_PASSAGE=""
RERANKER_MODEL=BAAI/bge-reranker-v2-m3
RERANKER_ONNX_FILE=none
```
</details>

<details>
<summary>4. 日本語特化 (日本語文書のみを扱う場合) の設定例</summary>

```dotenv
EMBEDDING_MODEL=pkshatech/GLuCoSE-base-ja
EMBEDDING_ONNX_FILE=none  # HuggingFaceのPyTorch形式を利用
EMBEDDING_PREFIX_QUERY=""
EMBEDDING_PREFIX_PASSAGE=""
RERANKER_MODEL=cross-encoder/mmarco-mMiniLMv2-L12-H384-v1
RERANKER_ONNX_FILE=onnx/model_quint8_avx2.onnx
```
</details>

<details>
<summary>5. プレフィックス不要・多言語 (旧標準の安定板) の設定例</summary>

```dotenv
EMBEDDING_MODEL=sentence-transformers/paraphrase-multilingual-mpnet-base-v2
EMBEDDING_ONNX_FILE=onnx/model.onnx  # ONNXに対応
EMBEDDING_PREFIX_QUERY=""
EMBEDDING_PREFIX_PASSAGE=""
RERANKER_MODEL=cross-encoder/mmarco-mMiniLMv2-L12-H384-v1
RERANKER_ONNX_FILE=onnx/model_quint8_avx2.onnx
```
</details>

> [!WARNING]
> **DBの再構築（データの全消去）について**
>
> 環境変数で `EMBEDDING_MODEL` を切り替えた場合、これまでに作成したベクトルの次元数や意味合いが変わるため、以前のデータベースは利用できなくなります。
> モデルを差し替える際は、常にホスト側の `data/lancedb` フォルダの中身を削除し、再度データの取り込み（Ingest）を実行し直してください。
