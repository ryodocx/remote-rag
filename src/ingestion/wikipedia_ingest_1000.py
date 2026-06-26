import time
import sys
import wikipedia
from src.database.client import DatabaseClient
from src.ingestion.chunker import chunk_markdown
from src.ingestion.normalize_html import normalize_to_markdown

sys.stdout.reconfigure(encoding='utf-8')

def ingest_1000_pages():
    wikipedia.set_lang("ja")
    # Wikipedia APIからのブロックを防ぐため、User-Agentを設定
    wikipedia.set_user_agent("RemoteRAGBot/1.2 (ryodocx)")
    client = DatabaseClient()
    
    total_target = 1000
    batch_size = 50 # APIに一度に要求するランダム記事数
    total_chunks = 0
    success_count = 0
    attempt_count = 0
    
    print(f"Starting to process {total_target} pages...")
    
    while attempt_count < total_target:
        # APIのランダム取得上限などを考慮し、バッチごとに取得する
        try:
            titles = wikipedia.random(min(batch_size, total_target - attempt_count))
            if isinstance(titles, str):
                titles = [titles]
        except Exception as e:
            print(f"Failed to fetch random titles: {e}")
            time.sleep(10)
            continue
            
        for title in titles:
            if attempt_count >= total_target:
                break
                
            attempt_count += 1
            try:
                print(f"[{attempt_count}/{total_target}] Fetching: {title}")
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
                    total_chunks += len(chunks)
                    success_count += 1
                    print(f"  -> Inserted {len(chunks)} chunks (Total chunks added in this run: {total_chunks})")
                
                # ブロック回避のための待機
                time.sleep(2)
                
            except wikipedia.exceptions.DisambiguationError:
                print(f"  -> Skipped: Disambiguation page")
                time.sleep(1)
            except wikipedia.exceptions.PageError:
                print(f"  -> Skipped: Page not found")
                time.sleep(1)
            except Exception as e:
                print(f"  -> Error processing {title}: {e}")
                # エラー発生時は少し長めにペナルティ待機を入れる
                time.sleep(5)
                
    print(f"\n--- Ingestion Complete ---")
    print(f"Successfully processed {success_count}/{attempt_count} pages.")
    print(f"Total new chunks inserted: {total_chunks}")
    
    if total_chunks > 0:
        try:
            client.create_fts_index()
            print("Re-created FTS index for hybrid search.")
        except Exception as e:
            print(f"Failed to create FTS index: {e}")

if __name__ == "__main__":
    ingest_1000_pages()
