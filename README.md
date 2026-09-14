# Memogram

**Memogram** is a channel adapter for [Memos](https://www.usememos.com/). It syncs messages and images from Telegram into your instance. The process is a single binary: adapters speak the platform protocol, Core talks to Memos.

Telegram is the only adapter today. Inbound is always a long connection (Bot API long poll). The process does not expose an HTTP server.

## Architecture

```text
  Telegram user
        |
        |  Bot API long poll  (no inbound HTTP)
        v
+---------------------------+
|  telegram Adapter         |
|  ACL, entities, forward,  |
|  media download, keyboard |
+-------------+-------------+
              |  InboundEvent
              v
+---------------------------+       +----------------------+
|  Core                     |------>|  Binding store       |
|  bind / create / search   |       |  data.txt            |
|  visibility / pin         |       |  (channel, user_id)  |
+-------------+-------------+       |  -> Memos token      |
              |                     +----------------------+
              |  Connect HTTP + user Bearer token
              v
         Memos instance
              |
              |  OutboundMessage
              v
        telegram Adapter.Reply
```

Rules:

- Core never imports a bot SDK. Adapters never call the Memos API.
- `data.txt` is the identity map, not a note store. Memos holds the content.
- A channel is registered only when its credentials are present. Startup fails if none are enabled.

## Prerequisites

- A running Memos instance
- A Telegram bot token (to enable the Telegram adapter)

## Installation

Download the binary for your OS from the [Releases](https://github.com/usememos/telegram-integration/releases) page, or build from source:

```sh
go build -o memogram ./cmd/memos-channel
```

## Configuration

Create a `.env` file in the project's root directory:

```env
SERVER_ADDR=https://your-memos.example
BOT_TOKEN=your_telegram_bot_token
BOT_PROXY_ADDR=https://api.your_proxy_addr.com
ALLOWED_USERNAMES=user1,user2,user3
DATA=data.txt
```

### Configuration Options

- `SERVER_ADDR` (required): Memos HTTP origin used by the Connect client. `https://host` is preferred. The historical `dns:host:port` prefix is still stripped and treated as `http://host:port`.
- `BOT_TOKEN`: Telegram bot token. If set, the Telegram adapter is registered.
- `BOT_PROXY_ADDR`: Optional Telegram Bot API proxy. Leave empty if not needed.
- `ALLOWED_USERNAMES`: Optional comma-separated Telegram usernames (no `@`). Telegram-only inbound firewall.
- `DATA`: Binding file path. Defaults to `data.txt`. Keep this file private; it stores access tokens.

### Username Restrictions

When `ALLOWED_USERNAMES` is set, only listed Telegram usernames can use the bot.

1. Allow specific users:

   ```env
   ALLOWED_USERNAMES=alex,john,emily
   ```

2. Allow all users (leave empty or omit the variable):

   ```env
   ALLOWED_USERNAMES=
   ```

Notes:

- Usernames must not include the `@` symbol
- Matching is case-insensitive and trims whitespace
- If the allowlist is set, accounts without a Telegram username are rejected
- Users not in the list receive: `your account <name> is not allowed to use this bot`

## Usage

### Starting the Service

#### Starting with binary

1. Download and extract the released binary, or build `./cmd/memos-channel`.
2. Create a `.env` file in the same directory as the binary.
3. Run:

   ```sh
   ./memogram
   ```

4. Talk to the bot in Telegram.

#### Starting with Docker

1. Build the image: `docker build -t memogram .`
2. Run with environment variables:

   ```sh
   docker run -d --name memogram \
     -e SERVER_ADDR=dns:localhost:5230 \
     -e BOT_TOKEN=your_telegram_bot_token \
     memogram
   ```

#### Starting with Docker Compose

This can sit next to Memos in the same compose file:

1. Create a folder for the service.
2. Clone this repository into a subfolder: `git clone https://github.com/usememos/telegram-integration memogram`
3. Create `.env`:

   ```sh
   SERVER_ADDR=dns:yourMemosUrl.com:5230
   BOT_TOKEN=your_telegram_bot_token
   ```

4. Create `docker-compose.yml`:

   ```yaml
   services:
     memogram:
       env_file: .env
       build: memogram
       container_name: memogram
   ```

5. Start with `docker compose up -d`.

### Interaction Commands

- `/start <access_token>`: Bind this Telegram user to a Memos access token.
- `/list`: Show the 10 most recently updated memos. Tap a row for the full note; use Prev/Next to page. From a note: Back, Edit, Delete.
- `/tags`: List tags, then the same paged list filtered to that tag.
- `/search <words>`: Same list card as `/list`, filtered by memo content.
- Send text: save as a memo. After Edit, the next text overwrites the open memo (`/cancel` aborts).
- Send files (photos, documents, voice, video): attach them to a memo.
- Public / Private / Pin buttons: update the saved memo.
