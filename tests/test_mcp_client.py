import asyncio
import os
import sys
import argparse
from mcp import ClientSession, StdioServerParameters
from mcp.client.stdio import stdio_client

async def run(query: str):
    # 仮想環境のPythonインタープリタのパスを取得
    venv_python = os.path.join(os.path.dirname(os.path.dirname(__file__)), ".venv", "Scripts", "python.exe")
    
    # サーバー起動パラメータ (stdioモードで mcp_server/server.py を実行)
    server_params = StdioServerParameters(
        command=venv_python,
        args=["-m", "src.mcp_server.server"]
    )
    
    async with stdio_client(server_params) as (read, write):
        async with ClientSession(read, write) as session:
            # MCPサーバーの初期化
            await session.initialize()
            
            print(f"\n=== 検索クエリ: '{query}' ===\n")
            
            # ツールを実行
            result = await session.call_tool(
                "search_wiki", 
                arguments={"query": query, "limit": 3}
            )
            
            # 結果を出力
            if result.content:
                print(result.content[0].text)
            else:
                print("関連する情報が見つかりませんでした。")

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="MCPサーバーの検索機能をテストするCLI")
    parser.add_argument("query", type=str, nargs="?", default="電源", help="検索したいキーワード")
    args = parser.parse_args()
    
    # asyncio の実行
    asyncio.run(run(args.query))
