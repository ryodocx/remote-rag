import time
import sys
import os

sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from src.mcp_server.searcher import WikiSearcher

def main():
    print("Loading models...")
    searcher = WikiSearcher()
    print("Models loaded.")
    
    test_queries = [
        "アインシュタインの相対性理論",
        "日本の四季と梅雨",
        "フランス料理のコース",
        "人工知能のディープラーニング",
        "プロ野球のドラフト会議",
        "徳川家康の外交政策",
        "スマートフォンの普及",
        "歌舞伎の隈取",
        "ブラックホールの特異点",
        "少子高齢化の労働力不足"
    ]
    
    total_time = 0
    
    print("Starting benchmark...")
    for q in test_queries:
        start_time = time.time()
        try:
            _ = searcher.search(q, limit=5)
        except Exception:
            pass
        end_time = time.time()
        elapsed = end_time - start_time
        print(f"Query: '{q}' -> {elapsed:.3f} seconds")
        total_time += elapsed
        
    avg_time = total_time / len(test_queries)
    print(f"\nAverage processing time per query: {avg_time:.3f} seconds")

if __name__ == "__main__":
    main()
