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

## Database

Default is a **local PostgreSQL 17** on your own machine (ADR-5). Without `DATABASE_URL` the backend keeps scans in memory.

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
- **API says `database: none`**: `DATABASE_URL` is not set; check `deploy/.env`. The API then keeps scans only in memory.
- **`password authentication failed for user "checker"`**: the role or password in `deploy/.env` does not match; see "Database".
