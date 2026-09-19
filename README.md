# Campfire Event Manager

Create Pokémon GO / Campfire meetups from reusable templates. Discord login gates access (Community Ambassadors); your Campfire session JWT is forwarded for GraphQL calls on your behalf.

## Stack

- **Go** API + embedded Nuxt SPA (same shape as [campfire-map](https://github.com/topi314/campfire-map))
- **Discord OAuth** + guild/whitelist CA check (from [campfire-tools](https://github.com/topi314/campfire-tools))
- **Postgres** for Discord sessions and per-user meetup templates
- Campfire GraphQL at `https://niantic-social-api.nianticlabs.com/graphql`

## Features

1. Login with Discord (must be whitelisted or in a configured CA guild)
2. Paste Campfire session JWT (browser localStorage only — not stored on the server)
3. Build meetup templates on the **Templates** page (title, placeholders, clock times, location, invites)
4. **Publish** templates so others can find them under **Browse templates** (`/templates/browse`); add or customize a clone (always linked to the original); open publishers on **Profiles**
5. Create a meetup at **`/create`**: pick template → optionally pick live event → fill placeholders → create (or save a **club draft** as a club admin and post later from **`/create/drafts`** — shared with other admins of that club). Edit a draft at **`/create/drafts/:id`**.
6. Edit an existing meetup on **`/create`**: pick club → pick upcoming meetup → tweak details → save
7. Manage templates at **`/templates`**, create at **`/templates/new`**, edit at **`/templates/:id/edit`**, browse at **`/templates/browse`** (open a listing at **`/templates/browse/:id`**)

When a live event is selected it supplies the **calendar day** and `campfireLiveEventId`; start/end **hours** come from the template. Without a live event, set the date and time yourself (template clocks use today).

## Setup

### 1. Discord application

1. Create an app at [Discord Developer Portal](https://discord.com/developers/applications)
2. Add a redirect URL: `{public_url}/auth/login/callback` (e.g. `http://localhost:8080/auth/login/callback`)
3. Enable OAuth2 scopes used by the app: `identify`, `guilds.members.read`
4. Copy client ID and secret into `config.toml`

### 2. Config

```bash
cp example.config.toml config.toml
```

Edit:

- `[server].public_url` — must match the OAuth redirect base
- `[server].app_url` — SPA origin for post-login redirects (use `http://localhost:3000` with Nuxt dev; omit when Go serves the embedded UI)
- `[discord_auth].client_id` / `client_secret`
- `[discord_auth].guild_ids` — CA Discord server ID(s)
- `[discord_auth].whitelist` — optional Discord user IDs that bypass guild check
- `[database]` — Postgres connection (defaults match `compose.yml`)
- `[basemaps].carto_api_key` — optional CARTO key for watermark-free Voyager tiles (same as campfire-map)

### 3. Run with Docker

```bash
cp example.config.toml config.toml
# set [database].host = "db" and fill Discord credentials
docker compose up --build
```

Open http://localhost:8080

### 4. Run locally (dev)

Start Postgres (or `docker compose up db -d`), then:

```bash
cp example.config.toml config.toml
# point database host at localhost

go run -tags dev . -config config.toml
```

`-tags dev` skips embedding `frontend/dist` so you don’t need a generated SPA for the API.

Frontend (hot reload):

```bash
cd frontend
npm install
npm run dev
```

Open http://localhost:3000 — `/api` and `/auth` are proxied to `:8080`.
After Discord login the API redirects to `app_url` (e.g. `http://localhost:3000/create`), not the API host.

### 5. Embed frontend into Go (optional)

Production builds (no `-tags dev`) embed `frontend/dist`. Generate assets first:

```bash
cd frontend
NUXT_PUBLIC_API_BASE= npm run generate
# PowerShell:
Remove-Item -Recurse -Force .\dist\* -ErrorAction SilentlyContinue
Copy-Item -Recurse .output\public\* .\dist\

cd ..
go build -o campfire-event-manager .
```

Docker builds without `-tags dev` and embeds the generated SPA. With `-tags dev`, non-API routes return 404 — use the Nuxt dev server for the UI.

## Campfire JWT

1. Open [campfire.nianticlabs.com](https://campfire.nianticlabs.com/) and sign in
2. Press F12 (or right-click → Inspect). Open **Application** in Chrome or Edge, or **Storage** in Firefox
3. Under **Local Storage**, select `https://campfire.nianticlabs.com`
4. Copy the value of `CapacitorStorage.sessionToken` (`eyJ…`)
5. Paste it into **Settings** in this app (surrounding quotes and a `Bearer` prefix are both fine)

The token is kept in the browser (`campfire-event-manager.sessionToken`) and sent as `Authorization: Bearer …` on Campfire API calls. The server never persists it.

## Meetup create fields

Create flow (home page):

1. **Club** — clubs you can create meetups in
2. **Live event** (optional) — when set, supplies the meetup day, `campfireLiveEventId`, and category (from the event name)
3. **Template** — filtered/auto-selected by live-event category when one is chosen
4. **Placeholders** — fill any `{{key}}` tokens
5. **Create** — tweak details and send `createPoiMeetup`

Times: template `startTime` / `endTime` (HH:mm) are combined with the live event’s date, or with today when no live event is linked. Location, invites, and copy come from the template. Cover photos are uploaded via Campfire `uploadFile` and can be removed via `DELETE /api/campfire/upload-image?photoId=…`.

### Edit existing

Switch to **Edit existing** on **`/create`**:

1. **Club** — same list as create
2. **Meetup** — upcoming/ongoing meetups from the club’s active feed
3. **Template** (optional) — overlay title, details, times (clock on the meetup’s day), location, cover, etc.
4. **Save** — updates via Campfire `editEvent`


## Template JSON

Upload either a bare payload:

```json
{
  "name": "Community Day — {{city}}",
  "details": "Meet at {{meetupSpot}}. Bring gifts!",
  "clubId": "…",
  "startTime": "14:00",
  "endTime": "17:00",
  "category": "Community Day",
  "latitude": 50.1,
  "longitude": 8.76,
  "allInvited": true,
  "placeholders": [
    { "key": "city", "label": "City name" },
    { "key": "meetupSpot", "label": "Meetup spot", "default": "central park" }
  ]
}
```

or a wrapped file:

```json
{
  "name": "Community Day",
  "payload": { "name": "…", "details": "…" }
}
```

Use `{{key}}` in title, description, address, or cover URL. Built-in keys (`club`, `liveEvent`, `category`, `date`, `dateShort`, `weekday`, `startTime`, `endTime`, `timezone`) fill automatically when the meetup is **posted** to Campfire (and in Preview). Drafts keep the `{{…}}` tokens. Any other key becomes a create-time field; set optional `placeholders` entries for labels/defaults. Prefer `startTime` / `endTime` (`HH:mm`) as template clock fields; legacy `eventTime` / `eventEndTime` still work (time-of-day only).

Optional `category` matches campfire-tools live-event categories (`Community Day`, `Raid Hour`, …). On create, templates are filtered by the selected live event’s inferred category and a matching categorized template is auto-selected.

`locationJitterMeters` (default 25) randomly offsets the pin within that radius when creating so meetups don’t stack on the map. Set `0` for exact coordinates.

## License

Apache License 2.0 — see [LICENSE](LICENSE).
