# Toko Loop — backend

Go/Fiber + PostgreSQL speaking tutor for Thai adults, powered only by Gemini. New API: `/ai-tutor/api/v2`. The frontend repository lives beside this one and uses a Next.js server proxy for Safari-compatible session cookies.

## Run locally

Requires Go 1.26, PostgreSQL 17+, and `ffmpeg` on PATH.

```sh
cp .env.example .env.local
# Set a NEW database containing `toko` in its name, and GEMINI_API_KEY.
set -a
source .env.local
set +a
go run .
```

The migration deliberately rejects database names without `toko`. It never connects to or erases the previous application's database. On a new database only, bootstrap creates `admin / password`. Change this password before AI or invitations can be used. Restarting or deploying does not reset the account. Admin can issue/revoke single-use invitations, valid for 7 days. Passwords use Argon2id; bearer session tokens and invitations are stored as SHA-256 hashes. Browser tokens stay in an HttpOnly, Secure, SameSite=Lax frontend cookie; the browser never receives the Gemini key.

## Learning system

- 100 original core lessons: Pre-A1, A1, A2, B1, B2-oriented; 20 lessons at each stage, four units of five.
- 50 work simulations across Tech, Banking, Business, Interview, Meeting; 20 additional everyday simulations for ordering food, directions, shopping, travel, friends, appointments and problem-solving.
- Pattern → four drills → two independent transfer tasks → completion. Text can practice meaning but cannot establish speaking mastery. Assisted answers and corrected retries are separate from independent successes.
- Hints: idea → keyword → pattern → sentence. Thai ideas can be supplied. Hint use, recent attempts and response times inform subsequent tutoring.
- Correctness and goal completion are separate. Optional natural/professional phrasing never automatically becomes a grammar failure. Missing or invalid Gemini assessment schemas are rejected.
- Spoken mistakes are deduplicated by stable error category. Reviews progress through 1, 3, 7, 14, 30, 60 days; mistakes return the next day, hints hold the stage. A failed review can be retried in the same session. Typed reviews do not advance the speaking schedule.
- Scenario drafts use the current lesson and recurring weaknesses. Learners edit the draft before playing. Completed scenes generate a goal-based summary and a separate retry session; original results remain intact.
- Level labels organize practice difficulty and are **not CEFR certification**. Completing these exercises does not guarantee fluency; varied independent speech and real conversations remain necessary.

Curriculum sources: `scripts/build_content.py`, then `scripts/enrich_content.py`. Generated, versioned JSON lives in `internal/content/`. No AI bill is incurred for seeding the curriculum.

## Audio and Live

Turn audio is decoded with FFmpeg to mono PCM16/16kHz. Only pipe input protocols are allowed; corrupt, very short and overlong recordings are rejected before assessment. Pronunciation is qualitative and requires actual clear audio. Silent/unintelligible input produces a re-record request, not a language weakness.

Live uses a one-use, 30-second WebSocket ticket and exact Origin allowlist. The backend proxies Gemini Live; audio enters as binary PCM16/16kHz and leaves as Gemini 24kHz PCM events. A session allows one active connection per learner, has daily/minute limits, 15-second frontend heartbeats, a silence timeout, cost checks and explicit stop. Recent persisted turns restore conversation context on reconnect or switching to turn mode. Interruption clears queued browser audio. Backgrounding pauses the microphone and Live connection.

`AUDIO_DIR` must be a persistent volume. Learner/Live recordings expire after 30 days and are deleted by the worker; transcript, feedback and progress remain. Lesson/response TTS is generated on demand and privately cached by user, text, voice, model, language and config version. File endpoints check ownership and support HTTP Range playback.

## Configuration and costs

Edit `config/models.yaml`; credentials come from environment. Each profile defines Gemini model ID, capability, generation limits, thinking options and separate text/audio prices. A fallback must be another explicitly configured, priced Gemini profile with the same capability. Changing providers is not supported.

