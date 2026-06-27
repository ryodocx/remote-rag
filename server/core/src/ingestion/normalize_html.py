import re

def normalize_to_markdown(html_content: str) -> str:
    """
    HTMLコンテンツをMarkdownに変換します。
    実運用では markdownify などのライブラリを使用しますが、
    ここでは簡易的なプレースホルダー実装または markdownify のラップを提供します。
    """
    try:
        from markdownify import markdownify as md
        return md(html_content, heading_style="ATX")
    except ImportError:
        # フォールバック: markdownify がない場合は簡易的にタグを除去
        print("Warning: markdownify not found. Using simple fallback HTML tag removal.")
        text = re.sub(r'<h[1-6][^>]*>(.*?)</h[1-6]>', r'\n## \1\n', html_content, flags=re.IGNORECASE)
        text = re.sub(r'<[^>]+>', '', text)
        return text
