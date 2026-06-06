# Knowledge Bot

A Slack bot that answers questions about your team's documentation using Gemini AI.

Users can mention the bot in any channel or send it a direct message. The bot reads markdown files from one or more configured sources and responds with accurate, grounded answers — always citing which source and page the information came from.

---

## How It Works

```
User asks a question in Slack
         ↓
Bot receives the message via Socket Mode (persistent WebSocket)
         ↓
Optional [source: name] prefix is parsed from the question
         ↓
Matching source(s) loaded as context into the prompt
         ↓
Gemini answers based strictly on the documentation
         ↓
Bot replies in-thread on Slack, citing which source each answer came from
```

Documentation is refreshed every 24 hours. For git-backed sources, a `git pull` is run automatically. For plain directories (not git repos), the files are simply re-read.

---

## Multiple Sources

The bot supports any number of documentation sources. Each source is a directory containing `.md` files — it can be a GitHub wiki clone, an internal docs folder, or any plain directory.

Configure sources in `.env` as a comma-separated list of `name:/path` pairs:

```env
WIKI_PATHS=service-wiki:/path/to/service-monorepo.wiki,playbook:/path/to/team-playbook
```

### Targeting a specific source

Prefix your question with `[source: <name>]` to query only that source:

```
@YourBot [source: service-wiki] how does the auth service work?
@YourBot [source: playbook] what's the on-call rotation process?
```

### Searching all sources

Omit the prefix to search everything. The bot will answer from whichever sources are relevant and cite each one:

```
@YourBot how do I set up a local environment?
```

The response will note which source and page each piece of information came from (e.g. *"According to the Setup page in service-wiki..."*).

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

With fewer than ~100 wiki pages across all sources, the total content fits comfortably within the model's context window. Full context was chosen because:

- The model sees every page on every request — no risk of missing something due to poor search ranking
- Zero infrastructure to manage (no vector DB, no embedding pipeline)
- Trivially simple to reason about and debug

If the combined documentation grows significantly (200+ pages), switching to RAG is the natural next step.

### Refresh strategy per source type

Rather than a single refresh strategy, the bot adapts per source:

- **Git repositories** — `git pull` is run on the configured interval. Zero-dependency, no API rate limits, atomic (all changes from one edit land together).
- **Plain directories** — files are re-read directly. Useful for docs that are managed outside of git (shared folders, mounted volumes, etc.).

If a source is unavailable at refresh time, the bot logs the error and continues serving the last successfully loaded content.

### Model choice: Gemini Flash Lite

The bot uses `gemini-2.0-flash-lite` by default — fast and cost-efficient. For Q&A over structured documentation this produces high-quality answers. To use a more capable model, swap the model name in `internal/claude/client.go`.

---

## Project Structure

```
Knowledge-bot/
├── main.go                   # Entry point — parses WIKI_PATHS, wires sources, Gemini, and Slack
├── internal/
│   ├── wiki/
│   │   └── loader.go         # Loads .md files from multiple sources, refreshes periodically
│   ├── claude/
│   │   └── client.go         # Wraps the Gemini SDK, builds the prompt
│   └── slack/
│       └── handler.go        # Socket Mode event loop, parses [source:] prefix, handles mentions and DMs
├── .env.example              # Template for environment variables
├── go.mod
└── go.sum
```

---

## Prerequisites

- Go 1.21+
- One or more directories containing `.md` files (git wiki clones or plain folders)
- A [Gemini API key](https://aistudio.google.com)
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

Fill in the values in `.env`:

```env
WIKI_PATHS=service-wiki:/path/to/service-monorepo.wiki,neuron-wiki:/path/to/neuron-env-wiki
GEMINI_API_KEY=your-gemini-api-key
SLACK_BOT_TOKEN=xoxb-...
SLACK_APP_TOKEN=xapp-...
```

**2. Run**

```bash
go run .
```

You should see:

```
source "service-wiki" loaded
source "neuron-wiki" loaded
knowledge bot starting with sources: service-wiki, neuron-wiki
connecting to slack...
connected to slack via socket mode
```

**3. Talk to the bot**

- **Search all sources**: `@YourBot how does credit scoring work?`
- **Target a source**: `@YourBot [source: neuron-wiki] what are the CF environments?`
- **Direct message**: just send a message directly to the bot (source prefix works here too)

---

## Configuration

| Variable          | Description                                                                 |
| ----------------- | --------------------------------------------------------------------------- |
| `WIKI_PATHS`      | Comma-separated list of `name:/path` pairs — git repos or plain directories |
| `GEMINI_API_KEY`  | Your Gemini API key from [aistudio.google.com](https://aistudio.google.com) |
| `SLACK_BOT_TOKEN` | Bot token from OAuth & Permissions (`xoxb-...`)                             |
| `SLACK_APP_TOKEN` | App-level token for Socket Mode (`xapp-...`)                                |

To change the refresh interval, edit `main.go`:

```go
loader.StartRefresh(24 * time.Hour) // change this
```

To use a different model, edit `internal/claude/client.go`:

```go
model: "gemini-2.0-flash-lite", // swap to e.g. "gemini-2.0-flash"
```

---

## Next Steps

### Hosting

The bot requires no inbound ports (Socket Mode handles connectivity), so it works on any machine with outbound internet access. Options:

- A small VM on AWS EC2, GCP, or DigitalOcean
- [Railway](https://railway.app) or [Fly.io] for managed containers
- Run as a systemd service on a self-hosted server

### Smarter responses

- Add a fallback message that links to the relevant wiki page URL when the bot cites a page
- Include the page's last-modified git date in the context so the model can note how recent the information is

### Scale to a larger wiki

If the combined documentation grows beyond ~200 pages and responses slow down or costs increase, migrate to a RAG architecture:

1. Embed each page with an embedding model
2. Store vectors in a lightweight store (pgvector, Qdrant, or Chroma)
3. At query time, retrieve the top-K most relevant pages per source and send only those as context
