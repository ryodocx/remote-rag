import sys
import traceback
from src.database.client import DatabaseClient

def main():
    sys.stdout.reconfigure(encoding='utf-8')
    client = DatabaseClient()
    
    unique_word = "STALEFTSBUG12345"
    
    chunk = [{
        "chunk_id": "test_chunk_1",
        "page_id": "test_page_1",
        "text": f"This is a test document containing {unique_word}.",
        "title": "Test Document",
        "url": "http://test"
    }]
    
    print("Adding chunk...")
    client.add_chunks(chunk)
    
    print("Creating FTS index...")
    client.create_fts_index()
    
    print("Searching for unique word (should succeed)...")
    res = client.table.search(unique_word, query_type="hybrid").limit(5).to_list()
    print(f"Found {len(res)} results.")
    
    print("Deleting chunk...")
    client.delete_chunks_by_page("test_page_1")
    
    print("Searching for unique word after deletion (stale FTS index)...")
    try:
        res2 = client.table.search(unique_word, query_type="hybrid").limit(5).to_list()
        print(f"Found {len(res2)} results.")
    except Exception as e:
        print("Hybrid Search Error due to stale FTS index!")
        traceback.print_exc()

if __name__ == "__main__":
    main()
