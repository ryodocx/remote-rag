"""
MCP (Model Context Protocol) サーバーのエントリーポイント。
FastMCPを使用して、Wikipedia RAGエンジンの検索機能を外部のAIエージェントに公開します。
"""
import logging
from mcp.server.fastmcp import FastMCP
from src.utils.logging_config import setup_logging

logger = logging.getLogger(__name__)

# MCPサーバーのインスタンスを作成
mcp = FastMCP("WikiRAG")

# 検索用のクラスを遅延初期化（モジュール読み込み時にDB接続を行わない）
_searcher = None

def _get_searcher():
    """WikiSearcher の遅延初期化。初回呼び出し時のみインスタンスを生成する。"""
    global _searcher
    if _searcher is None:
        from src.mcp_server.searcher import WikiSearcher
        _searcher = WikiSearcher(max_tokens=4000)
    return _searcher

@mcp.tool()
def search_wiki(query: str, limit: int = 5) -> str:
    """
    社内ナレッジベース（Wikipediaデータ）から、指定されたクエリに関連する情報を検索します。
    ハイブリッド検索（ベクトル＋キーワード）とRerankerを組み合わせた高精度な検索を実行し、
    LLMが解釈しやすいフォーマットの文字列として返却します。
    
    Args:
        query: 検索クエリ文字列。自然言語での質問や、単語の羅列などを指定します。
        limit: 取得したい最大件数。デフォルトは5件。
    """
    searcher = _get_searcher()
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
    import argparse
    
    parser = argparse.ArgumentParser(description="Run the WikiRAG MCP Server")
    parser.add_argument("--transport", choices=["stdio", "sse"], default="stdio", 
                        help="Transport protocol to use (stdio or sse)")
    parser.add_argument("--host", type=str, default="127.0.0.1", 
                        help="Host to listen on for SSE transport (default: 127.0.0.1)")
    parser.add_argument("--port", type=int, default=8000, 
                        help="Port to listen on for SSE transport (default: 8000)")
    
    args = parser.parse_args()
    
    # エントリーポイントでのみロギングを設定
    setup_logging()
    
    # SSE利用時のhost/port設定を反映
    mcp.settings.host = args.host
    mcp.settings.port = args.port
    
    if args.transport == "sse":
        logger.info(f"Starting MCP server with SSE transport on http://{args.host}:{args.port}")
    else:
        logger.info("Starting MCP server with stdio transport")
        
    # MCPサーバーを実行
    mcp.run(transport=args.transport)
