# ColetteDN

AI-powered domain name brainstorming, built on [Vega](https://github.com/everydev1618/govega).

Chat with Colette — describe your project and she'll brainstorm creative domain names, check availability via RDAP in real-time, and only show you what's actually available.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/everydev1618/colettedn/main/install.sh | bash
```

This downloads the binary, prompts for your [Anthropic API key](https://console.anthropic.com/), and you're done:

```bash
colettedn
```

Open **http://localhost:3001** and chat with Colette.

## Build from source

```bash
git clone https://github.com/everydev1618/colettedn.git
cd colettedn
go run cmd/colettedn/main.go
```

## How It Works

1. You describe your project to **Colette** (domain brainstorming expert)
2. She generates 20-30 creative names across .com, .io, .ai, .dev, .app, and more
3. She batch-checks availability using RDAP (free, authoritative registry lookups)
4. You only see domains that are actually available

You can also ask **Iris** (the orchestrator) to create additional agents — like a brand strategist who works with Colette to ask you discovery questions before searching.

## Architecture

This is a [Vega](https://github.com/everydev1618/govega) app. The entire thing is:

- **`colette.vega.yaml`** — Agent definition (system prompt, tool bindings)
- **`cmd/colettedn/main.go`** — Registers RDAP tools, starts the Vega dashboard
- **`internal/rdap/`** — RDAP client for domain availability checking (IANA bootstrap, retry logic, DNS fallback)
- **`internal/tools/`** — Thin wrappers that expose RDAP as Vega tools

Everything else — the web UI, chat streaming, agent orchestration, conversation history, cost tracking — comes from Vega.
