import argparse
import sys
from src.utils.logging_config import setup_logging
from src.mcp_server.searcher import WikiSearcher

def main():
    if sys.stdout.encoding != 'utf-8':
        sys.stdout.reconfigure(encoding='utf-8')
    
    # エントリーポイントでのみロギングを設定
    setup_logging()
    
    parser = argparse.ArgumentParser(description="Search LanceDB Wiki RAG directly")
    parser.add_argument("queries", type=str, nargs='+', help="The search queries (one or more)")
    parser.add_argument("--limit", type=int, default=5, help="Number of results to return")
    parser.add_argument("--max-tokens", type=int, default=4000, help="Maximum tokens for context")
    parser.add_argument("--search-type", type=str, choices=["vector", "fts", "hybrid"], default="hybrid", help="Search type (vector, fts, hybrid)")
    parser.add_argument("--update-fts", action="store_true", help="Update the Full-Text Search index before searching")
    
    args = parser.parse_args()
    
    try:
        searcher = WikiSearcher(max_tokens=args.max_tokens)
        
        if args.update_fts:
            print("Updating FTS index...")
            searcher.db_client.create_fts_index()
            print("FTS index updated.")
            
        for query in args.queries:
            print(f"Searching for: '{query}' (type: {args.search_type}, limit: {args.limit}, max_tokens: {args.max_tokens})")
            
            results = searcher.search(query, limit=args.limit, search_type=args.search_type)
            
            if not results:
                print("No results found.\n")
                continue
                
            print(f"\nFound {len(results)} results:\n")
            for i, result in enumerate(results, 1):
                print(f"--- Result {i} ---")
                print(f"Title: {result.get('title')}")
                print(f"URL: {result.get('url')}")
                relevance = result.get('relevance_score')
                score = result.get('score')
                distance = result.get('distance')
                if relevance is not None:
                    print(f"一致度 (Relevance): {relevance}")
                elif score is not None:
                    print(f"一致度 (Score): {score}")
                elif distance is not None:
                    print(f"一致度 (Distance): {distance}")
                print("Content Snippet:")
                text = result.get('text', '')
                # Print a snippet of the content (first 300 characters)
                snippet = text[:300] + "..." if len(text) > 300 else text
                print(f"{snippet}\n")
            
            print("=" * 40 + "\n")
            
    except Exception as e:
        print(f"Error during search: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()
