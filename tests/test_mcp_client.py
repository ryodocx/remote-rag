import pytest
import asyncio
import os
from mcp import ClientSession, StdioServerParameters
from mcp.client.stdio import stdio_client

@pytest.mark.asyncio
async def test_mcp_client_search():
    """MCPクライアントの接続と検索機能をテストする統合テスト"""
    venv_python = os.path.join(os.path.dirname(os.path.dirname(__file__)), ".venv", "Scripts", "python.exe")
    
    if not os.path.exists(venv_python):
        pytest.skip("Venv python not found. Skipping stdio client integration test.")
        
    server_params = StdioServerParameters(
        command=venv_python,
        args=["-m", "src.mcp_server.server"]
    )
    
    async with stdio_client(server_params) as (read, write):
        async with ClientSession(read, write) as session:
            await session.initialize()
            
            result = await session.call_tool(
                "search_wiki", 
                arguments={"query": "電源", "limit": 1}
            )
            
            assert result.content is not None
            assert len(result.content) > 0
            assert isinstance(result.content[0].text, str)
