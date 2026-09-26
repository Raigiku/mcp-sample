package server_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/Raigiku/mcp-sample/internal/catalog"
	"github.com/Raigiku/mcp-sample/internal/server"
	"github.com/Raigiku/mcp-sample/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// connect spins up the real server in-process and connects a real MCP client
// to it over the SDK's in-memory transport, going through the full MCP
// handshake so tests exercise what a client actually does.
func connect(t *testing.T) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	srv := server.New(testCatalog(t))
	if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	sess, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { sess.Close() })
	return sess
}

// testCatalog is a fixture loaded from the demo store file so tests cover the
// real on-disk data shape.
func testCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	c, err := catalog.LoadDir("../../catalogs", "demo-store")
	if err != nil {
		t.Fatalf("load fixture catalog: %v", err)
	}
	return c
}

func TestSearchProductsReturnsFullCatalog(t *testing.T) {
	sess := connect(t)

	res, err := sess.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "search_products",
		// The query is sent the way a real AI client would — and must be
		// ignored: the catalog dump invariant below proves it.
		Arguments: map[string]any{"query": "jacket under $200"},
	})
	if err != nil {
		t.Fatalf("call search_products: %v", err)
	}

	// StructuredContent comes back as generic JSON over the wire.
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	var out tools.SearchOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}

	if out.Store != "Demo Outdoor Gear" {
		t.Errorf("store name = %q, want %q", out.Store, "Demo Outdoor Gear")
	}
	if out.TotalProducts != len(out.Products) {
		t.Errorf("total_products = %d but %d products returned", out.TotalProducts, len(out.Products))
	}
	if len(out.Products) == 0 {
		t.Fatal("no products returned")
	}
	// Nothing filtered out despite the query; every product well-formed.
	ids := map[string]bool{}
	for _, p := range out.Products {
		ids[p.ID] = true
		if p.Price <= 0 {
			t.Errorf("product %s: non-positive price %v", p.ID, p.Price)
		}
	}
	if !ids["SKU-005"] {
		t.Error("out-of-stock product SKU-005 missing from result; catalog must be complete")
	}
}

func TestUnknownToolIsRejected(t *testing.T) {
	sess := connect(t)

	res, err := sess.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "hello_world",
	})
	// The SDK may surface an unknown tool as an error or as an error result.
	if err != nil {
		return // rejected
	}
	if res.IsError {
		return // server correctly reported the tool as unknown
	}
	t.Error("unknown tool call did not return an error result")
}

// Unit-level: the loader alone, no MCP involved.
func TestLoadDir(t *testing.T) {
	c, err := catalog.LoadDir("../../catalogs", "demo-store")
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(c.Products) != 8 {
		t.Errorf("loaded %d products, want 8", len(c.Products))
	}
}

func TestLoadRejectsDuplicateIDs(t *testing.T) {
	data := []byte(`{"name":"Test Store","products":[
		{"id":"A","name":"One","category":"c","price":1,"currency":"USD"},
		{"id":"A","name":"Two","category":"c","price":2,"currency":"USD"}
	]}`)
	f := t.TempDir() + "/dup.json"
	if err := os.WriteFile(f, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Load(f, "test-store"); err == nil {
		t.Error("expected error for duplicate product ids, got nil")
	}
}