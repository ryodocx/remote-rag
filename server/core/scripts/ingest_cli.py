import argparse
import sys
import os

# srcモジュールを参照できるようにパスを追加
sys.path.append(os.path.dirname(os.path.dirname(__file__)))

from src.utils.logging_config import setup_logging
from src.ingestion.wikipedia_ingest import ingest_wikipedia_pages
from scripts.pipeline import run_dummy_ingestion

def main():
    # Windows環境等での文字化け対策
    if sys.stdout.encoding != 'utf-8':
        sys.stdout.reconfigure(encoding='utf-8')
    
    setup_logging()
    
    parser = argparse.ArgumentParser(description="Ingest data into LanceDB")
    # サブコマンド(wiki, dummy)で処理を分岐させるための設定
    subparsers = parser.add_subparsers(dest="command", help="Ingestion source type")
    
    # --- Wikipediaインジェスト用コマンドの定義 ---
    wiki_parser = subparsers.add_parser("wiki", help="Ingest Wikipedia articles")
    wiki_parser.add_argument("--count", type=int, default=20, help="Number of random articles to ingest")
    wiki_parser.add_argument("--sleep", type=float, default=2.0, help="Seconds to sleep between requests")
    wiki_parser.add_argument("--titles", type=str, nargs="+", help="Specific article titles to ingest (overrides --count)")
    
    # --- ダミーデータインジェスト用コマンドの定義 ---
    subparsers.add_parser("dummy", help="Ingest dummy markdown data from data/dummy")
    
    args = parser.parse_args()
    
    if args.command == "wiki":
        # WikipediaのAPIを叩いて記事を取得し、チャンク分割してLanceDBへ保存
        print("Starting Wikipedia ingestion...")
        ingest_wikipedia_pages(
            count=args.count,
            sleep_sec=args.sleep,
            specific_titles=args.titles
        )
    elif args.command == "dummy":
        # ローカルのマークダウンファイル等のダミーデータを投入
        print("Starting Dummy data ingestion...")
        run_dummy_ingestion()
    else:
        # 引数が不足している場合はヘルプを表示して終了
        parser.print_help()
        sys.exit(1)

if __name__ == "__main__":
    main()
