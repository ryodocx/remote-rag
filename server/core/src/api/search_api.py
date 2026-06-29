"""
Custom GPTs / Actions 向け REST API エンドポイント。

ChatGPT の Custom GPTs Actions から社内ナレッジベースへアクセスするための
OpenAPI 3.0 準拠の REST インターフェースを提供します。

既存の WikiSearcher を再利用しており、認証は上位の Caddy + Auth Helper が担当します。
このサービス自体は認証ロジックを持ちません。
"""
import os
import sys
import logging
from typing import Optional

from fastapi import APIRouter, HTTPException, Query
from fastapi.responses import JSONResponse
from pydantic import BaseModel, Field

# srcモジュールへのパスを追加
sys.path.append(os.path.dirname(os.path.dirname(os.path.dirname(__file__))))

from src.mcp_server.searcher import WikiSearcher
from src.utils.logging_config import setup_logging
from src.utils.telemetry import setup_telemetry, get_tracer, get_meter

setup_logging()
logger = logging.getLogger(__name__)

setup_telemetry(service_name="rrag-rest-api")

# ---------------------------------------------------------------------------
# FastAPI アプリ定義
# ---------------------------------------------------------------------------

router = APIRouter()

tracer = get_tracer(__name__)
meter = get_meter(__name__)
api_search_counter = meter.create_counter(
    "rrag.api.search.count",
    description="Number of REST API search requests",
)

# ---------------------------------------------------------------------------
# Searcher の遅延初期化
# ---------------------------------------------------------------------------

_searcher: Optional[WikiSearcher] = None


def _get_searcher() -> WikiSearcher:
    """WikiSearcher の遅延初期化。初回呼び出し時のみインスタンスを生成する。"""
    global _searcher
    if _searcher is None:
        max_tokens = int(os.environ.get("WIKI_SEARCH_MAX_TOKENS", "4000"))
        _searcher = WikiSearcher(max_tokens=max_tokens)
    return _searcher


# ---------------------------------------------------------------------------
# レスポンスモデル
# ---------------------------------------------------------------------------

class SearchResult(BaseModel):
    """検索結果の1件分。"""
    title: str = Field(..., description="ドキュメントのタイトル / Document title")
    url: str = Field(..., description="ドキュメントの URL / Document URL")
    text: str = Field(..., description="関連するテキストチャンク / Relevant text chunk")
    relevance_score: Optional[float] = Field(
        None,
        description="Reranker による関連スコア（高いほど関連性が高い）/ Reranker relevance score (higher is better)",
    )
    distance: Optional[float] = Field(
        None,
        description="ベクトル距離（低いほど類似）/ Vector distance (lower is more similar)",
    )
    score: Optional[float] = Field(
        None,
        description="ハイブリッド検索スコア / Hybrid search score",
    )


class SearchResponse(BaseModel):
    """検索APIのレスポンス。"""
    results: list[SearchResult] = Field(..., description="検索結果リスト / Search results")
    total: int = Field(..., description="返却件数 / Number of results returned")
    query: str = Field(..., description="実行した検索クエリ / Executed search query")


class PageSummary(BaseModel):
    """ページ一覧の1件分。"""
    page_id: str = Field(..., description="ページの一意 ID / Unique page ID")
    title: str = Field(..., description="ページのタイトル / Page title")
    url: str = Field(..., description="ページの URL / Page URL")


class PageListResponse(BaseModel):
    """ページ一覧APIのレスポンス。"""
    pages: list[PageSummary] = Field(..., description="ページ一覧 / List of pages")
    total: int = Field(..., description="返却件数 / Number of pages returned")


class PageContentResponse(BaseModel):
    """ページ全文取得APIのレスポンス。"""
    page_id: str = Field(..., description="ページ ID / Page ID")
    content: str = Field(..., description="ページの全文テキスト / Full page text content")


# ---------------------------------------------------------------------------
# エンドポイント
# ---------------------------------------------------------------------------

