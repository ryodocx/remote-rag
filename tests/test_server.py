import sys
import os

# src モジュールをインポートできるようにパスを追加
sys.path.append(os.path.dirname(os.path.dirname(__file__)))

from src.mcp_server.server import search_wiki

def test_search():
    print("=== Testing Query: 'ラーメン' ===")
    result = search_wiki("ラーメン", limit=2)
    print(result)
    
    print("\n=== Testing Query: 'アーキテクチャ' ===")
    result2 = search_wiki("アーキテクチャ", limit=2)
    print(result2)

if __name__ == "__main__":
    test_search()
