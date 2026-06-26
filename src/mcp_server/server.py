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
        formatted.append(f"### {res['title']}\n- URL: {res['url']}\n- Content:\n{res['text']}")
        
    return "\n\n".join(formatted)

if __name__ == "__main__":
    # MCPサーバーを実行 (デフォルトで標準入出力による通信となります)
    mcp.run()
