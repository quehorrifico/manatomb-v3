# Mana Tomb

Mana Tomb is a hobby web application for *Magic: The Gathering* Commander players.  
Search for cards using the Scryfall API, build and edit Commander decks, and explore various gameplay tools as the project evolves.

This project also serves as a full‑stack learning environment focused on **Go**, **PostgreSQL**, **TailwindCSS**, and maintainable backend architecture.

> Mana Tomb is a non-commercial project. All card data and images are provided by Scryfall.

---

## Features

- Card search powered by Scryfall  
- Commander search and quick “Use as commander” flow  
- Deck builder (create, edit, add/remove cards)  
- User accounts and session-based authentication  
- TailwindCSS dark UI theme  
- Dockerized Go backend deployed on DigitalOcean  
- PostgreSQL database schema for users, decks, cards, and sessions  

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

```
MT_DB_DSN=postgres://username:password@localhost:5432/manatomb?sslmode=disable
MT_SESSION_KEY=dev-session-key-change-me
SCRYFALL_BASE_URL=https://api.scryfall.com
PORT=8080
```

### 3. Start PostgreSQL (example using Docker)

```
docker run --name manatomb-db \
  -e POSTGRES_PASSWORD=password \
  -p 5432:5432 \
  -d postgres:15
```

### 4. Run the server

```
go run ./cmd/server
```

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

- Environment variables configure the app (no secrets in code).  
- Backend connects to a managed PostgreSQL instance.  
- The server ensures required database tables exist on startup.  
- Deployments are triggered from changes to the `main` branch.

Sensitive credentials, connection strings, and production notes should be kept in a private ops document outside this repository.

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
