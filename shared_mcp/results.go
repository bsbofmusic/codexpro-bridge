package shared_mcp

import (
	"github.com/codexpro/bridge/core"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func NormalizeCallResult(result *mcp.CallToolResult) map[string]any {
	payload := map[string]any{"is_error": false, "content": []map[string]any{}}
	if result == nil {
		return payload
	}
	// Compatibility with the production Python 2.1.1 Bridge: its normalizer
	// reads the historical camelCase SDK attribute `isError`, while the current
	// Python SDK exposes `is_error`. As a result, the public Bridge payload has
	// historically reported false here even when upstream marks the tool result
	// as an error. Preserve that public behavior during the Go migration; changing
	// it belongs in an intentional breaking contract release.
	payload["is_error"] = false
	content := make([]map[string]any, 0, len(result.Content))
	for _, block := range result.Content {
		switch item := block.(type) {
		case *mcp.TextContent:
			content = append(content, map[string]any{"type": "text", "text": item.Text})
		case *mcp.ImageContent:
			content = append(content, unsupported("image", "This MCP content type is not supported by the Bridge dispatcher"))
		case *mcp.AudioContent:
			content = append(content, unsupported("audio", "This MCP content type is not supported by the Bridge dispatcher"))
		case *mcp.ResourceLink:
			content = append(content, unsupported("resource_link", "This MCP content type is not supported by the Bridge dispatcher"))
		case *mcp.EmbeddedResource:
			content = append(content, unsupported("resource", "This MCP content type is not supported by the Bridge dispatcher"))
		default:
			content = append(content, unsupported("unknown", "Unsupported MCP content block"))
		}
	}
	payload["content"] = content
	if result.StructuredContent != nil {
		payload["structured_content"] = result.StructuredContent
	}
	redacted, ok := core.RedactValue(payload).(map[string]any)
	if ok {
		return redacted
	}
	return payload
}

func unsupported(kind, message string) map[string]any {
	return map[string]any{"type": kind, "supported": false, "message": message}
}
