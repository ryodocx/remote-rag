from src.database.client import DatabaseClient
from src.utils.token_counter import count_tokens

class WikiSearcher:
    def __init__(self, max_tokens: int = 4000):
        self.db_client = DatabaseClient()
        self.max_tokens = max_tokens

    def search(self, query: str, limit: int = 5) -> list[dict]:
        """
        クエリを用いて検索を実行し、トークン数上限に収まるようにチャンクをフィルタリングして返します。
        """
        # DBから検索結果を取得（ハイブリッド検索またはベクトル検索）
        raw_results = self.db_client.search(query, limit=limit)
        
        filtered_results = []
        current_tokens = 0
        
        for result in raw_results:
            # チャンクごとにLLMへ渡すコンテキストをシミュレーションしてトークン数を計算
            context_text = f"Title: {result.get('title')}\nURL: {result.get('url')}\nContent:\n{result.get('text')}"
            tokens = count_tokens(context_text)
            
            # 上限を超える場合はこれ以降のチャンクを含めない（切り捨て）
            if current_tokens + tokens > self.max_tokens:
                break
                
            filtered_results.append({
                "title": result.get("title"),
                "url": result.get("url"),
                "text": result.get("text"),
                # LanceDBが返すスコア(_distance または _score)
                "distance": result.get("_distance", None),
                "score": result.get("_score", None)
            })
            
            current_tokens += tokens
            
        return filtered_results
