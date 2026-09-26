package tools

import (
	"context"
	"fmt"
	"sort"

	"github.com/Raigiku/mcp-sample/internal/catalog"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// WidgetMIMEType is the MIME type the MCP Apps standard requires for widget
// resources; the "mcp-app" profile tells compatible hosts to render it in an
// iframe rather than show the HTML as text.
const WidgetMIMEType = "text/html;profile=mcp-app"

// WidgetResourceURI is the widget's resource URI. It doubles as the cache key:
// hosts cache the HTML by URI, so a UI change ships as a new versioned URI
// rather than mutating this one.
const WidgetResourceURI = "ui://Raigiku/mcp-sample/products-v3.html"

// WidgetDomain is the HTTPS host serving the widget (the current ngrok tunnel).
// Hosts require it in the resource's _meta so they may frame the resource.
// It changes whenever the tunnel restarts — update it here.
const WidgetDomain = "https://infundibulate-lakiesha-prepigmental.ngrok-free.dev"

// Invocation progress strings the host shows around the tool call (pizzaz
// pattern: "openai/toolInvocation/invoking" while the tool runs, "invoked" when
// its result arrives). Exported so server.go and tests share one copy.
const (
	WidgetInvoking = "Loading products…"
	WidgetInvoked  = "Products ready."
)

// widgetHTML is the widget document: a display-only page that initiates the
// MCP Apps handshake (ui/initialize) on load and then listens for the host's
// ui/notifications/tool-result postMessage to render product cards.
// Vanilla, no build step, no external requests except the product images.
const widgetHTML = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
  :root { color-scheme: light dark; }
  body {
    margin: 0;
    padding: 12px;
    font-family: system-ui, -apple-system, "Segoe UI", sans-serif;
    font-size: 14px;
    line-height: 1.4;
    color: #1f2937;
    background: transparent;
  }
  .card {
    display: flex;
    gap: 12px;
    padding: 12px;
    margin-bottom: 10px;
    border: 1px solid rgba(128, 128, 128, 0.35);
    border-radius: 10px;
  }
  .card img {
    width: 88px;
    height: 88px;
    object-fit: cover;
    border-radius: 8px;
    flex-shrink: 0;
    background: rgba(128, 128, 128, 0.15);
  }
  .card .info { min-width: 0; }
  .card .name { margin: 0 0 2px; font-weight: 600; }
  .card .desc {
    margin: 0 0 6px;
    color: #6b7280;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
  .card .meta {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    color: #6b7280;
  }
  .price { font-weight: 600; color: inherit; }
  .badge {
    padding: 1px 8px;
    border-radius: 999px;
    font-size: 11px;
    font-weight: 600;
  }
  .in { background: rgba(37, 170, 90, 0.18); color: #166534; }
  .out { background: rgba(220, 60, 60, 0.16); color: #991b1b; }
  .card a { margin-left: auto; color: #2563eb; text-decoration: none; font-size: 13px; }
  .card a:hover { text-decoration: underline; }
  .empty { color: #6b7280; font-size: 13px; padding: 8px 4px; }
  @media (prefers-color-scheme: dark) {
    body { color: #e5e7eb; }
    .card .desc, .card .meta, .empty { color: #9ca3af; }
  }
</style>
</head>
<body>
<div id="root"></div>
<script type="module">
(() => {
  "use strict";

  const root = document.getElementById("root");
  if (!root) return;

  const fmt = (price, currency) => {
    try {
      return new Intl.NumberFormat(document.documentElement.lang || undefined, {
        style: "currency",
        currency: currency || "USD"
      }).format(price);
    } catch (e) {
      return currency ? currency + " " + price : String(price);
    }
  };

  const el = (tag, cls, text) => {
    const n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text !== undefined) n.textContent = text; // textContent: untrusted data stays inert
    return n;
  };

  const empty = () => {
    root.replaceChildren(el("p", "empty", "No products to display."));
  };

  const card = (p) => {
    const cardEl = el("div", "card");

    const img = document.createElement("img");
    if (typeof p.image_url === "string" && p.image_url) {
      img.src = p.image_url;
      img.alt = "";
      img.loading = "lazy";
    }
    img.addEventListener("error", () => { img.style.visibility = "hidden"; });
    cardEl.appendChild(img);

    const info = el("div", "info");
    info.appendChild(el("h3", "name", typeof p.name === "string" ? p.name : "Product"));

    const desc = typeof p.description === "string" ? p.description : "";
    if (desc) info.appendChild(el("p", "desc", desc));

    const meta = el("div", "meta");
    const price = typeof p.price === "number" ? p.price : NaN;
    if (!Number.isNaN(price)) {
      meta.appendChild(el("span", "price", fmt(price, p.currency)));
    }
    meta.appendChild(el("span", "badge " + (p.in_stock ? "in" : "out"),
      p.in_stock ? "In stock" : "Out of stock"));
    if (typeof p.category === "string" && p.category) {
      meta.appendChild(el("span", "category", p.category));
    }
    info.appendChild(meta);

    const row = el("div");
    row.className = "meta";
    if (typeof p.product_url === "string" && p.product_url) {
      const a = document.createElement("a");
      a.href = p.product_url;
      a.target = "_blank";
      a.rel = "noopener noreferrer";
      a.textContent = "View";
      row.appendChild(a);
    }
    info.appendChild(row);

    cardEl.appendChild(info);
    return cardEl;
  };

  const render = (products) => {
    root.replaceChildren(...products.map(card));
  };

  // --- MCP Apps messaging layer ---
  // The iframe must initiate the handshake: it sends a ui/initialize JSON-RPC
  // request on load, and only then do compatible hosts deliver tool results.

  const pending = new Map(); // json-rpc id -> {resolve, reject}
  let nextId = 1;

  const request = (method, params) => {
    const id = nextId++;
    try {
      window.parent.postMessage({ jsonrpc: "2.0", id, method, params }, "*");
    } catch (e) {
      return Promise.reject(e);
    }
    return new Promise((resolve, reject) => {
      pending.set(id, { resolve, reject });
    });
  };

  // Renders product cards if sc carries a non-empty products array; does
  // nothing otherwise (never clears an already-rendered view). Returns true
  // when something was rendered, false otherwise.
  const tryRenderProducts = (sc) => {
    const products = sc && Array.isArray(sc.products)
      ? sc.products.filter((p) => p && typeof p === "object")
      : [];
    if (products.length === 0) return false;
    render(products);
    return true;
  };

  window.addEventListener("message", (event) => {
    try {
      // Only trust messages coming from the hosting frame.
      if (event.source !== window.parent) return;
      const msg = event.data;
      if (!msg || typeof msg !== "object" || msg.jsonrpc !== "2.0") return;

      // Responses to our own requests (e.g. the ui/initialize result).
      if (msg.id !== undefined && pending.has(msg.id)) {
        const p = pending.get(msg.id);
        pending.delete(msg.id);
        if (msg.error) p.reject(msg.error);
        else p.resolve(msg.result);
        return;
      }

      if (msg.method === "ui/notifications/tool-result") {
        const params = msg.params || {};
        const sc = params.structuredContent
          || (params.toolResult && params.toolResult.structuredContent);
        const products = sc && Array.isArray(sc.products)
          ? sc.products.filter((p) => p && typeof p === "object")
          : [];
        if (products.length === 0) {
          empty();
          return;
        }
        render(products);
      }
      // ui/notifications/tool-input and anything else: ignore.
    } catch (e) {
      // Never throw: a bad message must not break the widget.
      try { empty(); } catch (ignored) {}
    }
  });

  // Initiate the handshake on load. Some hosts may also replay the triggering
  // tool call's result inside the initialize response, so check for products
  // there too (defensively; an empty result must not clear the view).
  request("ui/initialize", { appCapabilities: { availableDisplayModes: ["inline"] } })
    .then((result) => {
      if (!result || typeof result !== "object") return;
      tryRenderProducts(result.structuredContent
        || (result.toolResult && result.toolResult.structuredContent));
    })
    .catch(() => {}); // a failed handshake must not break the widget

  // --- ChatGPT compatibility path ---
  // ChatGPT exposes the tool result via window.openai globals rather than the
  // MCP Apps notifications; the globals appear asynchronously after load, so
  // poll briefly (pattern from OpenAI's own useOpenAiGlobal hook).
  const readOpenAiOutput = () => {
    try {
      const out = window.openai && window.openai.toolOutput;
      if (!out) return undefined;
      // toolOutput may be the CallToolResult ({structuredContent: ...}) or the
      // structured content itself.
      return out.structuredContent || out;
    } catch (e) {
      return undefined;
    }
  };

  let openAiTimer = null;
  let pollsLeft = 40;
  const stopOpenAiPoll = () => {
    if (openAiTimer !== null) {
      window.clearInterval(openAiTimer);
      openAiTimer = null;
    }
  };
  const pollOpenAi = () => {
    try {
      const sc = readOpenAiOutput();
      if (sc !== undefined && tryRenderProducts(sc)) {
        stopOpenAiPoll(); // rendered once: done polling
        return;
      }
    } catch (e) {
      stopOpenAiPoll(); // never throw
      return;
    }
    if (--pollsLeft <= 0) stopOpenAiPoll(); // exhausted: give up quietly
  };
  openAiTimer = window.setInterval(pollOpenAi, 250);

  empty();
})();
</script>
</body>
</html>
`

// WidgetResource returns the widget's MCP resource descriptor along with the
// HTML to serve for it. The URI is stable and versioned: hosts treat it as a
// cache key, so UI changes ship under a new version.
func WidgetResource() (*mcp.Resource, string) {
	r := &mcp.Resource{
		Name:        "products-widget",
		URI:         WidgetResourceURI,
		MIMEType:    WidgetMIMEType,
		Description: "HTML widget that displays products returned by render_products_widget as cards.",
	}
	return r, widgetHTML
}

// RenderInput selects which products the widget should display.
type RenderInput struct {
	ProductIDs []string `json:"product_ids" jsonschema:"ids of products to display, chosen from search_products results"`
}

// RenderOutput is the widget's payload; the host forwards it to the iframe via
// ui/notifications/tool-result and the widget renders these cards.
type RenderOutput struct {
	Store    string            `json:"store"`
	Products []catalog.Product `json:"products"`
}

// RenderProductsHandler returns the render_products_widget handler. It is
// presentation-only: it resolves IDs against the catalog and attaches the UI
// meta pointing hosts at the widget resource. All filtering remains the
// calling AI's job via search_products.
func RenderProductsHandler(c *catalog.Catalog) mcp.ToolHandlerFor[RenderInput, RenderOutput] {
	byID := make(map[string]catalog.Product, len(c.Products))
	for _, p := range c.Products {
		byID[p.ID] = p
	}

	return func(ctx context.Context, req *mcp.CallToolRequest, args RenderInput) (*mcp.CallToolResult, RenderOutput, error) {
		var (
			products []catalog.Product
			unknown  = map[string]bool{}
			seen     = map[string]bool{}
		)
		for _, id := range args.ProductIDs {
			if seen[id] {
				continue
			}
			seen[id] = true
			if p, ok := byID[id]; ok {
				products = append(products, p)
			} else {
				unknown[id] = true
			}
		}

		if len(unknown) > 0 {
			// Error (not an error result) so the model can self-correct and
			// retry with valid ids from search_products.
			ids := make([]string, 0, len(unknown))
			for id := range unknown {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			return nil, RenderOutput{}, fmt.Errorf("unknown product id(s): %v", ids)
		}

		out := RenderOutput{Store: c.StoreName, Products: products}
		res := &mcp.CallToolResult{
			// StructuredContent is left unset: the SDK populates it from out.
			Meta: mcp.Meta{
				"ui":                             map[string]any{"resourceUri": WidgetResourceURI},
				"openai/toolInvocation/invoking": WidgetInvoking,
				"openai/toolInvocation/invoked":  WidgetInvoked,
			},
		}
		return res, out, nil
	}
}
