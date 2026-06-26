import sys
import traceback
from src.database.client import DatabaseClient

def main():
    sys.stdout.reconfigure(encoding='utf-8')
    client = DatabaseClient()
    print("Executing FTS search...")
    try:
        results_fts = client.table.search("テスト", query_type="fts").limit(5).to_list()
        print(f"FTS Search success: {len(results_fts)} rows")
    except Exception as e:
        print(f"FTS Search Error:")
        traceback.print_exc()

    print("Executing Vector search...")
    try:
        results_vec = client.table.search("テスト", query_type="vector").limit(5).to_list()
        print(f"Vector Search success: {len(results_vec)} rows")
    except Exception as e:
        print(f"Vector Search Error:")
        traceback.print_exc()

    print("Executing Hybrid search (without Reranker)...")
    try:
        # without reranker first
        results_hyb = client.table.search("テスト", query_type="hybrid").limit(5).to_list()
        print(f"Hybrid Search success: {len(results_hyb)} rows")
    except Exception as e:
        print(f"Hybrid Search Error:")
        traceback.print_exc()

    print("Re-creating FTS index...")
    client.create_fts_index()
    print("Executing Hybrid search after index recreation...")
    try:
        results_hyb2 = client.table.search("テスト", query_type="hybrid").limit(5).to_list()
        print(f"Hybrid Search 2 success: {len(results_hyb2)} rows")
    except Exception as e:
        print(f"Hybrid Search 2 Error:")
        traceback.print_exc()

if __name__ == "__main__":
    main()
