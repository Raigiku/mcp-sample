package server_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
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

func TestRenderWidgetReturnsResourceURI(t *testing.T) {
	sess := connect(t)

	res, err := sess.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "render_products_widget",
		Arguments: map[string]any{"product_ids": []string{"SKU-001", "SKU-006"}},
	})
	if err != nil {
		t.Fatalf("call render_products_widget: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %v", res)
	}

	// Meta is map-shaped over the wire; check the UI pointer survives marshaling.
	meta, err := json.Marshal(res.Meta)
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(meta, &m); err != nil {
		t.Fatalf("unmarshal meta: %v", err)
	}
	ui, ok := m["ui"].(map[string]any)
	if !ok {
		t.Fatalf("meta has no ui object: %s", meta)
	}
	if got := ui["resourceUri"]; got != tools.WidgetResourceURI {
		t.Errorf("resourceUri = %v, want %q", got, tools.WidgetResourceURI)
	}

	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	var out tools.RenderOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
	if out.Store != "Demo Outdoor Gear" {
		t.Errorf("store = %q, want %q", out.Store, "Demo Outdoor Gear")
	}
	if len(out.Products) != 2 {
		t.Fatalf("got %d products, want 2", len(out.Products))
	}
	if out.Products[0].ID != "SKU-001" || out.Products[1].ID != "SKU-006" {
		t.Errorf("product ids = %s, %s; want SKU-001, SKU-006 (requested order)",
			out.Products[0].ID, out.Products[1].ID)
	}
}

func TestRenderWidgetUnknownID(t *testing.T) {
	sess := connect(t)

	res, err := sess.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "render_products_widget",
		Arguments: map[string]any{"product_ids": []string{"SKU-001", "NOPE"}},
	})
	// The SDK turns handler errors into error results carrying the message.
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("unknown product id did not produce an error result")
	}
	if len(res.Content) == 0 {
		t.Fatal("error result has no message for the model")
	}
	if text, ok := res.Content[0].(*mcp.TextContent); !ok || !strings.Contains(text.Text, "unknown product id(s)") {
		t.Errorf("error content %v does not name the unknown ids", res.Content[0])
	}
}

func TestWidgetResourceReadable(t *testing.T) {
	sess := connect(t)

	rr, err := sess.ReadResource(context.Background(), &mcp.ReadResourceParams{
		URI: tools.WidgetResourceURI,
	})
	if err != nil {
		t.Fatalf("read resource: %v", err)
	}
	if len(rr.Contents) != 1 {
		t.Fatalf("got %d contents, want 1", len(rr.Contents))
	}
	c := rr.Contents[0]
	if c.URI != tools.WidgetResourceURI {
		t.Errorf("content uri = %q, want %q", c.URI, tools.WidgetResourceURI)
	}
	if c.MIMEType != tools.WidgetMIMEType {
		t.Errorf("mimeType = %q, want %q", c.MIMEType, tools.WidgetMIMEType)
	}
	if !strings.Contains(c.Text, `<div id="root">`) {
		t.Error("widget HTML missing <div id=\"root\">")
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