@router.get(
    "/search",
    response_model=SearchResponse,
    summary="ナレッジベース検索 / Search knowledge base",
    description=(
        "指定したクエリで社内ナレッジベースをハイブリッド検索（ベクトル + キーワード + Reranker）します。\n\n"
        "Search the internal knowledge base using hybrid search "
        "(vector + keyword + CrossEncoder reranker). "
        "Returns the most relevant document chunks."
    ),
    tags=["Search"],
)
async def search(
    query: str = Query(..., description="検索クエリ（自然言語 or キーワード）/ Search query (natural language or keywords)"),
    limit: int = Query(5, ge=1, le=20, description="最大取得件数 (1〜20) / Maximum results (1-20)"),
    metadata_filter: Optional[str] = Query(
        None,
        description=(
            'メタデータでの絞り込み文字列。例: \'"category": "IT"\' / '
            'Metadata filter string. e.g. \'"category": "IT"\''
        ),
    ),
) -> SearchResponse:
    with tracer.start_as_current_span("api_search") as span:
        span.set_attribute("search.query", query)
        span.set_attribute("search.limit", limit)
        api_search_counter.add(1)

        searcher = _get_searcher()

        where_clause = None
        if metadata_filter:
            safe_filter = metadata_filter.replace("'", "''").replace("\\", "\\\\")
            where_clause = f"metadata LIKE '%{safe_filter}%'"

        try:
            raw = searcher.search(query, limit=limit, where=where_clause)
        except Exception as e:
            logger.error(f"Search failed: {e}")
            raise HTTPException(status_code=500, detail=f"Search error: {e}")

        results = [
            SearchResult(
                title=r["title"],
                url=r["url"],
                text=r["text"],
                relevance_score=r.get("relevance_score"),
                distance=r.get("distance"),
                score=r.get("score"),
            )
            for r in raw
        ]

        span.set_attribute("search.results_count", len(results))
        return SearchResponse(results=results, total=len(results), query=query)


@router.get(
    "/search/metadata",
    response_model=SearchResponse,
    summary="メタデータ検索 / Search by metadata",
    description=(
        "メタデータの Key:Value に完全一致する文書を検索します。\n\n"
        "Find documents that exactly match a metadata key-value pair."
    ),
    tags=["Search"],
)
async def search_by_metadata(
    key: str = Query(..., description='メタデータキー。例: "category" / Metadata key. e.g. "category"'),
    value: str = Query(..., description='メタデータ値。例: "IT" / Metadata value. e.g. "IT"'),
    limit: int = Query(5, ge=1, le=20, description="最大取得件数 (1〜20) / Maximum results (1-20)"),
) -> SearchResponse:
    with tracer.start_as_current_span("api_search_metadata") as span:
        span.set_attribute("search.metadata_key", key)
        span.set_attribute("search.metadata_value", value)

        searcher = _get_searcher()

        safe_key = key.replace("'", "''").replace("\\", "\\\\")
        safe_value = value.replace("'", "''").replace("\\", "\\\\")
        where_clause = f"metadata LIKE '%\"{safe_key}\": \"{safe_value}\"%'"

        try:
            raw = searcher.search(query="", limit=limit, search_type="fts", where=where_clause)
        except Exception as e:
            logger.error(f"Metadata search failed: {e}")
            raise HTTPException(status_code=500, detail=f"Search error: {e}")

        results = [
            SearchResult(
                title=r["title"],
                url=r["url"],
                text=r["text"],
                relevance_score=r.get("relevance_score"),
                distance=r.get("distance"),
                score=r.get("score"),
            )
            for r in raw
        ]

        span.set_attribute("search.results_count", len(results))
        return SearchResponse(results=results, total=len(results), query=f"{key}:{value}")


@router.get(
    "/pages",
    response_model=PageListResponse,
    summary="ページ一覧取得 / List all pages",
    description=(
        "ナレッジベースに存在するページ（ドキュメント）の一覧を取得します。\n\n"
        "Returns a list of all document pages available in the knowledge base."
    ),
    tags=["Pages"],
)
async def list_pages(
    limit: int = Query(50, ge=1, le=200, description="最大取得件数 (1〜200) / Maximum results (1-200)"),
) -> PageListResponse:
    with tracer.start_as_current_span("api_list_pages") as span:
        span.set_attribute("limit", limit)

        searcher = _get_searcher()

        try:
            raw = searcher.list_pages(limit=limit)
        except Exception as e:
            logger.error(f"List pages failed: {e}")
            raise HTTPException(status_code=500, detail=f"List pages error: {e}")

        pages = [
            PageSummary(page_id=p["page_id"], title=p["title"], url=p["url"])
            for p in raw
        ]

        span.set_attribute("results_count", len(pages))
        return PageListResponse(pages=pages, total=len(pages))


@router.get(
    "/pages/{page_id}",
    response_model=PageContentResponse,
    summary="ページ全文取得 / Read full page content",
    description=(
        "指定した page_id のドキュメント全文を取得します。\n\n"
        "Retrieves the full text content of a specific document by its page ID. "
        "Use the `/pages` endpoint first to discover available page IDs."
    ),
    tags=["Pages"],
)
async def read_page(
    page_id: str,
) -> PageContentResponse:
    with tracer.start_as_current_span("api_read_page") as span:
        span.set_attribute("page_id", page_id)

        searcher = _get_searcher()

        try:
            content = searcher.read_page(page_id)
        except Exception as e:
            logger.error(f"Read page failed: {e}")
            raise HTTPException(status_code=500, detail=f"Read page error: {e}")

        if not content:
            raise HTTPException(
                status_code=404,
                detail=f"Page not found: {page_id}",
            )

        return PageContentResponse(page_id=page_id, content=content)



