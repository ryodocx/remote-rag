import os
import sys
import uuid
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

TASK_STORE = {}
MAX_TASKS = 1000

def cleanup_tasks():
    if len(TASK_STORE) > MAX_TASKS:
        keys_to_delete = list(TASK_STORE.keys())[:len(TASK_STORE)-MAX_TASKS]
        for k in keys_to_delete:
            del TASK_STORE[k]

class IngestDocument(BaseModel):
    page_id: str
    title: str
    text: str = Field(..., description="Markdown or raw text content of the document")
    url: str
    metadata: Optional[str] = Field(default="{}", description="JSON string of metadata")

class IngestRequest(BaseModel):
    documents: List[IngestDocument]

def background_ingest(task_id: str, documents: List[IngestDocument]):
    TASK_STORE[task_id] = {"status": "processing", "message": "Ingestion started"}
    try:
        client = DatabaseClient()
        all_chunks = []
        failed_docs = []
        
        for doc in documents:
            try:
                # 1. 既存のチャンクを削除 (更新を想定)
                client.delete_chunks_by_page(doc.page_id)
                
                # 2. テキストのチャンク化
                chunks = chunk_markdown(doc.text, page_id=doc.page_id, title=doc.title, url=doc.url)
                
                # メタデータの付与
                for c in chunks:
                    c["metadata"] = doc.metadata
                    
                all_chunks.extend(chunks)
                logger.info(f"Generated {len(chunks)} chunks for {doc.page_id}")
            except Exception as e:
                logger.error(f"Failed to process document {doc.page_id}: {e}")
                failed_docs.append({"page_id": doc.page_id, "error": str(e)})
            
        if all_chunks:
            # 3. データベースへのUpsert
            try:
                client.add_chunks(all_chunks)
                logger.info(f"Successfully ingested {len(all_chunks)} chunks in background.")
            except Exception as e:
                logger.error(f"Failed to insert chunks to DB: {e}")
                TASK_STORE[task_id] = {"status": "failed", "error": f"DB Insertion failed: {e}", "failed_docs": failed_docs}
                return
            
        message = f"Successfully ingested {len(all_chunks)} chunks."
        if failed_docs:
            message += f" ({len(failed_docs)} documents failed)."
            
        TASK_STORE[task_id] = {
            "status": "completed" if not failed_docs else "partial_success",
            "message": message,
            "failed_docs": failed_docs
        }
            
    except Exception as e:
        logger.error(f"Error during background ingestion setup: {e}")
        TASK_STORE[task_id] = {"status": "failed", "error": str(e)}

@app.post("/ingest")
async def ingest_webhook(request: IngestRequest, background_tasks: BackgroundTasks):
    """
    Webhook endpoint to receive documents from external systems (Airbyte, Dify, etc.)
    and ingest them into LanceDB asynchronously.
    """
    if not request.documents:
        raise HTTPException(status_code=400, detail="No documents provided")
        
    task_id = str(uuid.uuid4())
    TASK_STORE[task_id] = {"status": "pending"}
    cleanup_tasks()
    
    background_tasks.add_task(background_ingest, task_id, request.documents)
    return {"status": "accepted", "task_id": task_id, "message": f"Ingestion for {len(request.documents)} documents started in background"}

def background_optimize(task_id: str):
    TASK_STORE[task_id] = {"status": "processing"}
    try:
        client = DatabaseClient()
        client.optimize()
        logger.info("Background optimization completed successfully.")
        TASK_STORE[task_id] = {"status": "completed", "message": "Optimization completed successfully"}
    except Exception as e:
        logger.error(f"Error during background optimization: {e}")
        TASK_STORE[task_id] = {"status": "failed", "error": str(e)}

@app.post("/optimize")
async def optimize_database(background_tasks: BackgroundTasks):
    """
    Endpoint to trigger database optimization and FTS index rebuild.
    """
    task_id = str(uuid.uuid4())
    TASK_STORE[task_id] = {"status": "pending"}
    cleanup_tasks()
    background_tasks.add_task(background_optimize, task_id)
    return {"status": "accepted", "task_id": task_id, "message": "Database optimization started in background"}

@app.get("/task/{task_id}")
async def get_task_status(task_id: str):
    if task_id not in TASK_STORE:
        raise HTTPException(status_code=404, detail="Task not found")
    return {"task_id": task_id, **TASK_STORE[task_id]}
