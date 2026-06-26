import os
import sys
import logging
from typing import List, Optional
from pydantic import BaseModel, Field
from fastapi import FastAPI, HTTPException, BackgroundTasks
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor

# srcモジュールへのパスを追加
sys.path.append(os.path.dirname(os.path.dirname(os.path.dirname(__file__))))

from src.database.client import DatabaseClient
from src.ingestion.chunker import chunk_markdown
from src.utils.logging_config import setup_logging
from src.utils.telemetry import setup_telemetry

setup_logging()
logger = logging.getLogger(__name__)

# Setup OpenTelemetry
setup_telemetry(service_name="rrag-ingest-api")

app = FastAPI(title="RRAG Ingestion Webhook API")
FastAPIInstrumentor.instrument_app(app)

class IngestDocument(BaseModel):
    page_id: str
    title: str
    text: str = Field(..., description="Markdown or raw text content of the document")
    url: str
    metadata: Optional[str] = Field(default="{}", description="JSON string of metadata")

class IngestRequest(BaseModel):
    documents: List[IngestDocument]

def background_ingest(documents: List[IngestDocument]):
    try:
        client = DatabaseClient()
        all_chunks = []
        
        for doc in documents:
            # 1. 既存のチャンクを削除 (更新を想定)
            client.delete_chunks_by_page(doc.page_id)
            
            # 2. テキストのチャンク化
            chunks = chunk_markdown(doc.text, page_id=doc.page_id, title=doc.title, url=doc.url)
            
            # メタデータの付与
            for c in chunks:
                c["metadata"] = doc.metadata
                
            all_chunks.extend(chunks)
            logger.info(f"Generated {len(chunks)} chunks for {doc.page_id}")
            
        if all_chunks:
            # 3. データベースへのUpsert
            client.add_chunks(all_chunks)
            # 4. FTSインデックスは自動作成せず、明示的な最適化エンドポイントに委ねる
            logger.info(f"Successfully ingested {len(all_chunks)} chunks in background.")
            
    except Exception as e:
        logger.error(f"Error during background ingestion: {e}")

@app.post("/ingest")
async def ingest_webhook(request: IngestRequest, background_tasks: BackgroundTasks):
    """
    Webhook endpoint to receive documents from external systems (Airbyte, Dify, etc.)
    and ingest them into LanceDB asynchronously.
    """
    if not request.documents:
        raise HTTPException(status_code=400, detail="No documents provided")
        
    background_tasks.add_task(background_ingest, request.documents)
    return {"status": "accepted", "message": f"Ingestion for {len(request.documents)} documents started in background"}

def background_optimize():
    try:
        client = DatabaseClient()
        client.optimize()
        logger.info("Background optimization completed successfully.")
    except Exception as e:
        logger.error(f"Error during background optimization: {e}")

@app.post("/optimize")
async def optimize_database(background_tasks: BackgroundTasks):
    """
    Endpoint to trigger database optimization and FTS index rebuild.
    """
    background_tasks.add_task(background_optimize)
    return {"status": "accepted", "message": "Database optimization started in background"}
