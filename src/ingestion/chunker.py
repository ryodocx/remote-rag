import uuid
from langchain_text_splitters import RecursiveCharacterTextSplitter
from src.database.schema import WikiChunk

def chunk_markdown(text: str, page_id: str, title: str, url: str) -> list[dict]:
    """
    Markdownテキストをチャンク化し、WikiChunkオブジェクトのリストを返します。
    """
    # チャンクサイズとオーバーラップを定義
    chunk_size = 500
    chunk_overlap = 50
    
    # langchain の RecursiveCharacterTextSplitter を使用
    # 見出し単位で分割されやすいようにセパレータを設定し、見出しをチャンクに残す
    splitter = RecursiveCharacterTextSplitter(
        chunk_size=chunk_size,
        chunk_overlap=chunk_overlap,
        separators=["\n## ", "\n### ", "\n#### ", "\n", " ", ""],
        keep_separator=True
    )
    
    chunks = splitter.split_text(text)
    
    wiki_chunks = []
    for chunk_text in chunks:
        chunk_id = str(uuid.uuid4())
        
        # 辞書として作成し、後続の table.add(data) 時にLanceDB側でベクトル化とスキーマ検証を行わせます。
        wiki_chunk = {
            "chunk_id": chunk_id,
            "page_id": page_id,
            "text": chunk_text,
            "title": title,
            "url": url
        }
        wiki_chunks.append(wiki_chunk)
        
    return wiki_chunks
