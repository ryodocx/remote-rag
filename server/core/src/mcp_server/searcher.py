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

    def search(self, query: str, limit: int = 5, search_type: str = "hybrid") -> list[dict]:
        """
        クエリを用いて検索を実行し、ノイズとなる低スコアのチャンクを除外した上で、
        トークン数上限に収まるように結果をフィルタリングして返します。
        
        Args:
            query (str): 検索する文字列。
            limit (int): 取得を試みる初期の最大検索結果件数（デフォルト: 5）。
            search_type (str): 検索手法（デフォルト: 'hybrid'）。
            
        Returns:
            list[dict]: フィルタリングされた、LLMへ渡すのに適した検索結果のリスト。
        """
        logger.info(f"Executing search for query: '{query}' (type: {search_type}, limit: {limit})")
        # DBから検索結果を取得（ハイブリッド検索またはベクトル検索）
        raw_results = self.db_client.search(query, limit=limit, search_type=search_type)
        
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
