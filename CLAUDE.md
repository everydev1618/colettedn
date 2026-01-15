# Claude Code Instructions

## Project Overview

ColetteDN is an AI-powered domain name brainstorming tool. Users describe their project and get categorized domain suggestions with real-time availability checking via Namecheap API.

## Before Making Infrastructure Changes

**ALWAYS read `docs/infrastructure.md` first** to understand:
- AWS region requirements (us-east-1 only)
- VPC/networking setup
- Environment variables
- Namecheap API dependencies

## Key Constraints

1. **Region**: All AWS resources MUST be in `us-east-1`
2. **Static IP**: Lambda uses NAT Gateway EIP (`100.52.14.203`) for Namecheap API - this IP must stay whitelisted
3. **Timeouts**: API Gateway has 29s limit; use Function URL for long operations

## Project Structure

```
├── cmd/lambda/         # Lambda entrypoint
├── internal/
│   ├── cache/          # DynamoDB/SQLite caching
│   ├── generator/      # Claude API integration
│   ├── handler/        # HTTP handlers
│   ├── killswitch/     # Cost protection
│   ├── namecheap/      # Domain availability API
│   └── ratelimit/      # Rate limiting
├── frontend/           # Static HTML/JS
├── docs/               # Infrastructure documentation
└── template.yaml       # SAM/CloudFormation template
```

## Common Commands

```bash
# Local development
go run cmd/lambda/main.go

# Deploy to AWS
sam build && sam deploy

# Check logs
aws logs tail /aws/lambda/colettedn-ColetteDNFunction-* --follow
```

## Environment Variables

See `docs/infrastructure.md` for full list. Local dev uses `.env` file.
