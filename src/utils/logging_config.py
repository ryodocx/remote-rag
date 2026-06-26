"""
ロギング設定のユーティリティモジュール。
アプリケーション全体で統一されたログ出力フォーマットを提供します。
各モジュールでは logging.getLogger(__name__) のみを使用し、
エントリーポイントで setup_logging() を呼び出してください。
"""
import logging


def setup_logging(level: int = logging.INFO) -> None:
    """
    アプリケーション全体のロギングを設定します。
    この関数はエントリーポイント（server.py, search_cli.py 等）で一度だけ呼び出してください。
    
    Args:
        level (int): ログレベル（デフォルト: logging.INFO）。
    """
    logging.basicConfig(
        level=level,
        format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    )
