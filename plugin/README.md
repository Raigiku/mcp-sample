# Demo Outdoor Gear — ChatGPT plugin package

Portable Agent Plugins package that connects ChatGPT/Codex to this repo's
ecommerce MCP server. Layout follows the
[plugin packaging guide](https://developers.openai.com/plugins/build/plugins):

```
plugin/
├── plugin.json   # portable manifest (identity + OpenAI install-surface metadata)
└── mcp.json      # bundled MCP server: streamable-http → the deployed store server
```

The MCP server this package points at is the Go binary in this repo, served
over the current ngrok tunnel:

```
https://infundibulate-lakiesha-prepigmental.ngrok-free.dev/mcp
```

## ⚠️ Ephemeral URL

The ngrok URL changes every time the tunnel restarts. If the tools stop
responding, restart the tunnel, get the new URL from
`http://localhost:4040/api/tunnels`, and update the `url` in `mcp.json`.

## Installing for testing (developer mode)

1. **Run the server** (repo root): `./bin/store-mcp -store demo-store -http :9090`
   and keep the ngrok tunnel running (`ngrok http 9090`).
2. **Enable developer mode** in ChatGPT: Settings → Security and login →
   Developer mode.
3. **Register the MCP connection**: [chatgpt.com/plugins](https://chatgpt.com/plugins)
   → plus button → enter the MCP URL above, no auth. Copy the technical ID
   from the browser URL (starts with `plugin_asdk_app`).
4. **Wire the registered app into the plugin**: create `plugin/.app.json`
   mapping the registered connection —
   ```json
   {
     "apps": {
       "demo-outdoor-gear": { "id": "plugin_asdk_app_..." }
     }
   }
   ```
   and add `"apps": "./.app.json"` under `extensions.com.openai` in
   `plugin.json`. (This step is only needed for the registered-server flow;
   the `mcp.json` server alone is enough for repo/personal marketplace
   installs.)
5. **Install from a local marketplace** (repo marketplace shown):
   add `.agents/plugins/marketplace.json` at the repo root pointing at
   `./plugin` (see the guide's "Build your own curated plugin list"), restart
   the ChatGPT desktop app, and install from your local source in the
   Plugins Directory.
6. Test in a new chat: "Show me what's in the store." — the AI should call
   `search_products`, then can call `render_products_widget` to display cards.

## What the AI gets

- `search_products` — the full catalog (data tool, no UI), for the AI to filter
- `render_products_widget` — read-only card display for the products the AI
  selected (MCP Apps UI via `_meta.ui.resourceUri`)