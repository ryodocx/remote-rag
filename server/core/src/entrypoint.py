"""
RRAG Server 統合エントリーポイント。
単一の FastAPI プロセスを立ち上げます。
"""
import os
import logging
import uvicorn

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)
logger = logging.getLogger("rrag-entrypoint")

def main():
    port = int(os.environ.get("PORT", "8000"))
    logger.info(f"Starting RRAG Integrated Server on port {port}")
    uvicorn.run("src.main:app", host="0.0.0.0", port=port, log_level="info")

if __name__ == "__main__":
    main()
