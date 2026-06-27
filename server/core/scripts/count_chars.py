import lancedb
import os
from datetime import datetime
import sys

# src モジュールをインポートするためにパスを追加
sys.path.append(os.path.dirname(os.path.dirname(__file__)))
from src.database.client import DB_PATH

db = lancedb.connect(DB_PATH)
try:
    table = db.open_table("wiki_chunks")
    data = table.to_arrow()
    total_chunks = len(data)

    texts = data["text"]
    total_chars = sum(len(str(t)) for t in texts)

    current_time = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    print(f"[{current_time}] Total Chunks: {total_chunks}, Total Characters: {total_chars}")
except Exception as e:
    print(f"Error: {e}")
