package tools

import (
	"context"

	"github.com/Raigiku/mcp-sample/internal/catalog"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SearchInput is the tool's input. Query is accepted (AI clients habitually
// send it) but intentionally ignored for now: the catalog is small enough to
// return whole, and the customer's AI does its own filtering.
type SearchInput struct {
	Query string `json:"query,omitempty" jsonschema:"optional free-text search; currently ignored, the full catalog is returned"`
}

// SearchOutput returns the entire catalog as clean structured data.
type SearchOutput struct {
	Store         string            `json:"store" jsonschema:"store display name"`
	TotalProducts int               `json:"total_products" jsonschema:"number of products in the catalog"`
	Products      []catalog.Product `json:"products" jsonschema:"all products with full details"`
}

// SearchProductsHandler returns a handler serving the full catalog.
// Data is captured once at startup; the handler is a pure read.
func SearchProductsHandler(c *catalog.Catalog) mcp.ToolHandlerFor[SearchInput, SearchOutput] {
	out := SearchOutput{
		Store:         c.StoreName,
		TotalProducts: len(c.Products),
		Products:      c.Products,
	}
	return func(ctx context.Context, req *mcp.CallToolRequest, args SearchInput) (*mcp.CallToolResult, SearchOutput, error) {
		return nil, out, nil
	}
}
