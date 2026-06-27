class RragBridge < Formula
  desc "Bridge CLI for Remote RAG MCP Server"
  homepage "https://github.com/ryodocx/remote-rag"
  version "1.0.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/ryodocx/remote-rag/releases/download/v1.0.0/rrag-bridge-darwin-arm64.tar.gz"
      sha256 "REPLACE_ME_MAC_ARM64_SHA"
    end
  end

  on_linux do
    if Hardware::CPU.intel?
      url "https://github.com/ryodocx/remote-rag/releases/download/v1.0.0/rrag-bridge-linux-amd64.tar.gz"
      sha256 "REPLACE_ME_LINUX_AMD64_SHA"
    end
  end

  def install
    bin.install "rrag-bridge"
  end
end
