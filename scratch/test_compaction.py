import sys
import traceback
from src.database.client import DatabaseClient

def main():
    sys.stdout.reconfigure(encoding='utf-8')
    client = DatabaseClient()
    table = client.table
    
    phrase = 'ヨーロッパカンファレンスリーグ'
    
    print(f"Testing FTS search for '{phrase}' BEFORE compaction...")
    try:
        table.search(phrase, query_type="fts").limit(5).to_list()
        print("Success BEFORE compaction!")
    except Exception as e:
        print(f"Failed BEFORE compaction: {e}")
        
    print("\nCompacting files...")
    table.optimize()
    table.cleanup_old_versions()
    
    print("Re-creating FTS index...")
    client.create_fts_index()
    
    print(f"\nTesting FTS search for '{phrase}' AFTER compaction...")
    try:
        table.search(phrase, query_type="fts").limit(5).to_list()
        print("Success AFTER compaction!")
    except Exception as e:
        print(f"Failed AFTER compaction: {e}")
        traceback.print_exc()

if __name__ == "__main__":
    main()
