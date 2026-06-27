"""
RRAG Server 統合エントリーポイント。

MCP Server (SSE)、Ingest API、Search REST API の3つのサービスを
子プロセスとして同時に起動し、1つのコンテナで運用します。

いずれかの子プロセスが異常終了した場合、残りのプロセスも終了させ、
コンテナ全体を停止させます。
"""
import os
import sys
import signal
import subprocess
import logging

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)
logger = logging.getLogger("rrag-entrypoint")


def main():
    """3つのサービスプロセスを起動し、いずれかの終了を監視する。"""

    # 環境変数からポート番号を取得（テスト時などに上書き可能）
    mcp_port = os.environ.get("MCP_PORT", "8000")
    ingest_port = os.environ.get("INGEST_PORT", "8001")
    search_port = os.environ.get("SEARCH_PORT", "8010")

    services = [
        {
            "name": "MCP Server (SSE)",
            "cmd": [
                sys.executable, "-m", "src.mcp_server.server",
                "--transport", "sse",
                "--host", "0.0.0.0",
                "--port", mcp_port,
            ],
        },
        {
            "name": "Ingest API",
            "cmd": [
                sys.executable, "-m", "uvicorn",
                "src.api.ingest_api:app",
                "--host", "0.0.0.0",
                "--port", ingest_port,
            ],
        },
        {
            "name": "Search REST API",
            "cmd": [
                sys.executable, "-m", "uvicorn",
                "src.api.search_api:app",
                "--host", "0.0.0.0",
                "--port", search_port,
            ],
        },
    ]

    processes: list[tuple[str, subprocess.Popen]] = []

    def shutdown(signum=None, frame=None):
        """全子プロセスを終了させる。"""
        logger.info("Shutting down all services...")
        for name, proc in processes:
            if proc.poll() is None:
                logger.info(f"  Terminating {name} (PID {proc.pid})")
                proc.terminate()
        for name, proc in processes:
            try:
                proc.wait(timeout=10)
            except subprocess.TimeoutExpired:
                logger.warning(f"  Force killing {name} (PID {proc.pid})")
                proc.kill()

    # SIGTERM / SIGINT を受け取ったら全プロセスを停止
    signal.signal(signal.SIGTERM, shutdown)
    signal.signal(signal.SIGINT, shutdown)

    # 各サービスを子プロセスとして起動
    for svc in services:
        logger.info(f"Starting {svc['name']}: {' '.join(svc['cmd'])}")
        proc = subprocess.Popen(
            svc["cmd"],
            stdout=sys.stdout,
            stderr=sys.stderr,
        )
        processes.append((svc["name"], proc))

    logger.info(f"All {len(processes)} services started.")

    # いずれかのプロセスが終了するまで待機
    try:
        while True:
            for name, proc in processes:
                ret = proc.poll()
                if ret is not None:
                    logger.error(
                        f"Service '{name}' exited with code {ret}. "
                        "Shutting down remaining services."
                    )
                    shutdown()
                    sys.exit(ret if ret != 0 else 1)
            # ビジーウェイトを避けるため短いスリープ
            import time
            time.sleep(1)
    except KeyboardInterrupt:
        shutdown()
        sys.exit(0)


if __name__ == "__main__":
    main()
