# Mana Tomb

Mana Tomb is a hobby web application for *Magic: The Gathering* players.
Search cards from a local Scryfall-powered database, build decks across formats, publish public deck pages, and explore gameplay tools as the project evolves.

This project also serves as a full‑stack learning environment focused on **Go**, **PostgreSQL**, **TailwindCSS**, and maintainable backend architecture.

> Mana Tomb is a non-commercial project. All card data and images are provided by Scryfall.

---

## Features

- Card search with local canonical card data and print-version browsing
- Commander search plus local deck creation for other MTG formats
- Deck builder with saved decks, guest decks, maybeboard support, and import flows
- Public deck publishing, browse, detail pages, and fork-to-account flow
- Deck analytics with format-aware validation warnings
- Goldfish playtest with mulligans, drag/drop zones, coin flip, dice roll, and token creation
- User accounts, session-based authentication, and account settings
- Dockerized Go backend with PostgreSQL persistence

---

## Tech Stack

- **Backend:** Go (net/http, html/template), modular monolith  
- **Database:** PostgreSQL  
- **Frontend:** Go templates + TailwindCSS  
- **Data Source:** Scryfall API  
- **Deployment:** Docker, DigitalOcean App Platform  

---

## Running Locally

### 1. Clone the repository

```
bash
git clone https://github.com/zeusborrego/manatomb-v3.git
cd manatomb-v3
```

### 2. Create a `.env` file

```
cp .env.example .env
```

Example values:

```env
DATABASE_URL=postgres://username:password@localhost:5432/manatomb?sslmode=disable
SESSION_SECRET=dev-session-secret-change-me
SESSION_COOKIE_SECURE=false
CARD_SYNC_ENABLED=true
CARD_SYNC_ON_START=false
PORT=8080
```

### 3. Start PostgreSQL (example using Docker)

```
docker run --name manatomb-db \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=password \
  -e POSTGRES_DB=manatomb \
  -p 5432:5432 \
  -d postgres:15
```

### 4. Run the server

```
go run ./cmd/server
```

The app reads `.env` automatically when you run it from the repo root.

To force a full Scryfall bulk sync immediately on startup:

```
go run ./cmd/server --sync-now
```

Sync behavior:

- The app schedules a bulk sync every 24 hours from process start.
- The app also runs an immediate startup sync automatically when local card data is missing or on an older app data version.
- You can force an immediate startup sync with `--sync-now` or `CARD_SYNC_ON_START=true`.

Navigate to:

```
http://localhost:8080
```

---

## Running with Docker

```
docker build -t manatomb .
docker run -p 8080:8080 --env-file .env manatomb
```

---

## Deployment Overview

Mana Tomb is deployed on DigitalOcean App Platform.

- Environment variables configure the app.
- Backend connects to a managed PostgreSQL instance.  
- The server ensures required database tables exist on startup.  
- Deployments are triggered from changes to the `main` branch.
- Health checks can target `GET /healthz`.

Sensitive credentials and connection strings should be kept outside this repository.

---

## Scryfall Attribution

Mana Tomb uses the Scryfall API for card data and images.

- Scryfall API: https://scryfall.com/docs/api  
- Card data © Scryfall  
- Card images © Wizards of the Coast  
- Required: project must remain free and non-commercial  

---

## Contributions

This is a personal learning project, but feedback and suggestions are welcome.