Defaults checked against [Gemini pricing](https://ai.google.dev/gemini-api/docs/pricing) and [deprecations](https://ai.google.dev/gemini-api/docs/deprecations) on 2026-09-05:

| Role | Model | Text input/output USD per 1M | Audio input/output USD per 1M |
|---|---|---|---|
| Helper | gemini-2.5-flash-lite | 0.10 / 0.40 | 0.30 / — |
| Tutor, feedback, scenario | gemini-3.1-flash-lite | 0.25 / 1.50 | 0.50 / — |
| TTS | gemini-2.5-flash-preview-tts | 0.50 / — | — / 10 |
| Live | gemini-3.1-flash-live-preview | 0.75 / 4.50 | 3 / 12 |

Default budget 600 THB/month, configurable up to 1,000; total application protection also caps 1,000 THB/month. Default exchange rate 35 THB/USD is a configurable accounting assumption. Reserve funds before calls, settle by modality from usage metadata, show 80% warnings and stop new AI work at the ceiling. Missing usage is explicitly estimated, including duration-based estimates for interrupted Live responses. These figures are app estimates, not invoices. History and cached audio remain accessible when the budget is exhausted.

The four engines are orchestrated in a single tutor call where possible; review scheduling, progress, deduplication and authorization run in Go. Original lessons/static hints need no model call; PostgreSQL stores queued TTS/scenario/summary jobs with bounded retries and idempotency keys.

## API contract

`contracts/openapi.json` is the canonical public contract, also served at `/ai-tutor/api/v2/openapi.json`. In frontend run `npm run generate:api` to regenerate TypeScript types. All practice answers require `request_id` UUIDs. Never generate a fresh UUID when retrying the same network submission.

## Verification

```sh
go test -race -timeout 90s ./...
go vet ./...
# Integration refuses any database without `_test` in its name and resets that test database only:
TEST_DATABASE_URL='postgres://toko:test-only@localhost:5432/toko_loop_test?sslmode=disable' go test -race ./internal/app
# Opt-in paid Gemini regression; synthetic/text fixtures, estimate capped at 5 THB:
go run ./cmd/eval
# Opt-in Live smoke test using credentials from ignored .toko-qa.json:
go run ./cmd/livecheck
```

Real Gemini evaluation and smoke evidence are in `reports/`. Evaluation includes valid alternatives, article errors, optional professional wording, Tech/Banking scenarios, off-topic answers, instruction injection and silence. Synthetic examples are not a substitute for a Thai-speaker pronunciation benchmark or physical iPhone/Bluetooth testing. See the frontend README for browser screenshots and release verification.

## Home server deployment

`bash deploy_local.sh` runs tests/vet, builds a Linux Docker image, uploads it through the existing Portainer endpoint, creates a **new isolated** PostgreSQL database/volume and an audio volume, and checks a candidate before switching the exact named app container. It stops if another app owns the requested port, network or volume. It retains the previous app container and automatically restores it if readiness or public HTTPS verification fails. It never commits/pushes Git or modifies unrelated containers.

Required `.env.deploy` values: `PORTAINER_URL`, `PORTAINER_API_KEY`, `ENDPOINT_ID`, `GEMINI_API_KEY`; optional `APP_NAME`, `EXTERNAL_PORT`, `RELEASE_ID`, `PUBLIC_BACKEND_URL`, `ALLOWED_ORIGINS`. Existing `.env` deployment credentials can be reused, but old AI-provider/config payloads are unused. No secrets are copied into the image.

Production target format: `https://api.example.com/ai-tutor/api/v2/health`. Cloudflare must route the configured API hostname to the configured external port and permit HTTPS/WSS. The check sends a named application User-Agent. Release metadata is saved locally in ignored `.toko-deploy-state.json`; use the previous container recorded there for a manual rollback if necessary. Back up PostgreSQL and the audio volume together. Do not use `docker volume prune` as part of deployment.

## Cost and queue update (2026-09-05)

Tutor requests send a compact lesson rubric and 8 recent turns (1000 characters each), with 2048 output tokens; helpers cap at 512. TTS timeout is 75 seconds. Ambiguous timeouts are not automatically replayed; explicit transient rejection retries at most once. Two bounded job workers avoid blocking scenario summaries behind audio. Unknown TTS usage is estimated from input length and audio token rates, while explicit rejected calls settle at zero. Reservations remain conservative hard-budget guards, not billed totals.

Rebuild content deterministically: `python3 scripts/build_content.py && python3 scripts/enrich_content.py && python3 scripts/refine_content.py`. The refinement adds 200 active vocabulary entries, lesson-specific drill rubrics, scenario learner roles and beginner rehearsal. See `reports/curriculum-review.md` for the original audit and remaining depth limitations.


## Additive learner continuity release — 2026-09-06

`learning_cursor` records the learner's selected lesson session. An active lesson is resumed by `POST /sessions` (200 with `resumed:true`), while a new/replay session returns 201. Finished sessions with attempts constitute studied evidence independently of speech mastery; legacy sessions are read in place. Curriculum returns `studied`, `completed` (mastered), and `active_session_id`. Daily selection orders by level/unit/ordinal rather than fixed 20-lesson offsets, and follows the last explicit lesson selection.

Review cue upgrades add `title` and `cue_version`, preserving targets, IDs, schedules and attempt evidence. New errors refresh their context; audio capitalization/punctuation/spelling keys do not create speaking review cards. Evaluation checks the communication goal and grammatical validity while allowing new details/paraphrases.

Focused QA: see `docs/qa-resume.md`. Run `TEST_DATABASE_URL=<dedicated toko_*_test DSN> env -u GOROOT go test ./internal/app -run TestLessonResume -count=1`. Deployment supports `TEST_RUN=TestLessonResume` to limit this release's checks as requested. Readiness candidates do not start AI job workers; switching waits for existing metered learner requests/Live to finish. Schema changes are additive, and existing users/session/history remain intact.

## Cache, session and durable audio changes

[docs/new-features.md](docs/new-features.md) documents the implemented contract for private per-user caching, session `auto_audio`, reply Thai/audio fields, MinIO-backed durable audio, and the PostgreSQL LAN-port rollout. `contracts/openapi.json` remains canonical. Backend tests, frontend typechecking, six tests, and the production build have passed; deployment validation is still pending.
