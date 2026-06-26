import os
import logging
import lancedb
from src.database.schema import WikiChunk

logger = logging.getLogger(__name__)

# デフォルトのデータベースパス（環境変数 LANCEDB_PATH で上書き可能）
_DEFAULT_DB_PATH = os.path.join(
    os.path.dirname(os.path.dirname(os.path.dirname(__file__))), "data", "lancedb"
)
DB_PATH = os.environ.get("LANCEDB_PATH", _DEFAULT_DB_PATH)


class DatabaseClient:
    """
    LanceDBを操作し、Wikipedia記事のチャンクデータに対するベクトル検索およびフルテキスト検索（FTS）を提供するクライアント。
    """
    def __init__(self, db_path: str = DB_PATH, table_name: str = "wiki_chunks"):
        """
        DatabaseClientの初期化。
        
        Args:
            db_path (str): LanceDBのデータ保存先ディレクトリパス。
            table_name (str): 操作対象のテーブル名（デフォルト: 'wiki_chunks'）。
        """
        os.makedirs(os.path.dirname(db_path), exist_ok=True)
        self.db = lancedb.connect(db_path)
        self.table_name = table_name
        self.table = self._get_or_create_table()
        self._reranker_instance = None

    @property
    def reranker(self):
        """
        CrossEncoderによるRerankerモデルを遅延読み込み（Lazy Load）で取得します。
        メモリ消費を抑えつつ、必要なタイミングで1度だけロードします。
        
        Returns:
            CrossEncoderReranker: 検索結果再評価用のモデルインスタンス。
        """
        if self._reranker_instance is None:
            from src.database.reranker import OnnxCrossEncoderReranker
            
            reranker_model = os.environ.get("RERANKER_MODEL", "cross-encoder/mmarco-mMiniLMv2-L12-H384-v1")
            reranker_onnx = os.environ.get("RERANKER_ONNX_FILE", "onnx/model_quint8_avx2.onnx")
            
            # 空文字や 'none' が指定された場合はRerankerを無効化
            if not reranker_model or reranker_model.lower() == "none":
                return None
                
            self._reranker_instance = OnnxCrossEncoderReranker(
                model_name=reranker_model,
                column="text",
                onnx_file_name=reranker_onnx if (reranker_onnx and reranker_onnx.lower() != "none") else None
            )

                
        return self._reranker_instance

    def _get_or_create_table(self):
        """
        指定されたテーブル名が存在する場合は開き、存在しない場合はスキーマに従って新規作成します。
        
        Returns:
            lancedb.table.Table: LanceDBのテーブルオブジェクト。
        """
        if self.table_name in self.db.table_names():
            return self.db.open_table(self.table_name)
        else:
            return self.db.create_table(self.table_name, schema=WikiChunk)

    def add_chunks(self, chunks: list[dict]):
        """
        データベースに複数のチャンクデータを一括追加します。
        
        Args:
            chunks (list[dict]): WikiChunkスキーマに準拠した辞書のリスト。
        """
        if not chunks:
            return
        
        self.table.add(chunks)
        logger.info(f"Successfully added {len(chunks)} chunks to table '{self.table_name}'.")

    def delete_chunks_by_page(self, page_id: str):
        """
        特定のページIDに紐づくチャンクを全て削除します。
        新規記事の挿入前に行うことで、古いデータの重複を防ぎます。
        
        Args:
            page_id (str): 削除対象となるWikipediaのページID。
        """
        try:
            # SQLインジェクション対策: シングルクォートをエスケープ
            safe_page_id = page_id.replace("'", "''")
            self.table.delete(f"page_id = '{safe_page_id}'")
            logger.debug(f"Deleted old chunks for page_id: {page_id}")
        except Exception as e:
            logger.warning(f"Failed to delete chunks for page {page_id}. It might be a new page. Error: {e}")

    def create_fts_index(self):
        """
        キーワード検索（フルテキスト検索）用のインデックスを再構築します。
        データの追加・削除の後に実行することで検索品質を維持します。
        """
        logger.info("Creating/Replacing FTS index...")
        self.table.create_fts_index("text", replace=True)
        logger.info("FTS index creation completed.")

    def optimize(self):
        """
        データベースのフラグメンテーションを解消し（コンパクション）、FTSインデックスを再構築します。
        データの追加・削除を繰り返した後に発生しうるTantivy（Rustコア）の不整合エラーを防ぎます。
        定期的な実行が推奨されます。
        """
        logger.info("Starting database optimization (compaction)...")
        self.table.optimize()
        logger.info("Optimization completed. Rebuilding FTS index...")
        self.create_fts_index()
        logger.info("All optimization tasks completed successfully.")

    def search(self, query: str, limit: int = 5, search_type: str = "hybrid"):
        """
        与えられたクエリに対して、ベクトル検索、FTS検索、またはハイブリッド検索を実行します。
        ハイブリッド検索が内部エラーで失敗した場合は、自動的にベクトル検索へフォールバックします。
        
        Args:
            query (str): 検索する文字列。
            limit (int): 取得する検索結果の最大件数（デフォルト: 5）。
            search_type (str): 検索手法（'hybrid', 'fts', 'vector' のいずれか）。
            
        Returns:
            list[dict]: 検索結果の辞書リスト。
            
        Raises:
            Exception: フォールバックも含め全ての検索手法が失敗した場合。
        """
        try:
            if search_type == "hybrid":
                # チャンク生成時のテキストに対してハイブリッド検索を行い、Rerankerで関連性を再計算
                q = self.table.search(query, query_type="hybrid")
                if self.reranker:
                    q = q.rerank(reranker=self.reranker)
                results = q.limit(limit).to_list()
            elif search_type == "fts":
                results = self.table.search(query, query_type="fts").limit(limit).to_list()
            else:
                # search_type == "vector"
                q = self.table.search(query, query_type="vector")
                if self.reranker:
                    q = q.rerank(reranker=self.reranker)
                results = q.limit(limit).to_list()
        except (OSError, ConnectionError, PermissionError) as e:
            # 致命的なI/Oエラーはフォールバックせず呼び出し側に伝播
            logger.error(f"Fatal I/O error during search: {e}")
            raise
        except Exception as e:
            # FTSインデックスが無い、またはLanceDB特有のArrow不整合エラー等で失敗時はベクトル検索にフォールバック
            logger.warning(f"Hybrid search failed, falling back to vector search. Error: {e}")
            try:
                q = self.table.search(query, query_type="vector")
                if self.reranker:
                    q = q.rerank(reranker=self.reranker)
                results = q.limit(limit).to_list()
            except Exception as inner_e:
                logger.error(f"Vector search fallback with reranker also failed: {inner_e}. Falling back to pure vector search.")
                results = self.table.search(query, query_type="vector").limit(limit).to_list()
        return results
