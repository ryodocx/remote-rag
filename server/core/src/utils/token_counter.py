"""
トークン数を計算するためのユーティリティモジュール。
LLMのコンテキストウィンドウ（入力文字数の上限）を超えないように、
検索結果のテキスト量を事前に計測・制御するために使用します。
"""
import logging
import tiktoken

logger = logging.getLogger(__name__)

# エンコーディングインスタンスのキャッシュ（毎回の再生成を防止）
_encoding_cache: dict[str, tiktoken.Encoding] = {}

def count_tokens(text: str, encoding_name: str = "cl100k_base") -> int:
    """
    指定されたエンコーディングモデルを使用してテキストのトークン数を高速に計算します。
    エンコーディングインスタンスはモジュールレベルでキャッシュされ、再利用されます。

    Args:
        text (str): トークン数を計算したい対象のテキスト。
        encoding_name (str): エンコーディング手法の名前。
                             デフォルトはOpenAIの最新モデルで標準的な 'cl100k_base'。

    Returns:
        int: 計算されたトークン数。
    """
    if encoding_name not in _encoding_cache:
        try:
            _encoding_cache[encoding_name] = tiktoken.get_encoding(encoding_name)
        except ValueError:
            logger.warning(f"Encoding '{encoding_name}' not found. Falling back to 'cl100k_base'.")
            _encoding_cache[encoding_name] = tiktoken.get_encoding("cl100k_base")

    return len(_encoding_cache[encoding_name].encode(text))
