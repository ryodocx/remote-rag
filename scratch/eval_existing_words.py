import os
import sys
import random
import re
from src.database.client import DatabaseClient
from src.mcp_server.searcher import WikiSearcher

def main():
    sys.stdout.reconfigure(encoding='utf-8')
    db_client = DatabaseClient()
    searcher = WikiSearcher()
    
    table = db_client.table
    print(f"Total rows in DB: {table.count_rows()}")
    
    # Load all rows to sample from
    rows = table.to_arrow().to_pylist()
    print(f"Fetched {len(rows)} rows.")
    
    # Extract long words
    # Regex for continuous Kanji/Katakana sequence of length 6 to 20
    pattern = re.compile(r'[一-龥ァ-ンヴー]{6,20}')
    
    test_cases = []
    random.shuffle(rows)
    
    for row in rows:
        text = row.get("text", "")
        title = row.get("title", "")
        url = row.get("url", "")
        
        matches = pattern.findall(text)
        if matches:
            phrase = random.choice(matches)
            test_cases.append({
                "phrase": phrase,
                "source_title": title,
                "source_url": url,
                "source_text": text
            })
            
        if len(test_cases) == 100:
            break
            
    print(f"Collected {len(test_cases)} phrases for testing.")
    
    report = "# Evaluation with 100 Phrases Existing in the DB\n\n"
    success_count = 0
    
    for i, case in enumerate(test_cases, 1):
        phrase = case["phrase"]
        report += f"### Test {i}: `{phrase}`\n"
        report += f"- **Source Title**: {case['source_title']}\n"
        
        try:
            results = searcher.search(phrase, limit=5, search_type="hybrid")
            
            if not results:
                report += "- **Result**: ❌ No results returned.\n\n"
                continue
                
            urls = [r.get("url") for r in results]
            if case["source_url"] in urls:
                idx = urls.index(case["source_url"])
                res = results[idx]
                relevance = res.get('relevance_score', 'N/A')
                report += f"- **Result**: ✅ Source document found at rank {idx+1}. (Relevance: {relevance})\n"
                success_count += 1
            else:
                top_relevance = results[0].get('relevance_score', 'N/A')
                report += f"- **Result**: ⚠️ Results returned, but source document not found in top 5. (Top Relevance: {top_relevance})\n"
                if phrase in results[0].get("text", ""):
                    report += f"  - *Note*: The phrase was found in the top retrieved document ({results[0].get('title')}).\n"
                    success_count += 1
                
            report += "\n"
                
        except Exception as e:
            report += f"- **Result**: ❌ Error during search: {e}\n\n"
            
    report += f"---\n**Summary**: {success_count} out of {len(test_cases)} queries successfully retrieved relevant documents.\n"
    
    out_path = r"C:\Users\ryotn\.gemini\antigravity\brain\258ce460-7f14-4094-8e10-4ccdcc9fdac5\eval_existing_words.md"
    with open(out_path, "w", encoding="utf-8") as f:
        f.write(report)
        
    print(f"Report generated at: {out_path}")

if __name__ == "__main__":
    main()
