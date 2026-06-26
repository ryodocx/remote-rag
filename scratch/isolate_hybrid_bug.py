import sys
import random
import re
import traceback
from src.database.client import DatabaseClient

def main():
    sys.stdout.reconfigure(encoding='utf-8')
    client = DatabaseClient()
    table = client.table
    
    rows = table.to_arrow().to_pylist()
    print(f"Total rows: {len(rows)}")
    
    pattern = re.compile(r'[一-龥ァ-ンヴー]{6,20}')
    
    test_phrases = []
    for row in rows:
        text = row.get("text", "")
        matches = pattern.findall(text)
        if matches:
            test_phrases.extend(matches)
            
    # Sample 5000 to find the bug
    test_phrases = random.sample(test_phrases, min(5000, len(test_phrases)))
    print(f"Testing {len(test_phrases)} phrases to isolate the bug...")
    
    for i, phrase in enumerate(test_phrases):
        if i % 100 == 0:
            print(f"Progress: {i}/{len(test_phrases)}")
            
        try:
            # We bypass the python try/except in client.search to catch the raw LanceDB error
            table.search(phrase, query_type="hybrid").limit(5).to_list()
        except Exception as e:
            if "Invalid argument error: all columns in a record batch must have the same length" in str(e):
                print(f"\n[FOUND BUG] Phrase: '{phrase}'")
                print(f"Error: {e}")
                
                # Test FTS and Vector individually
                print("Testing FTS...")
                try:
                    table.search(phrase, query_type="fts").limit(5).to_list()
                    print("FTS ok.")
                except Exception as e_fts:
                    print(f"FTS failed: {e_fts}")
                    
                print("Testing Vector...")
                try:
                    table.search(phrase, query_type="vector").limit(5).to_list()
                    print("Vector ok.")
                except Exception as e_vec:
                    print(f"Vector failed: {e_vec}")
                    
                break

if __name__ == "__main__":
    main()
