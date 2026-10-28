# Campfire Event Manager

Create Pokémon GO / Campfire meetups from reusable templates. Discord login gates access (Community Ambassadors); your Campfire session JWT is forwarded for GraphQL calls on your behalf.

## Stack

- **Go** API + embedded Nuxt SPA (same shape as [campfire-map](https://github.com/topi314/campfire-map))
- **Discord OAuth** + guild/whitelist CA check (from [campfire-tools](https://github.com/topi314/campfire-tools))
- **Postgres** for Discord sessions, templates, drafts, and cover images ([sqlc](https://sqlc.dev) + pgx; regenerate with `sqlc generate` or `go generate` from the repo root)
- Campfire GraphQL at `https://niantic-social-api.nianticlabs.com/graphql`

## Features

1. Login with Discord (must be whitelisted or in a configured CA guild)
2. Paste Campfire session JWT (browser localStorage by default; optional encrypted account sync)
3. Build meetup templates on the **Templates** page (title, placeholders, clock times, location, invites)
4. **Publish** templates so others can find them under **Browse templates** (`/templates/browse`); add or customize a clone (always linked to the original); open publishers on **Profiles**
5. Create a meetup at **`/create`**: pick template → optionally pick live event → fill placeholders → create (or save a **club draft** as a club admin and post later from **`/create/drafts`** — shared with other admins of that club). Edit a draft at **`/create/drafts/:id`**.
6. Edit an existing meetup on **`/edit`**: pick club → pick upcoming meetup → tweak details → save. The events pencil opens **`/edit?clubId=&eventId=`** for that meetup.
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

Open [campfire.scopely.com/discover](https://campfire.scopely.com/discover) and sign in, then copy `CapacitorStorage.sessionToken` from Local Storage for `https://campfire.scopely.com`. Paste it into **Settings** (surrounding quotes are fine).

### Chrome
1. Press F12 (or right-click → Inspect) and open the **Application** tab
2. Expand **Local Storage** → select `https://campfire.scopely.com`
3. Copy the value of `CapacitorStorage.sessionToken` (`eyJ…`)

### Edge
1. Press F12 (or right-click → Inspect) and open the **Application** tab (use the **≫** menu if needed)
2. Expand **Local Storage** → select `https://campfire.scopely.com`
3. Copy the value of `CapacitorStorage.sessionToken` (`eyJ…`)

### Firefox
1. Press F12 (or right-click → Inspect) and open the **Storage** tab
2. Expand **Local Storage** → select `https://campfire.scopely.com`
3. Copy the value of `CapacitorStorage.sessionToken` (`eyJ…`)

### Safari
1. Enable the Develop menu if needed: **Safari → Settings → Advanced → Show features for web developers**
2. Choose **Develop → Show Web Inspector** (or ⌥⌘I), then open the **Storage** tab
3. Under **Local Storage**, select `https://campfire.scopely.com` and copy `CapacitorStorage.sessionToken` (`eyJ…`)

The token is kept in the browser (`campfire-event-manager.sessionToken`) and attached to Campfire API calls. By default the server does not store it. Settings can turn on account sync once per Discord user; the server then keeps an encrypted copy and other signed-in browsers use it.

## Meetup create fields

Create flow (home page):

1. **Club** — PGO clubs where you are an admin
2. **Live event** (optional) — when set, supplies the meetup day, `campfireLiveEventId`, and category (from the event name)
3. **Template** — filtered/auto-selected by live-event category when one is chosen
4. **Placeholders** — fill any `{{key}}` tokens
5. **Create** — tweak details and send `createPoiMeetup`

Times: template `startTime` / `endTime` (HH:mm) are combined with the live event’s date, or with today when no live event is linked. Location, invites, and copy come from the template. Cover photos are stored in our DB (`POST /api/covers`) and attached to Campfire only when creating/editing a meetup.

### Edit existing

Open **`/edit`**:

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

Use `{{key}}` or `{{key[1]}}` in title, description, address, or cover URL. Built-in keys (`club`, `liveEvent`, `category`, `title`, `address`, `lat`, `lng`, `date`, `dateShort`, `weekday`, `startTime`, `endTime`, `timezone`, plus event-Pokémon keys below) fill automatically when the meetup is **posted** to Campfire (and in Preview). Drafts keep the `{{…}}` tokens.

`{{date}}`, `{{startTime}}`, and `{{endTime}}` accept a time-format id (the same ids as Settings): `{{date:dmy-dot-24}}`, `{{startTime:mdy-12}}`, `{{endTime:dmy-h}}`. `{{date:id}}` is the date half. `{{startTime:id}}` and `{{endTime:id}}` are the clock half, and they share a clock for a given id. Examples below are for 18 Oct 2026 at 15:45. Without an id, `{{date}}` is `Oct 18, 2026`, `{{dateShort}}` is `2026-10-18`, and `{{startTime}}` / `{{endTime}}` are `15:45`. An unknown id falls back to that default.

| Id | `{{date:id}}` | `{{startTime:id}}` / `{{endTime:id}}` |
| --- | --- | --- |
| `mdy-12` | Oct 18, 2026 | 3:45 PM |
| `mdy-slash-12` | 10/18/2026 | 3:45 PM |
| `dmy-12` | 18 Oct 2026 | 3:45 pm |
| `dmy-slash-12` | 18/10/2026 | 3:45 pm |
| `dmy-24` | 18 Oct 2026 | 15:45 |
| `dmy-dot-24` | 18.10.2026 | 15:45 |
| `dmy-slash-24` | 18/10/2026 | 15:45 |
| `dmy-dash-24` | 18-10-2026 | 15:45 |
| `dmy-dot-time` | 18.10.2026 | 15.45 |
| `dmy-h` | 18 Oct 2026 | 15 h 45 |
| `ymd-24` | 2026-10-18 | 15:45 |
| `ymd-slash-24` | 2026/10/18 | 15:45 |
| `ymd-dot-24` | 2026.10.18 | 15:45 |
| `hm-dmy` | 18 Oct 2026 | 15:45 |
| `hm-dmy-slash` | 18/10/2026 | 15:45 |

`hm-dmy` uses the same pieces as `dmy-24`, and `hm-dmy-slash` uses the same pieces as `dmy-slash-24`. Settings shows those two with the clock before the date when both appear together.

**Event Pokémon** (category-gated extraction from the live event title; catalog + CP math on the server):

- `{{eventPokemon}}` — species name(s) translated to the meetup language (defaults from the template; overridable on create); editable when used
- Research: `{{eventPokemonCeilingResearch}}` / `{{eventPokemonFloorResearch}}` — 15/15/15 & 10/10/10 at L15
- Egg: `{{eventPokemonCeilingEgg}}` / `{{eventPokemonFloorEgg}}` — 15/15/15 & 10/10/10 at L20
- Raid: `{{eventPokemonCeilingRaid}}` / `{{eventPokemonFloorRaid}}` — 15/15/15 & 10/10/10 at L20
- Raid weather: `{{eventPokemonCeilingRaidWeather}}` / `{{eventPokemonFloorRaidWeather}}` — same IVs at L25
- Super mega raid: `{{eventPokemonFloorSuperMegaRaid}}` / `{{eventPokemonFloorSuperMegaRaidWeather}}` — 11/11/11 at L20 / L25
- Shadow raid: `{{eventPokemonFloorShadowRaid}}` / `{{eventPokemonFloorShadowRaidWeather}}` — 6/6/6 at L20 / L25
- Indexes: `{{eventPokemon[1]}}`, `{{eventPokemonCeilingRaid[2]}}`, … (1-based). Unindexed = full comma-joined list.
- Language pin (name only): `{{eventPokemon:de}}`, `{{eventPokemon[2]:fr}}`. Unscoped `{{eventPokemon}}` uses the meetup language. Any other code falls back to English. A regional code such as `zh-CN` uses `zh`.

These codes each have a Pokémon name list of their own.

| Code | Language |
| --- | --- |
| `de` | German |
| `en` | English |
| `es` | Spanish |
| `fr` | French |
| `it` | Italian |
| `ja` | Japanese |
| `ko` | Korean |
| `zh` | Chinese |

Any other key becomes a create-time field (**required** unless you set a default in `placeholders`). Prefer `startTime` / `endTime` (`HH:mm`) as template clock fields; legacy `eventTime` / `eventEndTime` still work (time-of-day only).

Template **language** is set when creating/editing a template (used for Browse filters and Pokémon name translation). Publishing reuses the stored language.

After placeholders are applied, title must be ≤ **120 characters** and description ≤ **1000 characters** to post (drafts may exceed this).

Optional `category` matches live-event categories (`Community Day`, `Raid Hour`, `Super Mega Raid Day`, `Shadow Raid Day`, …). On create, templates are filtered by the selected live event’s inferred category and a matching categorized template is auto-selected. Super Mega Raid Day and Shadow Raid Day fall back to Raid Day.

`locationJitterMeters` (default 25) randomly offsets the pin within that radius when creating so meetups don’t stack on the map. Set `0` for exact coordinates.

## License

Apache License 2.0 — see [LICENSE](LICENSE).
