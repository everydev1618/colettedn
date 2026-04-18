# Claude Code Instructions

## Project Overview

ColetteDN is an AI-powered domain name brainstorming tool built on [Vega](https://github.com/everydev1618/govega). Users chat with Colette, describe their project, and get available domain suggestions verified via RDAP.

## Project Structure

```
├── cmd/colettedn/      # Main entrypoint (vega serve + custom tools)
├── colette.vega.yaml   # Agent definition
├── internal/
│   ├── rdap/           # RDAP domain availability checking
│   └── tools/          # Vega tool wrappers for RDAP
```

## Common Commands

```bash
# Run locally
go run cmd/colettedn/main.go

# Run tests
go test -short ./...

# Build binary
go build -o colettedn cmd/colettedn/main.go
```

## Configuration

API key is read from `~/.vega/env`:
```
ANTHROPIC_API_KEY=sk-ant-...
```
