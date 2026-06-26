import os
import psutil
from src.mcp_server.searcher import WikiSearcher

def print_memory(label):
    process = psutil.Process(os.getpid())
    mem_info = process.memory_info()
    mem_mb = mem_info.rss / (1024 * 1024)
    print(f"{label}: {mem_mb:.2f} MB")

def main():
    print_memory("Initial memory")
    
    # 検索器の初期化（この時点ではまだモデルはロードされないはず）
    searcher = WikiSearcher()
    print_memory("After searcher init")
    
    # 初回の検索実行（ここでEmbeddingとRerankerのモデルがロードされる）
    print("Running first search (loading models)...")
    searcher.search("テスト検索", limit=1)
    print_memory("After 1st search (models loaded)")
    
    # 2回目の検索
    print("Running second search...")
    searcher.search("人工知能", limit=5)
    print_memory("After 2nd search")

if __name__ == "__main__":
    main()
