package main

import (
	"os"
	"testing"
)

func TestParseConfig(t *testing.T) {
	// Backup env
	origRemote := os.Getenv("MCP_REMOTE_URL")
	origUseID := os.Getenv("USE_ID_TOKEN")
	defer func() {
		os.Setenv("MCP_REMOTE_URL", origRemote)
		os.Setenv("USE_ID_TOKEN", origUseID)
	}()

	os.Setenv("MCP_REMOTE_URL", "http://example.com/mcp")
	os.Setenv("USE_ID_TOKEN", "true")

	config := ParseConfig()

	if config.RemoteURL != "http://example.com/mcp" {
		t.Errorf("Expected RemoteURL http://example.com/mcp, got %s", config.RemoteURL)
	}
	if !config.UseIDToken {
		t.Errorf("Expected UseIDToken to be true")
	}
}
