import os
import lancedb
from src.database.schema import WikiChunk

# デフォルトのデータベースパス
DB_PATH = os.path.join(os.path.dirname(os.path.dirname(os.path.dirname(__file__))), "data", "lancedb")

class DatabaseClient:
    def __init__(self, db_path: str = DB_PATH, table_name: str = "wiki_chunks"):
        os.makedirs(os.path.dirname(db_path), exist_ok=True)
        self.db = lancedb.connect(db_path)
        self.table_name = table_name
        self.table = self._get_or_create_table()

    def _get_or_create_table(self):
        if self.table_name in self.db.table_names():
            return self.db.open_table(self.table_name)
        else:
            return self.db.create_table(self.table_name, schema=WikiChunk)

    def add_chunks(self, chunks: list[dict]):
        if not chunks:
            return
        
        self.table.add(chunks)

    def delete_chunks_by_page(self, page_id: str):
        """特定のページIDに紐づく古いチャンクを削除する"""
        try:
            self.table.delete(f"page_id = '{page_id}'")
        except Exception as e:
            print(f"Warning: Failed to delete chunks for page {page_id}. It might be a new page. Error: {e}")

    def create_fts_index(self):
        """ハイブリッド検索用の全文検索インデックスを作成（更新後に実行推奨）"""
        self.table.create_fts_index("text", replace=True)

    def search(self, query: str, limit: int = 5):
        """ベクトル検索を実行する。FTSインデックスがあればハイブリッド検索も可能"""
        try:
            from lancedb.rerankers import CrossEncoderReranker
            # 軽量なクロスエンコーダーモデルを指定（多言語対応が必要な場合は適宜変更）
            reranker = CrossEncoderReranker(model_name="cross-encoder/ms-marco-MiniLM-L-6-v2")
            
            # チャンク生成時のテキストに対してハイブリッド検索を行い、Rerankerで関連性を再計算
            results = self.table.search(query, query_type="hybrid").rerank(reranker=reranker).limit(limit).to_list()
        except Exception as e:
            # FTSインデックスが無い、またはRerankerモデルのロード失敗時等はベクトル検索にフォールバック
            print(f"Hybrid search failed, falling back to vector search. Error: {e}")
            results = self.table.search(query).limit(limit).to_list()
        return results
