import time
import sys
import argparse
import wikipedia
import logging
from src.database.client import DatabaseClient
from src.ingestion.chunker import chunk_markdown
from src.ingestion.normalize_html import normalize_to_markdown
from src.utils.logging_config import setup_logging

logger = logging.getLogger(__name__)

# Windowsのcp932環境でのprint時のUnicodeEncodeErrorを防ぐ
sys.stdout.reconfigure(encoding='utf-8')

def ingest_single_page(client: DatabaseClient, title: str) -> int:
    """1ページを処理する共通ロジック"""
    try:
        # auto_suggest=False にして、曖昧な検索によるエラーを防ぐ
        page = wikipedia.page(title, auto_suggest=False)
        html_content = page.html()
        url = page.url
        page_id = page.pageid
        
        # HTML to Markdown
        text = normalize_to_markdown(html_content)
        
        # Delete old chunks
        client.delete_chunks_by_page(str(page_id))
        
        # Chunk
        chunks = chunk_markdown(text, page_id=str(page_id), title=page.title, url=url)
        
        if chunks:
            client.add_chunks(chunks)
        return len(chunks)
    
    except wikipedia.exceptions.DisambiguationError:
        logger.info(f"Skipped '{title}': Disambiguation page")
    except wikipedia.exceptions.PageError:
        logger.warning(f"Skipped '{title}': Page not found")
    except Exception as e:
        logger.error(f"Error processing '{title}': {e}")
        # エラー時はペナルティ待機を呼び出し側で行うため再送出
        raise
    
    return 0

def ingest_wikipedia_pages(count: int = 20, sleep_sec: float = 2.0, specific_titles: list[str] = None):
    """
    指定件数または指定タイトルのWikipediaページをDBに取り込む。
    
    Args:
        count (int): ランダム取得するページ数。specific_titlesがある場合は無視される。
        sleep_sec (float): リクエスト間の待機時間(API制限回避用)。
        specific_titles (list[str]): 指定したタイトルのリスト。
    """
    wikipedia.set_lang("ja")
    # Wikipedia APIからのブロックを防ぐため、User-Agentを設定
    wikipedia.set_user_agent("RemoteRAGBot/1.3 (ryodocx)")
    client = DatabaseClient()
    
    if specific_titles:
        titles = specific_titles
        logger.info(f"Starting to process {len(titles)} explicitly specified pages...")
    else:
        logger.info(f"Fetching {count} random Wikipedia titles...")
        try:
            titles = wikipedia.random(count)
            if isinstance(titles, str):
                titles = [titles]
        except Exception as e:
            logger.error(f"Failed to fetch random titles: {e}")
            return

    total_chunks = 0
    success_count = 0
    
    for i, title in enumerate(titles):
        logger.info(f"[{i+1}/{len(titles)}] Fetching: {title}")
        try:
            chunks_inserted = ingest_single_page(client, title)
            if chunks_inserted > 0:
                total_chunks += chunks_inserted
                success_count += 1
                logger.info(f"  -> Inserted {chunks_inserted} chunks (Total so far: {total_chunks})")
            
            if i < len(titles) - 1:
                time.sleep(sleep_sec)
                
        except Exception:
            # エラー発生時は少し長めにペナルティ待機を入れる
            time.sleep(sleep_sec * 2.5)
            
    logger.info("--- Ingestion Complete ---")
    logger.info(f"Successfully processed {success_count}/{len(titles)} pages.")
    logger.info(f"Total new chunks inserted: {total_chunks}")
    
    if total_chunks > 0:
        try:
            client.create_fts_index()
            logger.info("Re-created FTS index for hybrid search.")
        except Exception as e:
            logger.error(f"Failed to create FTS index: {e}")

if __name__ == "__main__":
    setup_logging()
    
    parser = argparse.ArgumentParser(description="Ingest Wikipedia articles into LanceDB")
    parser.add_argument("--count", type=int, default=20, help="Number of random articles to ingest")
    parser.add_argument("--sleep", type=float, default=2.0, help="Seconds to sleep between requests")
    parser.add_argument("--titles", type=str, nargs="+", help="Specific article titles to ingest (overrides --count)")
    
    args = parser.parse_args()
    
    ingest_wikipedia_pages(
        count=args.count,
        sleep_sec=args.sleep,
        specific_titles=args.titles
    )
