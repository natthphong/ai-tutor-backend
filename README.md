# AI Tutor Loop — Backend

Go 1.25 + Fiber v2 backend for the AI Tutor Loop project (Thai → English
self-study tutor with LINE LIFF auth, Gemini/OpenRouter AI, MinIO storage,
and a parroto.app-style Shadowing Mode).

## Quick start

```bash
cd backend
cp config/config.example.yaml config/config.yaml   # edit DB, AI keys, MinIO
go run main.go                                     # listens on :8080
```

The server registers everything under `/{Server.Name}/api/v1`, default
`/ai-tutor/api/v1`. With the default config the frontend should point
`NEXT_PUBLIC_API_BASE_URL=http://127.0.0.1:8080/ai-tutor/api`.

## Database

Apply migrations in `sql/` order: `V1_table.sql`, `V002_tutor_tables.sql`,
`V7_data.sql`, `V9_cache.sql`, `V10_tutor_intelligence.sql`,
`V011_local_auth_and_shadowing.sql`. V011 adds:

- Local username/password fields on `tutor_users` (+ dev seed `test/test1234`)
- Shadowing tables: `shadowing_clips`, `shadowing_segments`,
  `shadowing_progress`, `shadowing_recordings`, `shadowing_notes`

Quick re-apply:
```bash
for f in sql/V*.sql; do psql -h localhost -U postgres -d ai_tutor -f "$f"; done
```

## Auth

Two flows live side-by-side:

| Path | Purpose |
|------|---------|
| `POST /v1/auth/line-login` | LINE LIFF (whitelist enforced) |
| `POST /v1/auth/line-refresh` | refresh access token |
| `GET  /v1/auth/line-me` | LINE profile (JWT) |
| `POST /v1/auth/register` | **Local** username/password signup |
| `POST /v1/auth/login` | **Local** login |
| `GET  /v1/auth/me` | Current user (JWT, works for both) |
| `POST /v1/auth/logout` | Stateless logout |

`tutor_users` rows have either `line_user_id`, `username`, or both.
`auth_kind='local'` users skip LINE notifications by design.

Dev seed for AI agents:
```
username: test
password: test1234
```

## Tutor evaluator (deterministic)

`internal/tutor/evaluator.go` replaces the old AI-dependent listening grader.

- `NormalizeAnswer` lowercases, trims, collapses spaces, strips terminal
  punctuation, normalises curly quotes and dashes, keeps intra-word
  apostrophes.
- `EvaluateAnswer(expected, user)` → `EvaluationResult` with:
  - `Score` 1.0 for normalised exact match, else weighted token-overlap +
    LCS order similarity in [0,1).
  - `MissingWords` / `ExtraWords` for hint/UX feedback.
  - `IsCorrect` only when `Score >= 0.95`.
- `BuildMaskedHint(target, level)` returns a deterministic mask that preserves
  word count, keeps apostrophes/hyphens, and reveals more letters with higher
  levels. Level 1 of `"Sarah is in her car"` → `S____ i_ i_ h__ c__`.
- `IsAnswerRevealRequest` triggers on `เฉลย`, `ขอคำตอบ`, `show answer`, …
  When matched, the tutor switches to `IntentRevealAnswer` and the AI is
  forced to reveal target + Thai meaning + grammar note (see prompt.go).

## Shadowing Mode

`/v1/shadowing/*` endpoints (see `handler/tutor/shadowing.go`). On
`POST /v1/shadowing/clips` the service:

1. Validates the YouTube URL (`ParseYouTubeID`).
2. Inserts a `shadowing_clips` row with `status='pending'`.
3. In a goroutine, tries `yt-dlp` for the 720p mp4, uploads to MinIO,
   asks Gemini to transcribe + segment + translate to Thai, then sets
   `status='ready'`.
4. If `yt-dlp` or Gemini is unavailable, it falls back to canned segments
   pointing at `youtube.com/embed/<id>` so the UI keeps working. Set
   `SHADOWING_LOCAL_FALLBACK=true` to force fallback.

The frontend polls `GET /v1/shadowing/clips/:clipId` until the clip becomes
`ready` and then renders the segment list / prev / next / replay / record /
notes UI under `/shadowing/:clipId`.

## Environment variables (most often overridden)

| Var | Purpose |
|-----|---------|
| `DATABASE_URL` | Postgres DSN (used by ops scripts, app reads `config.yaml`) |
| `JWT_SECRET` | HMAC secret for tutor JWT |
| `GEMINI_API_KEY` | primary LLM/TTS/STT |
| `OPENROUTER_API_KEY` / `OPENAI_API_KEY` | fallbacks |
| `MINIO_ENDPOINT`/`MINIO_ACCESS_KEY`/`MINIO_SECRET_KEY`/`MINIO_BUCKET`/`MINIO_PUBLIC_URL` | object storage |
| `LOCAL_AUTH_ENABLED` | always true today; the routes are mounted unconditionally |
| `SHADOWING_LOCAL_FALLBACK` | `true` to skip yt-dlp/Gemini |

## Running tests

```bash
cd backend
go test ./internal/tutor/... -v
```

Notable tests:
- `evaluator_test.go` — deterministic evaluator + masked hint + reveal regex.
- `brain_test.go` — intent classification.
- `decision_engine_test.go` — SRS decision tree.

## Running MinIO locally

```bash
docker run -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=minioadmin -e MINIO_ROOT_PASSWORD=minioadmin \
  quay.io/minio/minio server /data --console-address ":9001"
```
The backend will auto-create the bucket from `cfg.MinIO.Bucket` if missing.
