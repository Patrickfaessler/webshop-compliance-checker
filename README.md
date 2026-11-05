# Webshop Compliance Checker (working title)

Software-Praktikum WS 2026/27, Gruppe 9 (Fässler/Kahraman).

Enter a shop URL, get a plain-language report on accessibility (WCAG 2.1 AA / EAA) and EU e-commerce obligations with
a concrete fix per problem. No accounts. Algorithms first, Claude only as fallback.

**Stack:** Flutter (web) + Go (both mandatory), PostgreSQL, Playwright/Chromium, axe-core, Claude API (fallback).

## Layout

| Path | Content |
| --- | --- |
| `specs/` | **source of truth**: read `specs/README.md` first |
| `backend/` | Go module `compliancechecker` (api, worker, rules, scoring) |
| `frontend/` | Flutter web app |
| `testcorpus/` | HTML cases with expected results |
| `deploy/` | Docker Compose, `.env.example` |
| `infos` | original project sketch (V1) |

## Prerequisites (not yet installed on this machine)

- Go ≥ 1.23, Flutter (stable, with web enabled), Git, Docker Desktop

## First steps once installed

```powershell
# Frontend: generate the platform folder once (not committed by hand)
cd frontend; flutter create . --platforms web --project-name compliance_checker; flutter pub get; flutter test

# Backend
cd backend; go vet ./...; go test ./...

# Database
docker compose -f deploy/docker-compose.yml up -d
```

## Working method

Spec first, test second, code third (`specs/README.md`). Start with `specs/10-roadmap.md` M0, then rule A6 (`specs/rules/A6-html-lang.md`).
