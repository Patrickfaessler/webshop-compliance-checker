# Webshop Compliance Checker (working title)

Software-Praktikum WS 2026/27, Gruppe 9 (Fässler/Kahraman).

Enter a shop URL, get a plain-language report on accessibility (WCAG 2.1 AA / EAA) and EU e-commerce obligations with
a concrete fix per problem. No accounts. Algorithms first, Claude only as fallback.

**Stack:** Flutter (web) + Go (both mandatory), headless Chromium via chromedp, axe-core, PostgreSQL on Supabase (planned, ADR-5), Claude API (fallback, planned).

## Layout

| Path | Content |
| --- | --- |
| `specs/` | **source of truth**: read `specs/README.md` first |
| `backend/` | Go module `compliancechecker` (api, crawler, rules, scoring, worker) |
| `frontend/` | Flutter web app |
| `testcorpus/` | HTML cases with expected results per rule |
| `testshops/` | three fictional multi-page test shops (generator + generated site), published on GitHub Pages |
| `deploy/` | Docker Compose, `.env.example` |
| `infos` | original project sketch (V1) |

## Setup (Windows)

Install once (all via `winget`, except Flutter):

```powershell
winget install GoLang.Go
winget install Git.Git
winget install GitHub.cli
winget install Google.Chrome           # Flutter web dev; Edge also works for the backend crawler
winget install PostgreSQL.PostgreSQL.17 # local database, see "Database" below
winget install Microsoft.WSL           # only for Docker (WooCommerce test shop); needs virtualisation, reboot afterwards
winget install Docker.DockerDesktop    # optional until the WooCommerce test shop (M1)
git clone --depth 1 -b stable https://github.com/flutter/flutter.git C:\src\flutter
# then add C:\src\flutter\bin to your user PATH and open a new terminal
```

Android Studio and Visual Studio are **not** needed (web only). `flutter doctor` will still list them as missing; ignore that.

The backend crawler needs a Chromium-family browser: Edge (preinstalled on Windows), Chrome or Chromium. Set `CHROME_PATH` to override the auto-detection.

## Run the tests

```powershell
cd backend;  go vet ./...; go test ./...       # browser tests and the corpus run use Edge/Chrome if present
cd frontend; flutter pub get; flutter analyze; flutter test
```

## Run the app locally

```powershell
# terminal 1: API on :8080 (in-memory store, crawler in the same process)
cd backend; $env:CORS_ORIGIN = 'http://localhost:5000'; go run ./cmd/api

# terminal 2: Flutter web app on :5000
cd frontend; flutter run -d chrome --web-port 5000
```

## Deploy on a server (Docker + Traefik)

One compose file runs the app, the API (with Chromium) and the three test shops behind Traefik with Let's Encrypt.
You only enter your domain once.

1. DNS: point `check`, `teehaus`, `radwerk` and `seifen` (or a wildcard `*`) of your domain at the server. Open ports 80 and 443.
2. On the server: `git clone` the repo, then `cp deploy/.env.example deploy/.env` and set `DOMAIN=example.org` and `ACME_EMAIL=...`.
3. `docker compose -f deploy/docker-compose.server.yml up -d --build`

| Address | What |
| --- | --- |
| `https://check.<DOMAIN>` | the app (`/api` goes to the Go API) |
| `https://teehaus.<DOMAIN>`, `radwerk.`, `seifen.` | the test shops, expected results in `testshops/README.md` |

Update: `git pull`, then the same `up -d --build`. Logs: `docker compose -f deploy/docker-compose.server.yml logs -f api`.
Scans stay in memory (ADR-6), so no database is needed. Spec: `specs/07-architecture.md` (DEP-1).
If the server already runs Traefik, drop the `traefik` service and attach `app`, `api` and `shops` to its network
(and set `TRUSTED_PROXIES` to that network's subnet).

## Database

**Currently not needed (ADR-6):** scans are not stored. The backend keeps each result in memory for 1 hour and the user downloads it as PDF or JSON.
The PostgreSQL store below is kept for later and only used with `STORE=postgres`. The database tests still run when `TEST_DATABASE_URL` is set.

Setup, if you want it: a **local PostgreSQL 17** on your own machine (ADR-5).

```powershell
winget install PostgreSQL.PostgreSQL.17      # admin prompt; remember the superuser password
# create the app role and databases (psql asks for the superuser password)
$psql = "C:\Program Files\PostgreSQL\17\bin\psql.exe"
& $psql -U postgres -h localhost -c "CREATE ROLE checker LOGIN PASSWORD '<choose one>'"
& $psql -U postgres -h localhost -c "CREATE DATABASE checker OWNER checker"
& $psql -U postgres -h localhost -c "CREATE DATABASE checker_test OWNER checker"
```

Then copy `deploy/.env.example` to `deploy/.env` and fill in `DATABASE_URL` and `TEST_DATABASE_URL`. Never commit `deploy/.env`.

Alternatives: Docker (`docker compose -f deploy/docker-compose.yml --profile localdb up -d`, needs virtualisation) or a shared
Supabase project (session pooler string as `DATABASE_URL`). Only the Go backend ever connects; never put a database string into the Flutter app.

## Working method

Spec first, test second, code third (`specs/README.md`). Every test cites its spec ID. Current status and next steps: `specs/10-roadmap.md`.

## License

Our own code is MIT-licensed (`LICENSE`). Embedded third-party code keeps its own license, see `THIRD_PARTY_NOTICES.md` (axe-core is MPL-2.0).

## Troubleshooting

- **`An Application Control policy has blocked this file`** when running `go test`: Windows Smart App Control blocked a freshly
  built test binary in `%TEMP%`. It is not a code error. Build elsewhere:
  `New-Item -ItemType Directory -Force "$env:LOCALAPPDATA\gotest-tmp"; $env:GOTMPDIR = "$env:LOCALAPPDATA\gotest-tmp"; go test ./...`
- **Results are gone after a restart or after one hour**: intended (ADR-6). Download the PDF or JSON to keep a result.
- **`password authentication failed for user "checker"`**: the role or password in `deploy/.env` does not match; see "Database".
