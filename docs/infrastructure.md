# Infrastructure Specification

## AWS Region

**All resources MUST be deployed to `us-east-1`.**

Do not create resources in other regions.

## Architecture Overview

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   CloudFlare    │────▶│   API Gateway    │────▶│     Lambda      │
│  (colettedn.com)│     │   (HTTP API)     │     │                 │
└─────────────────┘     └──────────────────┘     └────────┬────────┘
                                                          │
                        ┌──────────────────┐              │
                        │   Function URL   │──────────────┤
                        │ (bypass timeout) │              │
                        └──────────────────┘              │
                                                          ▼
┌─────────────────┐                              ┌─────────────────┐
│    DynamoDB     │◀─────────────────────────────│  External APIs  │
│ (colettedn-*)   │                              │ - Anthropic     │
└─────────────────┘                              │ - RDAP (free)   │
                                                 └─────────────────┘
```

## Key Resources

### Lambda Function
- **Name**: `colettedn-ColetteDNFunction-*`
- **Runtime**: `provided.al2023` (Go binary)
- **Timeout**: 120 seconds (Function URL), 29 seconds (API Gateway)
- **Memory**: 256 MB

### DynamoDB Tables
| Table | Purpose |
|-------|---------|
| `colettedn-cache` | Domain availability cache (24h TTL) |
| `colettedn-users` | User accounts |
| `colettedn-tokens` | Auth tokens (magic links, sessions) |
| `colettedn-favorites` | Saved domains |
| `colettedn-history` | Search history |
| `colettedn-owned` | Registered domains |
| `colettedn-analytics` | Usage metrics |

### API Gateway
- **Type**: HTTP API (v2)
- **Custom Domain**: `colettedn.com`
- **Certificate**: ACM (DNS validated)

## Domain Availability Checking

Domain availability is checked via **RDAP** (Registration Data Access Protocol):
- Queries authoritative registries directly (Verisign for .com/.net, etc.)
- Free, no API key required
- More reliable than registrar APIs
- 404 response = domain available, 200 response = domain taken

Supported TLDs: `.com`, `.net`, `.org`, `.io`, `.ai`, `.co`, `.app`, `.dev`, `.me`, `.xyz`, `.tech`

## Environment Variables

| Variable | Description | Source |
|----------|-------------|--------|
| `ANTHROPIC_API_KEY` | Claude API key | SAM parameter |
| `APP_URL` | Application URL | Derived from DomainName |
| `FROM_EMAIL` | SES sender email | Derived from DomainName |
| `STRIPE_SECRET_KEY` | Stripe API key | SAM parameter |
| `STRIPE_WEBHOOK_SECRET` | Stripe webhook signing | SAM parameter |
| `STRIPE_PRICE_ID` | Subscription price ID | SAM parameter |

## Deployment

```bash
# Build and deploy (from project root)
sam build && sam deploy

# Or use Makefile
make deploy
```

### SAM Parameters Required
- `AnthropicApiKey`
- `StripeSecretKey`
- `StripeWebhookSecret`
- `StripePriceId`

## Outputs

| Output | Description |
|--------|-------------|
| `ApiUrl` | API Gateway endpoint |
| `FunctionUrl` | Lambda Function URL (no timeout) |
| `CustomDomainUrl` | https://colettedn.com/ |
| `ApiGatewayDomainTarget` | CNAME target for CloudFlare |
