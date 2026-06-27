import os
import sys
import glob
import logging

# srcモジュールを参照できるようにパスを追加
sys.path.append(os.path.dirname(os.path.dirname(__file__)))

from src.database.client import DatabaseClient
from src.ingestion.chunker import chunk_markdown
from src.ingestion.normalize_html import normalize_to_markdown
from src.utils.logging_config import setup_logging

logger = logging.getLogger(__name__)

def run_dummy_ingestion():
    client = DatabaseClient()
    dummy_dir = os.path.join(os.path.dirname(os.path.dirname(__file__)), "data", "dummy")

    md_files = glob.glob(os.path.join(dummy_dir, "*.md"))
    all_chunks = []

    for file_path in md_files:
        basename = os.path.basename(file_path)
        page_id = basename.split(".")[0] # e.g. sample1
        title = f"Title of {page_id}"
        url = f"https://dummy-wiki.local/{page_id}"

        with open(file_path, "r", encoding="utf-8") as f:
            raw_text = f.read()

        # 実運用ではWiki APIからHTMLを取得する想定。ここではダミーテキスト(MD)をそのまま通すが、構造上はHTML正規化を挟む。
        text = normalize_to_markdown(raw_text)

        # 1. 既存のチャンクを削除 (更新を想定したクリーンアップ)
        client.delete_chunks_by_page(page_id)

        # 2. テキストのチャンク化
        chunks = chunk_markdown(text, page_id=page_id, title=title, url=url)
        all_chunks.extend(chunks)
        logger.info(f"Processed {file_path}, generated {len(chunks)} chunks.")

    # 3. データベースへのUpsert
    if all_chunks:
        client.add_chunks(all_chunks)
        logger.info(f"Successfully inserted {len(all_chunks)} chunks to LanceDB.")

        # 4. ハイブリッド検索用にFTSインデックスを作成
        try:
            client.create_fts_index()
            logger.info("Created FTS index.")
        except Exception as e:
            logger.error(f"Failed to create FTS index (tantivy is required): {e}")
    else:
        logger.info("No chunks to insert.")

if __name__ == "__main__":
    setup_logging()
    run_dummy_ingestion()
