import pytest
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '../src')))
from ingestion.chunker import chunk_markdown

def test_chunk_markdown_no_headers():
    text = "This is a simple text without any headers."
    page_id = "test_page_1"
    title = "Test Page 1"
    url = "http://example.com/1"

    chunks = chunk_markdown(text, page_id, title, url)

    assert len(chunks) == 1
    chunk = chunks[0]
    assert chunk["page_id"] == page_id
    assert chunk["title"] == title
    assert chunk["url"] == url
    assert chunk["text"] == text
    assert "chunk_id" in chunk

def test_chunk_markdown_header_hierarchy():
    text = """# Header 1
Content 1
## Header 2
Content 2
### Header 3
Content 3
#### Header 4
Content 4
"""
    page_id = "test_page_2"
    title = "Test Page 2"
    url = "http://example.com/2"

    chunks = chunk_markdown(text, page_id, title, url)

    # We should have 4 chunks, one for each header block
    assert len(chunks) == 4

    assert chunks[0]["text"] == "[Context: Header 1]\n\nContent 1"
    assert chunks[1]["text"] == "[Context: Header 1 > Header 2]\n\nContent 2"
    assert chunks[2]["text"] == "[Context: Header 1 > Header 2 > Header 3]\n\nContent 3"
    assert chunks[3]["text"] == "[Context: Header 1 > Header 2 > Header 3 > Header 4]\n\nContent 4"

def test_chunk_markdown_length_fallback():
    # Create a text with a single header but content > 500 chars
    long_content = "A" * 600
    text = f"# Header 1\n{long_content}"

    page_id = "test_page_3"
    title = "Test Page 3"
    url = "http://example.com/3"

    chunks = chunk_markdown(text, page_id, title, url)

    # It should be split into multiple chunks because length > 500
    assert len(chunks) > 1

    # Check that context is preserved in all chunks
    for chunk in chunks:
        assert chunk["text"].startswith("[Context: Header 1]\n\n")

def test_chunk_markdown_empty_text():
    text = ""
    page_id = "test_page_4"
    title = "Test Page 4"
    url = "http://example.com/4"

    chunks = chunk_markdown(text, page_id, title, url)

    assert len(chunks) == 0
