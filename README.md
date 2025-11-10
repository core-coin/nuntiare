# Nuntiare

Nuntiare is a notification service that listens to Core blockchain events and delivers alerts to subscribers via Telegram and email. It exposes a lightweight HTTP API for registering wallets and inspecting subscription status while persisting state in PostgreSQL.

## Features
- Watches Core blocks in real time through the configured RPC endpoint.
- Detects transfers for multiple token types:
  - **CBC20 tokens** - fungible tokens like CTN, USDT, etc.
  - **CBC721 tokens** - NFTs and unique assets
  - **Native XCB transfers** - native Core blockchain currency
- Automatically discovers and watches tokens from the [.well-known token registry](https://github.com/bchainhub/well-known) with hourly updates.
- Tracks wallet subscriptions, payments, whitelist status, and notification preferences in PostgreSQL.
- Sends notifications through Telegram bots and SMTP email providers.
- Provides simple HTTP endpoints for registering wallets and checking if a subscription is active.
- Ships with Docker Compose for spin‑up alongside PostgreSQL.

## Prerequisites
- Go 1.22+
- PostgreSQL 13+
- A running Core RPC node reachable over WebSocket/HTTP (e.g. `ws://127.0.0.1:8546`).
- (Optional) Telegram bot token and SMTP credentials if you intend to send notifications through those channels.

## Quick Start (local Go environment)
1. **Clone and enter the repository**
   ```bash
   git clone https://github.com/core-coin/nuntiare.git
   cd nuntiare
   ```
2. **Create a configuration file**
   ```bash
   cp .env-sample .env
   ```
   Edit `.env` to point to your PostgreSQL instance, Core RPC endpoint, smart contract address, and notification credentials.
3. **Install Go dependencies**
   ```bash
   go mod download
   ```
4. **Start PostgreSQL** (locally or via Docker) and make sure the values in `.env` match the running instance.
5. **Run the service**
   ```bash
   make run
   ```
   The application loads configuration from environment variables (or `.env` via `godotenv`), performs database auto-migrations, connects to the blockchain, and starts the HTTP API on the configured port (default `6532`).

The binary can also be executed directly: `./nuntiare --postgres-user=... --blockchain-service-url=...` to override individual options at runtime.

## Running with Docker Compose
1. Copy `.env-sample` to `.env` and adjust values. Docker Compose exports the variables into both the application and PostgreSQL containers.
2. Bring everything up:
   ```bash
   docker-compose --env-file .env up --build -d
   ```
3. Stop the stack when finished:
   ```bash
   docker-compose down
   ```

This setup starts a `postgres` container and the `nuntiare` service exposed on `localhost:6532`.

## Configuration
| Variable | Description | Default |
| --- | --- | --- |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | PostgreSQL credentials and database name. | `postgres` / `password` / `nuntiare` |
| `POSTGRES_HOST` / `POSTGRES_PORT` | PostgreSQL host and port. | `localhost` / `5432` |
| `BLOCKCHAIN_SERVICE_URL` | Core RPC endpoint (`xcbclient.Dial` compatible). | `http://localhost:8545` |
| `SMART_CONTRACT_ADDRESS` | Core Token (CTN) contract address used for subscription payments. **This is the only token used for subscription payments.** | _none_ |
| `NETWORK_ID` | Chain ID forwarded to go-core. Also determines network name for .well-known registry: `1` = xcb (mainnet), `3` = xab (devin). | `1` |
| `WELL_KNOWN_URL` | Base URL for the .well-known token registry service. | `https://coreblockchain.net` |
| `API_PORT` | HTTP API port. | `6532` |
| `DEVELOPMENT` | Enables more verbose logging when `true`. | `false` |
| `TELEGRAM_BOT_TOKEN` | Bot token from [@BotFather](https://t.me/BotFather). Needed for Telegram notifications. | _none_ |
| `SMTP_HOST` / `SMTP_PORT` / `SMTP_ALTERNATIVE_PORT` | SMTP server host and ports. | `smtp.example.com` / `587` / `465` |
| `SMTP_USER` / `SMTP_PASSWORD` | SMTP authentication credentials. | _none_ |
| `SMTP_SENDER` | Email sender address used in outgoing messages. | _none_ |

All options are also exposed as CLI flags. Run `go run ./cmd/nuntiare --help` to see the full list (`--postgres-user`, `--api-port`, `--telegram-bot-token`, etc.). Flag values override environment variables.

## HTTP API
Base URL: `http://<host>:<API_PORT>/api/v1`

| Endpoint | Method | Purpose | Request Body/Params |
| --- | --- | --- | --- |
| `/subscription` | POST | Register a wallet, subscription address, and notification preferences. | JSON body (see below) |
| `/is_subscribed` | GET | Check if a wallet currently has an active subscription. | Query param: `address` |

### POST `/subscription` - Register Wallet

**Request Body (JSON):**
```json
{
  "origin": "string (required)",
  "subscriber": "string (required)",
  "destination": "string (required)",
  "network": "string (required)",
  "telegram": "string (optional)",
  "email": "string (optional)"
}
```

**Fields:**
- `origin`: Originator/source identifier (e.g., "payto", "Acme")
- `subscriber`: Subscription payment address (where user sends CTN for subscription)
- `destination`: Wallet address to watch for incoming transfers
- `network`: Network identifier (e.g., "xcb" for mainnet, "xab" for devin)
- `telegram`: (Optional) Telegram username without `@`. User must run `/start` with the bot to activate.
- `email`: (Optional) Email address for notifications

**Response (Success - 201 Created):**
```json
{
  "success": true,
  "message": "Wallet registered successfully",
  "address": "0xReceivingWallet",
  "subscription_address": "0xSubscriptionWallet"
}
```

**Response (Error - 400/500):**
```json
{
  "success": false,
  "error": "Error message"
}
```

**Example registration request:**
```bash
curl -X POST http://localhost:6532/api/v1/subscription \
  -H "Content-Type: application/json" \
  -d '{
    "origin": "payto",
    "subscriber": "cb1234567890abcdef1234567890abcdef12345678",
    "destination": "cb9876543210fedcba9876543210fedcba98765432",
    "network": "xcb",
    "telegram": "alice_core",
    "email": "alice@example.com"
  }'
```

### GET `/is_subscribed` - Check Subscription Status

**Query Parameters:**
- `address`: Wallet address to check

**Response (200 OK):**
```json
true
```
or
```json
false
```

**Example:**
```bash
curl "http://localhost:6532/api/v1/is_subscribed?address=cb9876543210fedcba9876543210fedcba98765432"
```

## How Notifications Work
- The service keeps long-lived subscriptions to new block headers from the configured Core RPC endpoint.
- For each block it checks transactions for:
  - **Native XCB transfers** targeting registered wallets
  - **CBC20 token transfers** (fungible tokens) for all tokens in the .well-known registry
  - **CBC721 token transfers** (NFTs) for all NFT contracts in the .well-known registry
  - **CTN transfers** to subscription addresses for payment tracking
- The token list is automatically fetched from the .well-known service on startup and refreshed every hour to ensure new tokens are detected.
- **Subscription Payments**: Only the CTN token (configured via `SMART_CONTRACT_ADDRESS`) is used for subscription payments. Wallets stay subscribed while their accumulated CTN payments are at least 200 CTN within the trailing month. Payments are tracked by monitoring transfers to each wallet's `SubscriptionAddress`.
- Telegram notifications are sent once the bot has a chat ID for the registered username (user must send `/start`). Email notifications use basic SMTP authentication.
- **Core Blockchain Hashing**: The Core blockchain uses SHA3-NIST for hashing instead of Keccak-256 used by Ethereum.

## Database
Nuntiare uses GORM with automatic migrations for the following tables:
- `wallets`: wallet metadata, whitelisting, and subscription address.
- `subscription_payments`: historical CTN payments (used to confirm active subscriptions).
- `notification_providers`, `telegram_providers`, `email_providers`: notification preferences per wallet.

**Note**: Token metadata from the .well-known registry is cached in memory (not in the database) for performance. The cache is refreshed hourly.

Migrations run automatically at startup. You only need to provide a reachable PostgreSQL instance.

## Development Tips
- `make run` – build and start the service.
- `make test` – execute unit tests.
- `make fmt` – format the Go code.
- `make clean` – remove build artifacts.
- `make docker-run` / `make docker-down` – convenience wrappers around Docker Compose.

Logs default to structured output; set `DEVELOPMENT=true` for more verbose debugging information.

## Well-Known Token Registry Integration

Nuntiare integrates with the [.well-known token registry](https://github.com/bchainhub/well-known) to automatically discover and watch token contracts on the Core blockchain:

- **Automatic Token Discovery**: On startup, the service fetches all CBC20 and CBC721 tokens from the configured .well-known service.
- **Hourly Updates**: The token list is refreshed every hour to detect newly added tokens.
- **Token Types Supported**:
  - **CBC20**: Fungible tokens (e.g., stablecoins, utility tokens)
  - **CBC721**: Non-fungible tokens (NFTs)
- **In-Memory Cache**: Token metadata (address, symbol, decimals, type) is stored in a thread-safe in-memory cache for fast lookup during block processing. The cache is protected by a read-write mutex to handle concurrent access safely.
- **Network-Specific**: Tokens are fetched for the specific network configured via the `NETWORK` environment variable (e.g., `mainnet`, `devin`).

The service automatically detects transfers for all tokens in the registry and sends notifications to subscribed wallets without requiring manual configuration of contract addresses.

## Troubleshooting
- **Cannot connect to Core RPC**: verify `BLOCKCHAIN_SERVICE_URL`, ensure the node accepts WebSocket connections, and that the smart contract address is correct. The service will retry subscriptions every five seconds if the channel closes.
- **No notifications after registering**: confirm the wallet paid at least 200 CTN to the assigned subscription address and that the Telegram user initiated the bot session (if using Telegram). Check the database tables to ensure the wallet registration succeeded.
- **Email errors**: validate SMTP credentials and ports. The service currently uses TLS/STARTTLS on the primary port and falls back to the alternative port if configured.
- **Well-known service errors**: verify that `WELL_KNOWN_URL` is correct and accessible. Check that `NETWORK` matches the network name used by the well-known service (e.g., `devin` for testnet, `mainnet` for production). The service will log errors but continue operating with previously cached tokens if the well-known service is temporarily unavailable.

With a running Core node, PostgreSQL, and notification credentials configured, `nuntiare` runs as a single binary or Docker service ready to deliver blockchain payment alerts.
