import wikipedia
from src.database.client import DatabaseClient
from src.ingestion.chunker import chunk_markdown
from src.ingestion.normalize_html import normalize_to_markdown

def ingest_wikipedia_pages():
    wikipedia.set_lang("ja")
    # Wikipedia APIからのブロックを防ぐため、User-Agentを設定
    wikipedia.set_user_agent("RemoteRAGBot/1.0 (ryodocx)")
    client = DatabaseClient()
    
    titles = [
        "徳川家康", "フランス革命", "寿司", "ラーメン", "エベレスト",
        "ガラパゴス諸島", "オリンピック", "サッカー", "レオナルド・ダ・ヴィンチ", "浮世絵",
        "心理学", "経済学", "ビートルズ", "歌舞伎", "源氏物語",
        "カレーライス", "ピラミッド", "月", "恐竜", "コーヒー"
    ]
    
    all_chunks = []
    
    for title in titles:
        try:
            print(f"Fetching: {title}")
            page = wikipedia.page(title)
            html_content = page.html()
            url = page.url
            page_id = page.pageid
            
            # HTML to Markdown (normalize_to_markdown will handle it)
            text = normalize_to_markdown(html_content)
            
            # Delete old chunks
            client.delete_chunks_by_page(str(page_id))
            
            # Chunk
            chunks = chunk_markdown(text, page_id=str(page_id), title=page.title, url=url)
            all_chunks.extend(chunks)
            
            print(f"-> Chunked '{page.title}' into {len(chunks)} chunks.")
        except Exception as e:
            print(f"Failed to process {title}: {e}")
            
    if all_chunks:
        client.add_chunks(all_chunks)
        print(f"Successfully inserted {len(all_chunks)} chunks to LanceDB.")
        try:
            client.create_fts_index()
            print("Created FTS index.")
        except Exception as e:
            print(f"Failed to create FTS index: {e}")
    else:
        print("No chunks to insert.")

if __name__ == "__main__":
    ingest_wikipedia_pages()
