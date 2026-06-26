from lancedb.pydantic import LanceModel, Vector
from lancedb.embeddings import get_registry

# 多言語対応の sentence-transformers モデルをロード (日本語に強いモデルを一旦使用)
model_name = "paraphrase-multilingual-MiniLM-L12-v2"
embed_func = get_registry().get("sentence-transformers").create(name=model_name)

class WikiChunk(LanceModel):
    """
    LanceDB用のスキーマ定義。
    text フィールドを SourceField として指定することで、Insert時に自動でベクトル化が行われます。
    """
    chunk_id: str
    page_id: str
    text: str = embed_func.SourceField()
    vector: Vector(embed_func.ndims()) = embed_func.VectorField() # type: ignore
    title: str
    url: str
