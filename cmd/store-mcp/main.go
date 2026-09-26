// Command store-mcp is an MCP server exposing a store's product catalog
// to AI agents. It speaks MCP over stdio, the standard transport for
// locally launched servers.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"github.com/Raigiku/mcp-sample/internal/catalog"
	"github.com/Raigiku/mcp-sample/internal/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	var (
		catalogsDir = flag.String("catalogs", "catalogs", "directory containing per-store catalog JSON files")
		storeID     = flag.String("store", "demo-store", "store id; loads <catalogs>/<store>.json")
		httpAddr    = flag.String("http", "", "if set, serve MCP over streamable HTTP at this address (e.g. :8080) instead of stdio")
	)
	flag.Parse()

	// Log to stderr: stdout carries the MCP protocol when running over stdio.
	log.SetPrefix("store-mcp: ")

	c, err := catalog.LoadDir(*catalogsDir, *storeID)
	if err != nil {
		log.Fatalf("loading catalog: %v", err)
	}
	log.Printf("store %q: loaded %d products", c.StoreID, len(c.Products))

	srv := server.New(c)
	if *httpAddr != "" {
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv },
			&mcp.StreamableHTTPOptions{Stateless: true})
		log.Printf("listening for MCP over HTTP on %s", *httpAddr)
		log.Fatalf("http server error: %v", http.ListenAndServe(*httpAddr, handler))
	}

	if err := srv.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
