# Knowledge Bot

A Slack bot that answers questions about your team's GitHub wiki using Claude AI.

Users can mention the bot in any channel or send it a direct message. The bot reads your wiki documentation and responds with accurate, grounded answers — citing the relevant wiki pages.

---

## How It Works

```
User asks a question in Slack
         ↓
Bot receives the message via Socket Mode (persistent WebSocket)
         ↓
All wiki pages are loaded as context into the prompt
         ↓
Claude answers based strictly on the wiki content
         ↓
Bot replies in-thread on Slack
```

The wiki content is refreshed every 10 minutes by running `git pull` on the local wiki clone, so the bot stays in sync with edits made on GitHub without any webhooks or external triggers.

---

## Design Choices

### Socket Mode (no public URL needed)

Slack bots can receive events in two ways: via an HTTP webhook (Slack pushes to your server), or via Socket Mode (your bot opens a persistent WebSocket connection to Slack).

Socket Mode was chosen because:

- No need for a publicly reachable server or domain during development
- Works behind firewalls and on local machines
- Easier to get started — no reverse proxy, no SSL cert, no ngrok

When you decide on hosting, Socket Mode works there too without any changes.

### Full-context loading (no vector database)

There are two common approaches to give an AI knowledge of a document set:

**RAG (Retrieval Augmented Generation)** — embed documents into a vector database, search for relevant chunks at query time, send only those chunks to the model. Required for large corpora but adds significant complexity.

**Full context** — load all documents directly into the system prompt on every request. Simpler, and often more accurate since the model sees everything.

With fewer than 50 wiki pages, the total content fits comfortably within Claude's context window. Full context was chosen because:

- Claude sees every page on every request — no risk of missing something due to poor search ranking
- Zero infrastructure to manage (no vector DB, no embedding pipeline)
- Trivially simple to reason about and debug

If the wiki grows significantly (200+ pages), switching to RAG is the natural next step.

### `git pull` for sync

GitHub wikis are git repositories. Rather than polling the GitHub API or setting up webhooks, the bot simply runs `git pull` on the locally cloned wiki repo every 24 hours. This is:

- Zero-dependency (just git)
- No API rate limits
- Atomic — all page changes from a single edit land together
- Easy to tune (change the interval in `main.go`)

The tradeoff is a small staleness window (up to 10 minutes). For a knowledge bot, this is acceptable.

### Model choice: Claude Haiku

The bot uses `claude-haiku-4-5` by default — Anthropic's fastest and most cost-efficient model. For Q&A over structured documentation, Haiku produces high-quality answers at a fraction of the cost of larger models. If you need more complex reasoning or synthesis, swap to `claude-sonnet-4-6` in `internal/claude/client.go`.

---

## Project Structure

```
Knowledge-bot/
├── main.go                   # Entry point — wires wiki, Claude, and Slack together
├── internal/
│   ├── wiki/
│   │   └── loader.go         # Reads .md files from the wiki repo, refreshes periodically
│   ├── claude/
│   │   └── client.go         # Wraps the Anthropic SDK, builds the prompt
│   └── slack/
│       └── handler.go        # Socket Mode event loop, handles mentions and DMs
├── .env.example              # Template for environment variables
├── go.mod
└── go.sum
```

---

## Prerequisites

- Go 1.21+
- A local clone of your GitHub wiki repo
- An [Anthropic API key](https://console.anthropic.com)
- A Slack workspace where you can create apps

---

## Slack App Setup

1. Go to [api.slack.com/apps](https://api.slack.com/apps) and click **Create New App → From scratch**.
2. **Enable Socket Mode**

   - Sidebar: *Socket Mode* → toggle on
   - Generate an **App-Level Token** with the `connections:write` scope
   - Save this token — it starts with `xapp-`
3. **Set Bot Token Scopes**

   - Sidebar: *OAuth & Permissions → Scopes → Bot Token Scopes*
   - Add: `app_mentions:read`, `chat:write`, `im:history`, `im:read`
4. **Subscribe to Events**

   - Sidebar: *Event Subscriptions* → toggle on
   - Under *Subscribe to bot events*, add: `app_mention`, `message.im`
5. **Install to Workspace**

   - Sidebar: *OAuth & Permissions* → *Install to Workspace*
   - Copy the **Bot Token** — it starts with `xoxb-`
6. **Invite the bot to channels** where you want it to respond to mentions.

---

## Running the Bot

**1. Clone this repo and set up your environment**

```bash
cp .env.example .env
```

Fill in the four values in `.env`:

```env
WIKI_PATH=/path/to/your/local/wiki/clone
ANTHROPIC_API_KEY=sk-ant-...
SLACK_BOT_TOKEN=xoxb-...
SLACK_APP_TOKEN=xapp-...
```

**2. Run**

```bash
go run .
```

You should see:

```
wiki loaded: 22 pages
knowledge bot starting...
connecting to slack...
connected to slack via socket mode
```

**3. Talk to the bot**

- **In a channel**: `@YourBot how does credit scoring work?`
- **Direct message**: just send a message directly to the bot

The bot replies in-thread with an answer grounded in your wiki content.

---

## Configuration

| Variable            | Description                                       |
| ------------------- | ------------------------------------------------- |
| `WIKI_PATH`       | Absolute path to your local wiki git clone        |
| GEMINI_API_KEY      | Your Model API key                               |
| `SLACK_BOT_TOKEN` | Bot token from OAuth & Permissions (`xoxb-...`) |
| `SLACK_APP_TOKEN` | App-level token for Socket Mode (`xapp-...`)    |

To change the wiki refresh interval, edit `main.go`:

```go
loader.StartRefresh(10 * time.Minute) // change this
```

To use a more powerful model, edit `internal/claude/client.go`:

```go
Model: "gemini-3.1-flash-lite",
```

---

## Next Steps

### Hosting

To run this continuously, deploy it to any server or cloud provider. The bot requires no inbound ports (Socket Mode handles connectivity), so it works on any machine with outbound internet access. Options:

- A small VM on AWS EC2, GCP, or DigitalOcean
- [Railway](https://railway.app) or [Fly.io] for managed containers
- Run as a systemd service on a self-hosted server

### Smarter responses

- Add a fallback message that links to the relevant wiki page URL when Claude cites a page
- Include the page's last-modified git date in the context so Claude can note how recent information is

### Scale to a larger wiki

If the wiki grows beyond ~100 pages and responses slow down or costs increase, migrate to a RAG architecture:

1. Embed each page with Anthropic's embedding API or OpenAI
2. Store vectors in a lightweight store (pgvector, Qdrant, or Chroma)
3. At query time, retrieve the top-K most relevant pages and send only those as context

### Multi-wiki support

The `wiki.Loader` can be extended to load from multiple directories — useful if knowledge is spread across more than one wiki repo.
