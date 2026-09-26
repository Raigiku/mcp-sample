// Package server provides the shared MCP server bootstrap.
// Each deployable in cmd/ wires its own set of tools into this.
package server

import (
	"context"

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

	// The widget: an MCP Apps-style UI resource plus the render tool whose
	// result meta points compatible hosts at that resource in an iframe.
	widgetResource, widgetHTML := tools.WidgetResource()
	srv.AddResource(widgetResource, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      widgetResource.URI,
				MIMEType: tools.WidgetMIMEType,
				Text:     widgetHTML,
			}},
		}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "render_products_widget",
		Description: "Displays products as cards in the chat UI. " +
			"Call search_products FIRST, then pass the ids of the products to display here.",
	}, tools.RenderProductsHandler(c))

	return srv
}
