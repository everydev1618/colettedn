# Infrastructure Specification

## AWS Region

**All resources MUST be deployed to `us-east-1`.**

Do not create resources in other regions.

## Architecture Overview

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   CloudFlare    │────▶│   API Gateway    │────▶│     Lambda      │
│  (colettedn.com)│     │   (HTTP API)     │     │  (VPC-attached) │
└─────────────────┘     └──────────────────┘     └────────┬────────┘
                                                          │
                        ┌──────────────────┐              │
                        │   Function URL   │──────────────┤
                        │ (bypass timeout) │              │
                        └──────────────────┘              │
                                                          ▼
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│    DynamoDB     │◀────│  Private Subnet  │────▶│   NAT Gateway   │
│ (colettedn-cache│     │   (Lambda)       │     │  (Static EIP)   │
└─────────────────┘     └──────────────────┘     └────────┬────────┘
                                                          │
                                                          ▼
                                                 ┌─────────────────┐
                                                 │  External APIs  │
                                                 │ - Anthropic     │
                                                 │ - Namecheap     │
                                                 └─────────────────┘
```

## Key Resources

### Lambda Function
- **Name**: `colettedn-ColetteDNFunction-*`
- **Runtime**: `provided.al2023` (Go binary)
- **Timeout**: 120 seconds (Function URL), 29 seconds (API Gateway)
- **Memory**: 256 MB
- **VPC**: Attached to private subnet for static outbound IP

### VPC Configuration
- **CIDR**: `10.0.0.0/16`
- **Public Subnet**: `10.0.1.0/24` (NAT Gateway)
- **Private Subnet**: `10.0.2.0/24` (Lambda)
- **NAT Gateway EIP**: `100.52.14.203` (static IP for Namecheap whitelist)

### DynamoDB
- **Table**: `colettedn-cache`
- **Billing**: Pay-per-request
- **TTL**: Enabled on `ttl` attribute
- **VPC Endpoint**: Gateway endpoint (free, bypasses NAT)

### API Gateway
- **Type**: HTTP API (v2)
- **Custom Domain**: `colettedn.com`
- **Certificate**: ACM (DNS validated)

## Environment Variables

| Variable | Description | Source |
|----------|-------------|--------|
| `ANTHROPIC_API_KEY` | Claude API key | SAM parameter |
| `NAMECHEAP_API_USER` | Namecheap API username | SAM parameter |
| `NAMECHEAP_API_KEY` | Namecheap API key | SAM parameter |
| `NAMECHEAP_USERNAME` | Namecheap account username | SAM parameter |
| `NAMECHEAP_CLIENT_IP` | Static IP for API auth | `!GetAtt NatEIP.PublicIp` |
| `NAMECHEAP_SANDBOX` | Use sandbox API | `false` |

## Namecheap API Setup

1. The Lambda uses NAT Gateway for outbound traffic
2. NAT Gateway has static EIP: `100.52.14.203`
3. This IP **must be whitelisted** in Namecheap API Access settings
4. API Access: https://ap.www.namecheap.com/settings/tools/apiaccess/

## Deployment

```bash
# Build and deploy (from project root)
sam build && sam deploy

# Or use Makefile
make deploy
```

### SAM Parameters Required
- `AnthropicApiKey`
- `NamecheapApiUser`
- `NamecheapApiKey`
- `NamecheapUsername`

## Outputs

| Output | Description |
|--------|-------------|
| `ApiUrl` | API Gateway endpoint |
| `FunctionUrl` | Lambda Function URL (no timeout) |
| `CustomDomainUrl` | https://colettedn.com/ |
| `ApiGatewayDomainTarget` | CNAME target for CloudFlare |
| `NamecheapWhitelistIP` | IP to whitelist in Namecheap |
