\"\"\"
トークン数を計算するためのユーティリティモジュール。
LLMのコンテキストウィンドウ（入力文字数の上限）を超えないように、
検索結果のテキスト量を事前に計測・制御するために使用します。
\"\"\"
import logging
import tiktoken

logger = logging.getLogger(__name__)

def count_tokens(text: str, encoding_name: str = "cl100k_base") -> int:
    """
    指定されたエンコーディングモデルを使用してテキストのトークン数を高速に計算します。
    
    Args:
        text (str): トークン数を計算したい対象のテキスト。
        encoding_name (str): エンコーディング手法の名前。
                             デフォルトはOpenAIの最新モデルで標準的な 'cl100k_base'。
                             
    Returns:
        int: 計算されたトークン数。
    """
    try:
        encoding = tiktoken.get_encoding(encoding_name)
    except ValueError:
        logger.warning(f"Encoding '{encoding_name}' not found. Falling back to 'cl100k_base'.")
        encoding = tiktoken.get_encoding("cl100k_base")
        
    return len(encoding.encode(text))

