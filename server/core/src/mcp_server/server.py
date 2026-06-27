"""
MCP (Model Context Protocol) サーバーのエントリーポイント。
FastMCPを使用して、Wikipedia RAGエンジンの検索機能を外部のAIエージェントに公開します。
"""
import logging
from mcp.server.fastmcp import FastMCP
from src.utils.logging_config import setup_logging
from src.utils.telemetry import setup_telemetry, get_tracer, get_meter

logger = logging.getLogger(__name__)
tracer = get_tracer(__name__)
meter = get_meter(__name__)

# Metrics
search_counter = meter.create_counter(
    "rrag.search.count",
    description="Number of search requests processed",
)

# MCPサーバーのインスタンスを作成
mcp = FastMCP("WikiRAG")

# 検索用のクラスを遅延初期化（モジュール読み込み時にDB接続を行わない）
_searcher = None

def _get_searcher():
    """WikiSearcher の遅延初期化。初回呼び出し時のみインスタンスを生成する。"""
    global _searcher
    if _searcher is None:
        import os
        from src.mcp_server.searcher import WikiSearcher
        max_tokens = int(os.environ.get("WIKI_SEARCH_MAX_TOKENS", "4000"))
        _searcher = WikiSearcher(max_tokens=max_tokens)
    return _searcher

@mcp.tool()
def search_wiki(query: str, limit: int = 5, metadata_filter: str = None) -> str:
    """
    社内ナレッジベース（Wikipediaデータ）から、指定されたクエリに関連する情報を検索します。
    ハイブリッド検索（ベクトル＋キーワード）とRerankerを組み合わせた高精度な検索を実行し、
    LLMが解釈しやすいフォーマットの文字列として返却します。
    
    Args:
        query: 検索クエリ文字列。自然言語での質問や、単語の羅列などを指定します。
        limit: 取得したい最大件数。デフォルトは5件。
        metadata_filter: オプション。メタデータ（JSON）での絞り込み文字列 (例: '"category": "IT"')
    """
    with tracer.start_as_current_span("search_wiki") as span:
        span.set_attribute("search.query", query)
        span.set_attribute("search.limit", limit)
        search_counter.add(1, {"metadata_filter": bool(metadata_filter)})
        
        searcher = _get_searcher()
        
        where_clause = None
        if metadata_filter:
            safe_filter = metadata_filter.replace("'", "''").replace("\\", "\\\\")
            where_clause = f"metadata LIKE '%{safe_filter}%'"
            
        results = searcher.search(query, limit=limit, where=where_clause)
        
        if not results:
            span.set_attribute("search.results_count", 0)
            return "No relevant information found in the Wiki."
            
        span.set_attribute("search.results_count", len(results))
        
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

@mcp.tool()
def read_wiki_page(page_id: str) -> str:
    """
    指定されたページID（WikiのID等）から文書全体を読み込みます。
    
    Args:
        page_id: 取得したいページのID文字列。
    """
    with tracer.start_as_current_span("read_wiki_page") as span:
        span.set_attribute("page_id", page_id)
        searcher = _get_searcher()
        content = searcher.read_page(page_id)
        if not content:
            return f"No document found for page_id: {page_id}"
        return content

@mcp.tool()
def search_by_metadata(key: str, value: str, limit: int = 5) -> str:
    """
    メタデータの Key:Value に完全一致する文書を検索します。
    
    Args:
        key: メタデータのキー（例: "category"）
        value: メタデータの値（例: "IT"）
        limit: 取得したい最大件数。デフォルトは5件。
    """
    searcher = _get_searcher()
    
    safe_key = key.replace("'", "''").replace("\\", "\\\\")
    safe_value = value.replace("'", "''").replace("\\", "\\\\")
    where_clause = f"metadata LIKE '%\"{safe_key}\": \"{safe_value}\"%'"
    
    results = searcher.search(query="", limit=limit, search_type="fts", where=where_clause)
    
    if not results:
        return f"No document found for metadata {key}:{value}."
        
    formatted = []
    for i, res in enumerate(results, 1):
        formatted.append(f"### {res['title']}\n- URL: {res['url']}\n- Content Snippet:\n{res['text'][:300]}...")
        
    return "\n\n".join(formatted)

@mcp.tool()
def list_wiki_pages(limit: int = 50) -> str:
    """
    社内ナレッジベース（Wikipediaデータ）に存在するページの一覧を取得します。
    どのようなドキュメントが存在するか把握したい場合や、要約・全件取得のための
    page_id を探す際に使用します。
    
    Args:
        limit: 取得したい最大件数。デフォルトは50件。
    """
    with tracer.start_as_current_span("list_wiki_pages") as span:
        span.set_attribute("limit", limit)
        searcher = _get_searcher()
        pages = searcher.list_pages(limit=limit)
        
        if not pages:
            span.set_attribute("results_count", 0)
            return "No pages found in the database."
            
        span.set_attribute("results_count", len(pages))
        
        formatted = ["### Available Pages"]
        for i, p in enumerate(pages, 1):
            formatted.append(f"{i}. **{p['title']}** (ID: `{p['page_id']}`) - {p['url']}")
            
        return "\n".join(formatted)

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
    
    # エントリーポイントでのみロギング・テレメトリを設定
    setup_logging()
    setup_telemetry(service_name="rrag-mcp-server")
    
    # SSE利用時のhost/port設定を反映
    mcp.settings.host = args.host
    mcp.settings.port = args.port
    
    if args.transport == "sse":
        logger.info(f"Starting MCP server with SSE transport on http://{args.host}:{args.port}")
    else:
        logger.info("Starting MCP server with stdio transport")
        
    # MCPサーバーを実行
    mcp.run(transport=args.transport)
