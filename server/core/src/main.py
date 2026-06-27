import os
import sys
import logging
from fastapi import FastAPI, HTTPException
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor

# srcモジュールへのパスを追加
sys.path.append(os.path.dirname(os.path.dirname(__file__)))

from src.api.search_api import router as search_router
from src.mcp_server.server import mcp
from src.utils.logging_config import setup_logging
from src.utils.telemetry import setup_telemetry

setup_logging()
logger = logging.getLogger("rrag-server-main")

setup_telemetry(service_name="rrag-server-main")

app = FastAPI(
    title="RRAG Integrated Server",
    description="Consolidated server for REST API and MCP Server",
    version="1.0.0"
)

# Instrument the main FastAPI app
FastAPIInstrumentor.instrument_app(app)

# 1. Mount MCP SSE App
# FastMCP's internal Starlette app handles /sse and /messages.
# By mounting it at /mcp, the endpoints become /mcp/sse and /mcp/messages.
app.mount("/mcp", mcp.sse_app)

# 2. Include Search REST API
# Prefix /api matches the existing routing and Caddyfile paths
app.include_router(search_router, prefix="/api")

# 3. Conditionally include Ingest API (Lazy import)
if os.environ.get("ENABLE_INGEST_API", "false").lower() == "true":
    logger.info("ENABLE_INGEST_API is true. Loading Ingest API router...")
    from src.api.ingest_api import router as ingest_router
    app.include_router(ingest_router, prefix="/ingest")
    logger.info("Ingest API mounted at /ingest")
else:
    logger.info("ENABLE_INGEST_API is false. Ingest API is disabled.")

# 4. Unified Health Checks
@app.get("/healthz", tags=["Health"], summary="Health check")
async def healthz():
    return {"status": "ok"}

@app.get("/readyz", tags=["Health"], summary="Readiness check")
async def readyz():
    try:
        from src.api.search_api import _get_searcher
        searcher = _get_searcher()
        # 疎通確認
        searcher.list_pages(limit=1)
        return {"status": "ready"}
    except Exception as e:
        logger.error(f"Readiness check failed: {e}")
        raise HTTPException(status_code=503, detail="Service Unavailable")
