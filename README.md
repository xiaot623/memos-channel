# Memogram

**Memogram** is a channel adapter for [Memos](https://www.usememos.com/). It syncs messages and images from Telegram and Feishu/Lark into your instance. The process is a single binary: adapters speak the platform protocol, Core talks to Memos.

Inbound is always a long connection (Telegram Bot API long poll, Feishu/Lark WebSocket). The process does not expose an HTTP server. A channel is registered only when its credentials are present.

## Architecture

```text
  Telegram user              Feishu/Lark user
        |                            |
        |  Bot API long poll         |  Open Platform WebSocket
        |  (no inbound HTTP)         |  (no inbound HTTP)
        v                            v
+---------------------------+  +---------------------------+
|  telegram Adapter         |  |  feishu Adapter           |
|  ACL, entities, forward,  |  |  ACL, post/md, cards,     |
|  media download, keyboard |  |  media download           |
+-------------+-------------+  +-------------+-------------+
              |  InboundEvent                |
              +--------------+---------------+
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
                    Adapter.Reply (same channel)
```

Rules:

- Core never imports a bot SDK. Adapters never call the Memos API.
- `data.txt` is the identity map, not a note store. Memos holds the content.
- A channel is registered only when its credentials are present. Startup fails if none are enabled.
- Telegram and Feishu bindings are independent (`telegram` vs `feishu` keys in `data.txt`).

## Prerequisites

- A running Memos instance
- At least one channel credential:
  - A Telegram bot token, and/or
  - A Feishu/Lark custom app App ID and App Secret (see [Feishu](#feishu))

## Installation

Download the binary for your OS from the [Releases](https://github.com/usememos/telegram-integration/releases) page, or build from source:

```sh
go build -o memogram ./cmd/memos-channel
```

## Configuration

Create a `.env` file in the project's root directory:

```env
SERVER_ADDR=https://your-memos.example
BASE_URL=https://your-memos.example
BOT_TOKEN=your_telegram_bot_token
BOT_PROXY_ADDR=https://api.your_proxy_addr.com
ALLOWED_USERNAMES=user1,user2,user3
FEISHU_APP_ID=cli_xxx
FEISHU_APP_SECRET=your_app_secret
FEISHU_BASE_URL=https://open.feishu.cn
FEISHU_ALLOWED_OPEN_IDS=
DATA=data.txt
```

### Configuration Options

- `SERVER_ADDR` (required): Memos HTTP origin used by the Connect client. `https://host` is preferred. The historical `dns:host:port` prefix is still stripped and treated as `http://host:port`.
- `BASE_URL` (optional): Public `https://` origin for Open buttons and saved-memo links. `http://` values are ignored. If unset, memogram falls back to the Memos instance profile URL when that is also `https://`. Otherwise no Open button is shown.
- `BOT_TOKEN`: Telegram bot token. If set, the Telegram adapter is registered.
- `BOT_PROXY_ADDR`: Optional Telegram Bot API proxy. Leave empty if not needed.
- `ALLOWED_USERNAMES`: Optional comma-separated Telegram usernames (no `@`). Telegram-only inbound firewall.
- `FEISHU_APP_ID` / `FEISHU_APP_SECRET`: Feishu/Lark custom app credentials. Both must be set to register the Feishu adapter.
- `FEISHU_BASE_URL`: Optional Open Platform origin. Defaults to `https://open.feishu.cn`. Use `https://open.larksuite.com` for Lark International.
- `FEISHU_ALLOWED_OPEN_IDS`: Optional comma-separated Feishu `open_id` values. Feishu-only inbound firewall. Empty means anyone in the app's availability range.
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

## Feishu

Create a **custom enterprise app** (商店应用 cannot use long connection):

- China: [https://open.feishu.cn/app](https://open.feishu.cn/app) → 创建企业自建应用
- Lark International: [https://open.larksuite.com/app](https://open.larksuite.com/app)

Then:

1. Enable the **Bot** capability.
2. Copy App ID and App Secret into `FEISHU_APP_ID` / `FEISHU_APP_SECRET`. For Lark International set `FEISHU_BASE_URL=https://open.larksuite.com`.
3. Permission management → **批量导入** this JSON (`user` stays empty; the adapter uses `tenant_access_token`):

```json
{
  "scopes": {
    "tenant": [
      "im:message:send_as_bot",
      "im:message.p2p_msg:readonly",
      "im:message:readonly",
      "im:resource"
    ],
    "user": []
  }
}
```

- `im:message:send_as_bot` (以应用身份发消息): send text and interactive cards, and patch cards.
- `im:message.p2p_msg:readonly` (获取用户发给机器人的单聊消息): receive p2p `im.message.receive_v1`. Without this, the bot DM often has no input box.
- `im:message:readonly` (获取单聊、群组消息): download images/files from user messages.
- `im:resource` (获取与上传图片或文件资源): required on many tenants for the resource API. This is an advanced scope and may need admin approval. The adapter only downloads files into Memos; it does not upload media back to Feishu.

Permission JSON does **not** subscribe events. Events and callbacks are two separate pages; both must use **long connection**:

- **事件配置**: add `im.message.receive_v1` (接收消息). Until this is subscribed and published, the bot DM is often a blank chat with no composer.
- **回调配置**: add `card.action.trigger` (卡片回传交互, new). Do not use 消息卡片回传交互 (旧); the old callback cannot use long connection.

Then set the bot **availability range** to include your account (otherwise sends fail with `230013`), create a version, and **publish**. Console copy of scope names may change; match the scope IDs above.

Saving long connection sometimes requires this app to already have a live WebSocket. Put the credentials in `.env`, start memogram, then save the subscription in the console.

The Feishu adapter only handles **p2p** chats. Optional bot menu commands (`/start`, `/list`, `/tags`, `/search`) can be configured in the console; the binary does not call the menu API. Card callbacks should return within about 3 seconds (Memos on the same host is usually fine).

## Usage

### Starting the Service

#### Starting with binary

1. Download and extract the released binary, or build `./cmd/memos-channel`.
2. Create a `.env` file in the same directory as the binary.
3. Run:

   ```sh
   ./memogram
   ```

4. Talk to the bot in Telegram or Feishu.

#### Starting with Docker

1. Build the image: `docker build -t memogram .`
2. Run with environment variables:

   ```sh
   docker run -d --name memogram \
     -e SERVER_ADDR=dns:localhost:5230 \
     -e BASE_URL=https://your-memos.example \
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
   BASE_URL=https://your-memos.example
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

- `/start <access_token>`: Bind this chat user to a Memos access token.
- `/list`: Show the 10 most recently updated memos. Tap a row for the full note; use Prev/Next to page. From a note: Back, Edit, Delete, and Open when `BASE_URL` (or the instance https URL) is set.
- `/tags`: List tags, then the same paged list filtered to that tag.
- `/search <words>`: Same list card as `/list`, filtered by memo content.
- Send text: save as a memo. After Edit, the next text overwrites the open memo (`/cancel` aborts).
- Send files (photos, documents, voice, video): attach them to a memo.
- Public / Private / Pin buttons: update the saved memo.

Feishu uses the same commands. Adapter-owned labels (buttons, list titles, bind prompts) are Chinese; Core error strings such as `List expired. Send /list again.` stay English.
