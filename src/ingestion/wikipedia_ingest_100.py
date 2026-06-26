import time
import sys
import wikipedia
from src.database.client import DatabaseClient
from src.ingestion.chunker import chunk_markdown
from src.ingestion.normalize_html import normalize_to_markdown

# Windowsのcp932環境でのprint時のUnicodeEncodeErrorを防ぐ
sys.stdout.reconfigure(encoding='utf-8')

def ingest_100_pages():
    wikipedia.set_lang("ja")
    # Wikipedia APIからのブロックを防ぐため、User-Agentを設定
    wikipedia.set_user_agent("RemoteRAGBot/1.1 (ryodocx)")
    client = DatabaseClient()
    
    print("Fetching 100 random Wikipedia titles...")
    try:
        # ランダムな100件のタイトルを取得
        titles = wikipedia.random(100)
    except Exception as e:
        print(f"Failed to fetch random titles: {e}")
        return

    print(f"Starting to process {len(titles)} pages with sleep intervals to avoid API blocks...")
    
    total_chunks = 0
    success_count = 0
    
    for i, title in enumerate(titles):
        try:
            print(f"[{i+1}/{len(titles)}] Fetching: {title}")
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
                # 順次DBにInsertしていく
                client.add_chunks(chunks)
                total_chunks += len(chunks)
                success_count += 1
                print(f"  -> Inserted {len(chunks)} chunks (Total so far: {total_chunks})")
            
            # 連続リクエストによるブロック（Expecting value error等）を防ぐため、2秒待機
            time.sleep(2)
            
        except wikipedia.exceptions.DisambiguationError:
            print(f"  -> Skipped: Disambiguation page")
        except wikipedia.exceptions.PageError:
            print(f"  -> Skipped: Page not found")
        except Exception as e:
            print(f"  -> Error processing {title}: {e}")
            # エラー発生時は少し長めにペナルティ待機を入れる
            time.sleep(5)
            
    print(f"\n--- Ingestion Complete ---")
    print(f"Successfully processed {success_count}/{len(titles)} pages.")
    print(f"Total new chunks inserted: {total_chunks}")
    
    if total_chunks > 0:
        try:
            client.create_fts_index()
            print("Re-created FTS index for hybrid search.")
        except Exception as e:
            print(f"Failed to create FTS index: {e}")

if __name__ == "__main__":
    ingest_100_pages()
