import uuid
from langchain_text_splitters import RecursiveCharacterTextSplitter, MarkdownHeaderTextSplitter

def chunk_markdown(text: str, page_id: str, title: str, url: str) -> list[dict]:
    """
    Markdownテキストを階層（見出し）を意識してチャンク化し、WikiChunkオブジェクトのリストを返します。
    抽出した見出し階層情報をチャンクの先頭に付与することで、LLMへの文脈保持を強化します。
    """
    # 1. 見出しベースの分割
    headers_to_split_on = [
        ("#", "H1"),
        ("##", "H2"),
        ("###", "H3"),
        ("####", "H4"),
    ]
    md_splitter = MarkdownHeaderTextSplitter(headers_to_split_on=headers_to_split_on)
    header_docs = md_splitter.split_text(text)
    
    # 2. 文字数ベースの分割 (見出し内が大きすぎる場合のフォールバック)
    chunk_size = 500
    chunk_overlap = 50
    char_splitter = RecursiveCharacterTextSplitter(
        chunk_size=chunk_size,
        chunk_overlap=chunk_overlap,
        separators=["\n\n", "\n", " ", ""],
        keep_separator=True
    )
    
    final_docs = char_splitter.split_documents(header_docs)
    
    wiki_chunks = []
    for doc in final_docs:
        chunk_id = str(uuid.uuid4())
        
        # 見出しメタデータを階層化してコンテキスト文字列を作成
        context_parts = []
        for h_level in ["H1", "H2", "H3", "H4"]:
            if h_level in doc.metadata:
                context_parts.append(doc.metadata[h_level])
                
        context_str = " > ".join(context_parts)
        
        # 本文の先頭にコンテキストを埋め込む
        if context_str:
            enriched_text = f"[Context: {context_str}]\n\n{doc.page_content}"
        else:
            enriched_text = doc.page_content
            
        wiki_chunk = {
            "chunk_id": chunk_id,
            "page_id": page_id,
            "text": enriched_text,
            "title": title,
            "url": url
        }
        wiki_chunks.append(wiki_chunk)
        
    return wiki_chunks
