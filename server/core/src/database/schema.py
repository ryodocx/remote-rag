from lancedb.pydantic import LanceModel, Vector
from lancedb.embeddings import EmbeddingFunctionRegistry, TextEmbeddingFunction
from pydantic import PrivateAttr

model_name = "paraphrase-multilingual-MiniLM-L12-v2"
registry = EmbeddingFunctionRegistry.get_instance()

@registry.register("quantized-sentence-transformers")
class QuantizedSentenceTransformerEmbeddings(TextEmbeddingFunction):
    name: str = model_name
    _model: any = PrivateAttr(default=None)
    _ndims: int = PrivateAttr(default=None)
    
    def ndims(self):
        if self._ndims is None:
            self._ndims = len(self.generate_embeddings(["test"])[0])
        return self._ndims

    def generate_embeddings(self, texts):
        if self._model is None:
            import logging
            from sentence_transformers import SentenceTransformer
            logging.getLogger(__name__).info("Loading Quantized SentenceTransformer with ONNX...")
            # 施策1 & 3: ONNXランタイムと量子化済みモデル(INT8)を組み合わせて極限までメモリ削減
            self._model = SentenceTransformer(
                self.name,
                backend="onnx",
                model_kwargs={"file_name": "onnx/model_quint8_avx2.onnx"}
            )
        
        # 戻り値をリスト形式に変換 (LanceDB用)
        return self._model.encode(texts).tolist()

embed_func = registry.get("quantized-sentence-transformers").create(name=model_name)

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
