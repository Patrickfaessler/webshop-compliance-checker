# Webshop Compliance Checker (working title)

Software-Praktikum WS 2026/27, Gruppe 9 (Fässler/Kahraman).

Enter a shop URL, get a plain-language report on accessibility (WCAG 2.1 AA / EAA) and EU e-commerce obligations with
a concrete fix per problem. No accounts. Algorithms first, Claude only as fallback.

**Stack:** Flutter (web) + Go (both mandatory), headless Chromium via chromedp, axe-core, PostgreSQL (planned), Claude API (fallback, planned).

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
winget install Microsoft.WSL           # needed by Docker Desktop, reboot afterwards
winget install Docker.DockerDesktop
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

Database (from milestone M1): `docker compose -f deploy/docker-compose.yml up -d`.

## Working method

Spec first, test second, code third (`specs/README.md`). Every test cites its spec ID. Current status and next steps: `specs/10-roadmap.md`.

## License

Our own code is MIT-licensed (`LICENSE`). Embedded third-party code keeps its own license, see `THIRD_PARTY_NOTICES.md` (axe-core is MPL-2.0).
