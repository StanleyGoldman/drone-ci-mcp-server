# drone-ci-mcp-server

An [MCP (Model Context Protocol)](https://modelcontextprotocol.io) server that exposes your [Drone CI](https://www.drone.io) instance as tools for Claude and other MCP clients.

Once connected, you can ask Claude things like:
- "What's the status of the latest build for acme/widget?"
- "Show me the logs for the failing step in build 42"
- "Trigger a build of acme/gadget on the feature/x branch"
- "Cancel build 99 for acme/widget"

## MCP Tools

| Tool | Description |
|---|---|
| `list_repos` | List all repositories with Drone CI enabled |
| `get_repo` | Get details for a specific repository |
| `list_builds` | List recent builds for a repository |
| `get_build` | Get build details including stages and steps |
| `get_build_logs` | Get log output for a specific build step |
| `trigger_build` | Trigger a new build on a branch |
| `cancel_build` | Cancel a running build |
| `restart_build` | Restart an existing build from the same commit |

## Development

### Prerequisites

- Go 1.24+
- Docker + Docker Compose (for Docker targets)
- [golangci-lint](https://golangci-lint.run/usage/install/) (for `make lint`)

### Using a Dev Container (recommended)

Open this repository in VS Code and select **Reopen in Container**. The dev container provides Go, Docker-outside-of-Docker, and the GitHub CLI pre-installed.

### Building and testing

```bash
# Build the binary
make build

# Run unit tests
make test

# Run unit + integration tests (full HTTP stack, no external services required)
make test-integration

# Generate an HTML coverage report (opens coverage.html)
make coverage

# Run the linter
make lint
```

### Running locally

```bash
cp .env.example .env
# Edit .env — set DRONE_SERVER and DRONE_TOKEN at minimum
make run
```

The server starts on `http://localhost:8080` by default.

## Deployment

### Local network (Claude Desktop)

Use this when Claude Desktop is on the same network as your Drone CI server.

```bash
cp .env.example .env
# Fill in DRONE_SERVER and DRONE_TOKEN
docker compose up -d
```

The MCP server will be available at `http://<host-ip>:8080`.

Add it to your Claude Desktop config (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "drone-ci": {
      "type": "http",
      "url": "http://192.168.1.100:8080"
    }
  }
}
```

If you set `MCP_AUTH_TOKEN`, include it as a header:

```json
{
  "mcpServers": {
    "drone-ci": {
      "type": "http",
      "url": "http://192.168.1.100:8080",
      "headers": {
        "Authorization": "Bearer your_token_here"
      }
    }
  }
}
```

### Internet-facing (claude.ai online sessions)

Use this to allow Claude on claude.ai to reach your server over the internet.

**Prerequisites:**
- A domain name with an A record pointing to this host
- Ports 80 and 443 open on your firewall / router port-forwarded to this host

```bash
cp .env.example .env
# Fill in DRONE_SERVER, DRONE_TOKEN, MCP_AUTH_TOKEN (strongly recommended), and MCP_HOSTNAME
docker compose -f docker-compose.caddy.yml up -d
```

Caddy automatically obtains and renews a TLS certificate from Let's Encrypt.

Add it in the Claude web app's MCP settings:

```
URL: https://drone-mcp.yourdomain.com
Authorization: Bearer your_token_here
```

## Configuration Reference

| Variable | Required | Default | Description |
|---|---|---|---|
| `DRONE_SERVER` | Yes | — | Drone CI server URL (e.g. `http://drone.example.com`) |
| `DRONE_TOKEN` | Yes | — | Drone CI personal access token |
| `MCP_PORT` | No | `8080` | Port the MCP server listens on |
| `MCP_AUTH_TOKEN` | No | — | Bearer token to protect the MCP endpoint. Empty = no auth |
| `MCP_HOSTNAME` | Caddy only | — | Domain name for auto-TLS (Caddy deployment only) |

## Access Control

- **No auth** (`MCP_AUTH_TOKEN` empty): suitable for trusted local networks where the port is not exposed to the internet.
- **Bearer token** (`MCP_AUTH_TOKEN` set): every request must include `Authorization: Bearer <token>`. Strongly recommended for any internet-facing deployment.
- **TLS**: provided by Caddy in the `docker-compose.caddy.yml` deployment. Caddy auto-obtains a Let's Encrypt certificate.

## Architecture

```
Claude (online) ──HTTPS──► Caddy (TLS, :443) ──► MCP server (:8080)
                                                        │
Local Claude ──────────────────────────────────────────►│
                                                        │
                                                   Drone CI API
```

The MCP server uses **Streamable HTTP transport** (not stdio), which supports multiple concurrent clients and works over a network.
