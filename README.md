<div align="center">
  <h1>🚀 RRAG (Remote RAG) MCP Server & Auth Proxy</h1>
  <p><strong>A remote MCP server with an integrated OAuth 2.0 authentication proxy and hybrid RAG backend.</strong></p>

  <p>
    <b>🛡️ OAuth 2.0 & JWKS Auth</b> &nbsp;•&nbsp; 
    <b>🎯 Hybrid RAG Search</b> &nbsp;•&nbsp; 
    <b>🔌 MCP Protocol Support</b>
  </p>

  [![Docker](https://img.shields.io/badge/docker-%230db7ed.svg?style=flat&logo=docker&logoColor=white)](#)
  [![Python](https://img.shields.io/badge/python-3.14+-blue.svg)](#)
  [![Go](https://img.shields.io/badge/go-1.26+-00ADD8.svg)](#)
  [![MCP](https://img.shields.io/badge/Model_Context_Protocol-Enabled-green.svg)](https://modelcontextprotocol.io/)
  [![License](https://img.shields.io/badge/license-MIT-green.svg)](#)
</div>

<p align="center">
  <em><a href="README_ja.md">日本語の README はこちら (Japanese README)</a></em>
</p>

<br />

An enterprise-ready platform providing a highly accurate **RAG (Retrieval-Augmented Generation) engine** to search internal knowledge bases (e.g., wikis, confidential documents), delivered to AI agents like Cursor or Claude Desktop via the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/).

More than just a search server, RRAG natively integrates a **generic OAuth 2.0 / OIDC authentication and proxy mechanism**, enabling secure remote access over the network.

### 🌟 Key Architecture Concepts
1. **Standardized Identity Management**: Replaces static API keys with standard OAuth 2.0 / OIDC integrations (e.g., Okta, Auth0, GitLab) for dynamic and secure access control.
2. **Hybrid Search Pipeline**: Combines Vector search, Full-Text Search (FTS), and a Cross-Encoder (Reranker) to improve retrieval precision.
3. **Transparent Client Bridge**: A local daemon that handles token management and proxies standard I/O (stdio) to Server-Sent Events (SSE).

---

## 📖 Table of Contents

- [Motivation](#motivation)
- [Features](#features)
- [Use Case & Demo](#use-case--demo)
- [Architecture Overview](#architecture-overview)
- [Quick Start: Deployment to Integration](#quick-start-deployment-to-integration)
- [No-Auth Mode (Local Testing)](#no-auth-mode-local-testing)
- [Directory Structure](#directory-structure)
- [Documentation](#documentation)
- [FAQ](#faq)
- [Contributing](#contributing)

---

## 🤔 Motivation

Integrating the **Model Context Protocol (MCP)** into enterprise environments typically presents two main technical challenges:

1. **Secure Remote Access**: Connecting to remote MCP servers often relies on distributing static API keys to individual clients. This approach increases the risk of credential leakage and introduces significant management overhead (e.g., key rotation).
2. **Retrieval Precision**: Standard vector searches may struggle to find exact matches or handle complex queries over large internal document bases without a well-tuned RAG pipeline.

**RRAG MCP Server & Auth Proxy** addresses these issues by acting as an authentication proxy using standard **OAuth 2.0 / OIDC** and providing a pre-configured hybrid search backend, without requiring complex authentication logic inside the AI agent.

---

## ✨ Features

*   **🛡️ OAuth 2.0 Auth Proxy**
    Caddy + a custom Auth Helper validate OAuth 2.0 tokens right before MCP communication. It supports both **Token Introspection (RFC 7662)** and **Local JWT Validation via JWKS**.
*   **👤 Attribute-based Access Control (ABAC)**
    Configure fine-grained filtering based on IdP claims. Validate `iss`, `aud`, `client_id`, and `scopes` (strict AND conditions), along with user attributes like `email` domains, exact emails, or `groups` (AND conditions).
*   **💻 Client Bridge**
    Automatically integrates with macOS Keychain or Windows Credential Manager. By acquiring/refreshing tokens via browser (PKCE flow), it bridges the AI agent's standard I/O (`stdio`) to the server's `SSE`.
*   **🔍 Hybrid Search Backend (Vector + FTS)**
    Leveraging [LanceDB](https://lancedb.github.io/lancedb/), it fuses vector search and keyword search. It then applies a CrossEncoder (Reranker) to extract relevant chunks.
*   **🧩 Configurable Models**
    Swap Embedding and Reranker models via environment variables. Defaults to `intfloat/multilingual-e5-base`.

---

## 💡 Use Case & Demo

Once configured, you can seamlessly reference internal data from Claude Desktop or Cursor.

> **👤 User:**  
> "Search and tell me about the history and current status of our internal AI project."
> 
> **🤖 AI Agent (Claude):**  
> *(Automatically calls RRAG's MCP tool `search_wiki`)*  
> "According to the search results, the internal AI project started as the third AI boom with the emergence of deep learning in the 2000s. The latest meeting notes (Project X) indicate the current status is..."

---

## 🧩 Architecture Overview

The local Bridge CLI automates token acquisition, while the Caddy + Auth Helper on the server acts as an **Authentication Proxy**. This allows the MCP server to remain free of authentication logic, focusing entirely on RAG searches.

(For detailed sequence diagrams, see the **[Architecture Document](docs/ARCHITECTURE.md)**)

```mermaid
graph LR
    %% Client Layer
    Agent[AI Agent<br/>Cursor, Claude etc.] -->|1. stdio connection| Bridge[Bridge CLI<br/>Auto Token Refresh]
    
    %% Network Proxy Layer
    Bridge ==>|2. HTTPS<br/>w/ Bearer Token| Proxy[Caddy + Auth Helper<br/>🔒 Auth Proxy]
    
    %% External IdP
    IdP((IdP<br/>Okta, Auth0 etc.))
    Proxy -.->|3. Token Validation<br/>Introspection or JWKS| IdP
    
    %% Application Layer (Protected)
    Proxy -->|4. Pass if valid| MCPServer[MCP Server<br/>RAG Engine]
    MCPServer <--> LanceDB[(LanceDB)]

    %% Styles
    style Proxy fill:#ffe6e6,stroke:#ff4d4d,stroke-width:3px
    style Bridge fill:#e6f3ff,stroke:#4da6ff,stroke-width:2px
    style MCPServer fill:#f9f9f9,stroke:#cccccc,stroke-dasharray: 5 5
```

---

## 🚀 Quick Start: Deployment to Integration

Prerequisites: Docker, Docker Compose (Server side), and Go (for Client build).

### 1. Server Environment Setup & Startup
First, launch the server (RAG engine and proxy).

```bash
cd deploy

# 1. Set Auth environment variables (IdP info, JWKS URL, etc.)
# * For a no-auth testing environment, create an empty .env file.
touch .env

# 2. Build and start servers
docker compose up -d
```

### 2. Ingest Sample Data
Run a script inside the `rrag-server` container to import sample data into the RAG database.

```bash
# Fetch 20 Wikipedia articles and save to DB
docker exec -it rrag-server python scripts/ingest_cli.py wiki --count 20
```
*(On first run, AI models are automatically downloaded and cached in a Docker volume)*

### 3. Client (Bridge) Installation
Prepare the Bridge CLI on the local PC running the AI agent.

**macOS / Linux (Homebrew):**
```bash
brew tap ryodocx/remote-rag
brew install rrag-bridge
```

**Windows (Scoop):**
```powershell
scoop bucket add rrag https://github.com/ryodocx/remote-rag.git
scoop install rrag-bridge
```

**Manual Download (Windows, etc.):**
1. Download the archive for your environment (e.g., `rrag-bridge-windows-amd64.zip`) from the [GitHub Releases](https://github.com/ryodocx/remote-rag/releases) page.
2. Extract the ZIP file to any folder (e.g., `C:\tools\rrag-bridge`).
3. Add the extracted folder to your system's `PATH` environment variable.
4. Restart your terminal and verify the `rrag-bridge` command works.

**Install via `go install` (All OS):**
```bash
go install github.com/ryodocx/remote-rag/client/bridge@latest
```

### 4. Integration with Claude Desktop / Cursor
Register the server in your AI agent's MCP configuration file (e.g., `claude_desktop_config.json` for Claude Desktop).

```json
{
  "mcpServers": {
    "rrag": {
      "command": "/absolute/path/to/rrag-bridge",
      "args": ["--url", "https://<your-deployed-domain>/v1/mcp/sse"]
    }
  }
}
```

Setup complete!

---

## 🔓 No-Auth Mode (Local Testing)

If you want to easily test in a secure internal network without OAuth, you can bypass authentication.

### Pattern 1: Mock Authentication at the Proxy
Leave the auth variables in `deploy/.env` **empty**. The `auth-helper` will consider any Bearer token valid.

```env
OAUTH_INTROSPECT_URL=
OAUTH_CLIENT_ID=
OAUTH_CLIENT_SECRET=
OAUTH_JWKS_URL=
```
*(The Bridge CLI client requests will pass through even with a dummy token)*

### Pattern 2: Call the RAG Engine Directly (Easiest)
If you don't need network access and just want to search local data, run the Python RAG engine directly via `stdio`.

**Claude Desktop / Cursor Config:**
```json
{
  "mcpServers": {
    "rrag-local": {
      "command": "python",
      "args": [
        "/absolute/path/to/server/core/src/mcp_server/server.py",
        "--transport",
        "stdio"
      ]
    }
  }
}
```
*(Requires Python 3.14+ and packages from `server/core/requirements.txt`)*

---

## 📁 Directory Structure

```text
.
├── client/          # Go-based Bridge CLI called by agents
├── server/          # Server-side logic
│   ├── core/        # Python RAG engine & MCP Server (FastMCP)
│   └── auth-helper/ # Go Auth proxy (Token validation / JWKS / Caching)
├── deploy/          # Deployment configs (Docker Compose, Caddyfile)
└── docs/            # Documentation
```

---

## 📚 Documentation

| Document | Content |
| :--- | :--- |
| **[Architecture](docs/ARCHITECTURE.md)** | System components, proxy architecture, and PKCE auth sequence diagrams. |
| **[Operations Manual](docs/OPERATIONS.md)** | Server environment variables, deployment steps, and troubleshooting. |
| **[Okta Setup Guide](docs/OKTA_SETUP.md)** | App registration and custom claims config for Okta (OAuth 2.0 / OIDC). |
| **[GitLab Setup Guide](docs/GITLAB_SETUP.md)** | App registration and claim details for GitLab (gitlab.com / Self-hosted). |
| **[ChatGPT Setup Guide](docs/CHATGPT_CUSTOM_GPTS_SETUP.md)** | App registration and Custom GPTs (Actions) configuration. |
| **[Copilot Studio Setup Guide](docs/COPILOT_STUDIO_SETUP.md)** | Custom Connector and OAuth 2.0 configuration for Microsoft Copilot Studio. |
| **[Web App / REST API Integration Guide](docs/WEB_APP_INTEGRATION.md)** | How to integrate REST API with web apps like Dify, Zapier, and Retool. |
| **[AI Models Guide](docs/MODELS.md)** | How to swap Embedding/Reranker models, with comparisons for quality, speed, and memory usage. |
| **[Development Guide](docs/DEVELOPMENT.md)** | Build instructions, local environments, and testing AI models. |

---

## ❓ FAQ

**Q. Is this tied to a specific IdP (Okta, Auth0, Entra ID)?**  
A. No. It supports any standard OAuth 2.0 / OIDC compliant authorization server via Token Introspection (RFC 7662) or JWKS local validation.

**Q. Can I change the AI models?**  
A. Yes. You can easily switch out models via environment variables—from ultra-lightweight to high-end models like "BGE-M3". See [MODELS.md](docs/MODELS.md).

**Q. Does the Bridge CLI support Windows?**  
A. Yes. Since it's written in Go, it's cross-compiled for macOS, Linux, and Windows. Credentials are safely stored in each OS's native secret manager.

---

## 📄 Contributing

*   **Issues and Pull Requests are highly welcomed!**
*   We welcome contributions such as new AI model benchmarks, client enhancements, and bug fixes.
