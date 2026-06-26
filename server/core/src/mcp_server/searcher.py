import logging
from src.database.client import DatabaseClient
from src.utils.token_counter import count_tokens

logger = logging.getLogger(__name__)

# スコアフィルタリングの閾値定数
# Reranker の relevance_score に対する閾値。これ以下のスコアのチャンクは除外される。
DEFAULT_RELEVANCE_THRESHOLD = -1.0
# クエリがチャンク本文に完全一致する場合の緩和閾値（マニアックな用語の救済用）
EXACT_MATCH_RELEVANCE_THRESHOLD = -5.0


class WikiSearcher:
    """
    Wikiチャンクデータベースから適切な文書を検索し、LLMに渡すコンテキストとして
    適切なトークン数上限に収まるようフィルタリングするクラス。
    """
    def __init__(self, max_tokens: int = 4000):
        """
        WikiSearcherの初期化。
        
        Args:
            max_tokens (int): LLMに渡す文脈の最大許容トークン数（デフォルト: 4000）。
        """
        self.db_client = DatabaseClient()
        self.max_tokens = max_tokens
        logger.debug(f"WikiSearcher initialized with max_tokens={max_tokens}")

    def search(self, query: str, limit: int = 5, search_type: str = "hybrid", where: str = None) -> list[dict]:
        """
        クエリを用いて検索を実行し、ノイズとなる低スコアのチャンクを除外した上で、
        トークン数上限に収まるように結果をフィルタリングして返します。
        
        Args:
            query (str): 検索する文字列。
            limit (int): 取得を試みる初期の最大検索結果件数（デフォルト: 5）。
            search_type (str): 検索手法（デフォルト: 'hybrid'）。
            where (str): SQL WHERE句による絞り込み。
            
        Returns:
            list[dict]: フィルタリングされた、LLMへ渡すのに適した検索結果のリスト。
        """
        logger.info(f"Executing search for query: '{query}' (type: {search_type}, limit: {limit}, where: {where})")
        # DBから検索結果を取得（ハイブリッド検索またはベクトル検索）
        raw_results = self.db_client.search(query, limit=limit, search_type=search_type, where=where)
        
        filtered_results = []
        current_tokens = 0
        
        for result in raw_results:
            relevance_score = result.get("_relevance_score")
            score = result.get("_score")
            text = result.get("text", "")
            
            # クエリが本文に完全一致（大文字小文字無視）で含まれているかチェック
            # マニアックな用語の検索時にスコアが低くても救済するためのロジック
            is_exact_match = query.lower() in text.lower()
            
            # 完全一致の場合は閾値を大幅に緩和、それ以外は厳格な閾値を適用
            threshold = EXACT_MATCH_RELEVANCE_THRESHOLD if is_exact_match else DEFAULT_RELEVANCE_THRESHOLD
            
            if relevance_score is not None and relevance_score <= threshold:
                logger.debug(f"Result dropped due to low relevance_score: {relevance_score} <= {threshold}")
                continue
            if score is not None and score <= 0:
                logger.debug(f"Result dropped due to negative hybrid score: {score}")
                continue
                
            # チャンクごとにLLMへ渡すコンテキストをシミュレーションしてトークン数を計算
            context_text = f"Title: {result.get('title')}\nURL: {result.get('url')}\nContent:\n{result.get('text')}"
            tokens = count_tokens(context_text)
            
            # 上限を超える場合はこれ以降のチャンクを含めない（切り捨て）
            if current_tokens + tokens > self.max_tokens:
                logger.info(f"Token limit reached ({current_tokens + tokens} > {self.max_tokens}). Stopping accumulation.")
                break
                
            filtered_results.append({
                "title": result.get("title"),
                "url": result.get("url"),
                "text": result.get("text"),
                # LanceDBが返すスコア(_distance または _score または _relevance_score)
                "distance": result.get("_distance", None),
                "score": result.get("_score", None),
                "relevance_score": result.get("_relevance_score", None)
            })
            
            current_tokens += tokens
            
        logger.info(f"Search complete. Returning {len(filtered_results)} chunks (total tokens: {current_tokens}).")
        return filtered_results

    def read_page(self, page_id: str) -> str:
        """
        指定されたページIDの全てのチャンクを取得し、結合して返します。
        """
        chunks = self.db_client.get_chunks_by_page(page_id)
        if not chunks:
            return ""
        
        # チャンクをテキストで結合
        text_parts = []
        title = chunks[0].get("title", "")
        url = chunks[0].get("url", "")
        
        for c in chunks:
            text_parts.append(c.get("text", ""))
            
        full_text = "\n\n".join(text_parts)
        
        if tokens > self.max_tokens:
            logger.warning(f"Page {page_id} exceeds token limit ({tokens} > {self.max_tokens}).")
            # トークン数超過時は警告文を追加
            return f"Title: {title}\nURL: {url}\n\n[WARNING: Document is very long ({tokens} tokens) and may exceed context limits.]\n\n{full_text}"
            
        return f"Title: {title}\nURL: {url}\n\n{full_text}"

    def list_pages(self, limit: int = 50) -> list[dict]:
        """
        データベース内に存在するユニークなページ（ドキュメント）のリストを取得します。
        
        Args:
            limit (int): 取得する最大ページ数
            
        Returns:
            list[dict]: page_id, title, url を含む辞書のリスト
        """
        try:
            # LanceDBでは単純なDISTINCTが難しいため、多めに取得してメモリ上でユニーク化する
            chunks = self.db_client.table.search().limit(limit * 20).to_list()
            seen_pages = set()
            pages = []
            for c in chunks:
                page_id = c.get("page_id")
                if page_id not in seen_pages:
                    seen_pages.add(page_id)
                    pages.append({
                        "page_id": page_id,
                        "title": c.get("title", ""),
                        "url": c.get("url", "")
                    })
                    if len(pages) >= limit:
                        break
            return pages
        except Exception as e:
            logger.error(f"Failed to list pages: {e}")
            return []
