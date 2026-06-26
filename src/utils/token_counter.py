import tiktoken

def count_tokens(text: str, encoding_name: str = "cl100k_base") -> int:
    """
    指定されたエンコーディングモデルを使用してテキストのトークン数を計算します。
    OpenAI系のモデル（GPT-3.5, GPT-4等）で一般的な cl100k_base をデフォルトとします。
    """
    try:
        encoding = tiktoken.get_encoding(encoding_name)
    except ValueError:
        encoding = tiktoken.get_encoding("cl100k_base")
        
    return len(encoding.encode(text))
