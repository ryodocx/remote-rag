import os
from lancedb.pydantic import LanceModel, Vector
from lancedb.embeddings import EmbeddingFunctionRegistry, TextEmbeddingFunction
from pydantic import PrivateAttr

# 環境変数からモデル設定を取得（デフォルトは multilingual-e5-base の INT8版）
EMBEDDING_MODEL = os.environ.get("EMBEDDING_MODEL", "intfloat/multilingual-e5-base")
EMBEDDING_ONNX_FILE = os.environ.get("EMBEDDING_ONNX_FILE", "onnx/model_qint8_avx512_vnni.onnx")

# e5モデルなどで要求されるPrefix（未指定の場合はデフォルトを使用）
EMBEDDING_PREFIX_QUERY = os.environ.get("EMBEDDING_PREFIX_QUERY", "query: ")
EMBEDDING_PREFIX_PASSAGE = os.environ.get("EMBEDDING_PREFIX_PASSAGE", "passage: ")

registry = EmbeddingFunctionRegistry.get_instance()

@registry.register("quantized-sentence-transformers")
class QuantizedSentenceTransformerEmbeddings(TextEmbeddingFunction):
    name: str = EMBEDDING_MODEL
    _model: any = PrivateAttr(default=None)
    _ndims: int = PrivateAttr(default=None)
    
    def ndims(self):
        if self._ndims is None:
            env_dim = os.environ.get("VECTOR_DIM", "768")
            if env_dim:
                self._ndims = int(env_dim)
            else:
                self._ndims = len(self.generate_embeddings(["test"])[0])
        return self._ndims

    def compute_source_embeddings(self, texts: list[str], *args, **kwargs):
        """DBインサート時のベクトル化処理。必要に応じてPrefix(passage:)を付与します。"""
        if EMBEDDING_PREFIX_PASSAGE:
            texts = [EMBEDDING_PREFIX_PASSAGE + str(t.as_py() if hasattr(t, "as_py") else t) for t in texts]
        return self.generate_embeddings(texts)

    def compute_query_embeddings(self, query: str, *args, **kwargs):
        """検索時のベクトル化処理。必要に応じてPrefix(query:)を付与します。"""
        if EMBEDDING_PREFIX_QUERY:
            query = EMBEDDING_PREFIX_QUERY + query
        return self.generate_embeddings([query])

    def generate_embeddings(self, texts):
        """SentenceTransformerによる実際の埋め込み生成処理"""
        if self._model is None:
            import logging
            from sentence_transformers import SentenceTransformer
            logging.getLogger(__name__).info(f"Loading Quantized SentenceTransformer ({self.name}) with ONNX...")
            
            kwargs = {}
            if EMBEDDING_ONNX_FILE and str(EMBEDDING_ONNX_FILE).lower() != "none":
                kwargs["file_name"] = EMBEDDING_ONNX_FILE
                
            self._model = SentenceTransformer(
                self.name,
                backend="onnx",
                model_kwargs=kwargs if kwargs else None
            )
        
        # 戻り値をリスト形式に変換 (LanceDB用)
        return self._model.encode(texts).tolist()

embed_func = registry.get("quantized-sentence-transformers").create(name=EMBEDDING_MODEL)

class WikiChunk(LanceModel):
    """
    LanceDB用のスキーマ定義。
    Wikipediaの記事データを分割した「チャンク（文章の塊）」ごとに保存します。
    text フィールドを SourceField として指定することで、Insert時に自動でベクトル化が行われます。
    
    NOTE: このモジュールをimportすると、embed_func の初期化（および ndims() でのダミー推論）が
    走るため、テスト等で不要な場合は注意が必要です。これは LanceDB の Vector フィールド定義に
    次元数が必要というスキーマ設計上の制約です。
    """
    chunk_id: str
    page_id: str
    text: str = embed_func.SourceField()
    vector: Vector(embed_func.ndims()) = embed_func.VectorField() # type: ignore
    title: str
    url: str
    metadata: str = "{}" # JSON文字列形式のメタデータ（検索やフィルタ用）
