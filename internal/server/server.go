// Package server provides the shared MCP server bootstrap.
// Each deployable in cmd/ wires its own set of tools into this.
package server

import (
	"github.com/Raigiku/mcp-sample/internal/catalog"
	"github.com/Raigiku/mcp-sample/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName    = "ecommerce-mcp"
	serverVersion = "v0.2.0"
)

// New creates an MCP server for the given store catalog and registers its tools.
func New(c *catalog.Catalog) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: serverName, Version: serverVersion}, nil)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "search_products",
		Description: "Returns the store's full product catalog with all details (price, sizes, stock, images). " +
			"Filtering and ranking are done client-side by the AI agent.",
	}, tools.SearchProductsHandler(c))

	return srv
}