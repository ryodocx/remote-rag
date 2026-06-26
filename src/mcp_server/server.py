from mcp.server.fastmcp import FastMCP
from src.mcp_server.searcher import WikiSearcher

# MCPサーバーのインスタンスを作成
mcp = FastMCP("WikiRAG")

# 検索用のクラスを初期化（トークン上限4000）
searcher = WikiSearcher(max_tokens=4000)

@mcp.tool()
def search_wiki(query: str, limit: int = 5) -> str:
    """
    ナレッジベース（Wiki）からクエリに関連する情報を検索します。
    
    Args:
        query: 検索クエリ (String)
        limit: 取得件数 (Integer)
    """
    results = searcher.search(query, limit=limit)
    
    if not results:
        return "No relevant information found in the Wiki."
        
    formatted = []
    for i, res in enumerate(results, 1):
        score_text = ""
        if res.get('relevance_score') is not None:
            score_text = f"- Relevance Score: {res['relevance_score']:.2f}\n"
        elif res.get('score') is not None:
            score_text = f"- FTS Score: {res['score']:.2f}\n"
        elif res.get('distance') is not None:
            score_text = f"- Vector Distance: {res['distance']:.2f}\n"
            
        formatted.append(f"### {res['title']}\n- URL: {res['url']}\n{score_text}- Content:\n{res['text']}")
        
    return "\n\n".join(formatted)

if __name__ == "__main__":
    # MCPサーバーを実行 (デフォルトで標準入出力による通信となります)
    mcp.run()
