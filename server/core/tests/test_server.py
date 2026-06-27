import pytest
import sys
import os
from unittest.mock import patch, MagicMock

# src モジュールをインポートできるようにパスを追加
sys.path.append(os.path.dirname(os.path.dirname(__file__)))

from src.mcp_server.server import search_wiki

def test_search_wiki_returns_results():
    """実際の検索処理を通す統合テスト"""
    result = search_wiki("ラーメン", limit=2)
    assert isinstance(result, str)
    if "No relevant information" not in result:
        assert len(result) > 0
        assert "Content:" in result

def test_search_wiki_no_results():
    """モックを用いた単体テスト（結果なしのケース）"""
    with patch('src.mcp_server.server._get_searcher') as mock_get_searcher:
        mock_searcher = MagicMock()
        mock_searcher.search.return_value = []
        mock_get_searcher.return_value = mock_searcher

        result = search_wiki("存在しないクエリ12345", limit=1)
        assert result == "No relevant information found in the Wiki."
