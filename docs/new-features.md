# Cache, guided lessons, listening and daily meets

`contracts/openapi.json` is the canonical API contract. This note explains the
new behavior around that contract; it does not replace it.

## Private application cache

The replaceable Go cache interface is:

```go
Get(context.Context, string) ([]byte, bool, error)
Set(context.Context, string, []byte, time.Duration) error
DeletePrefix(context.Context, string) error
```

The in-memory implementation uses a 30-second default TTL and a 32 MiB maximum
(`CACHE_TTL_SECONDS` and `CACHE_MAX_MB`). Cache keys include a per-user epoch.
The important authenticated GET responses—curriculum, daily plan, library and
progress—are therefore private to the learner. User-owned scenarios and review
data use the same boundary. Authentication endpoints are never cached.

Any mutation invalidates that learner's cache prefix. Background jobs and Live
state changes do the same, so cached data cannot outlive a relevant write. The
cache is only an acceleration layer: PostgreSQL and durable media storage remain
authoritative. Private audio is not a shared response-cache entry and every
retrieval still checks ownership.

## Session progress and compatibility

`POST /sessions` accepts optional `auto_audio`. On a new session, omission
defaults to `false`; on a resumed active lesson, omission keeps that session's
saved setting. Use `PATCH /sessions/{id}/settings` with
`{"auto_audio": true | false}` to update an active session. It returns the saved
`auto_audio` value. Existing sessions behave as `false` unless explicitly set.

`Session.progress` and each turn result can include:

```json
{
  "percent": 0,
  "completed_drills": 0,
  "total_drills": 4,
  "independent_conversations": 0,
  "required_conversations": 2,
  "ready_to_complete": false
}
```

New lesson sessions set `SessionState.lesson_flow` to `guided-v2`. The learner
first sees the pattern, then completes two independent speaking rounds: a personal
use, followed by a roleplay use with one useful extra detail. Progress is 0, 50,
then 100 percent; the second successful independent round completes the lesson.
In guided sessions `total_drills` is 0. An active legacy session with no
`lesson_flow` continues to use the four-drill flow, then two independent
conversations. `POST /sessions/{id}/complete` remains available for the existing
manual-finish flow, and an unfinished active lesson resumes with saved history
and settings.

## Idempotent Thai hints

`POST /sessions/{id}/hints` accepts `idea` and an optional UUID `request_id`.
`idea` is limited to 500 Unicode characters, so Thai characters count as
characters rather than UTF-8 bytes. Repeating a successful request ID returns its
saved hint without another helper call. A failed helper call does not increment
the hint level, so the learner can retry without losing a hint step.

## English reply, Thai companion and playback

The same tutor evaluation now returns the English `reply` and `Feedback.reply_th`.
`reply_th` is optional in historical feedback. The model turn also persists
`Turn.text_th`. `POST /sessions/{id}/turns/{turnID}/translate` is retained for a
legacy model turn that has no Thai text: it uses the cached helper result when
available and saves the result as `text_th`. New replies do not need a separate
translation request.

`TurnResult` may include `reply_audio_id`, `audio_error`, `progress`, and
`session_completed`. When auto-audio is enabled, the backend synthesizes the
English reply and returns its private audio ID. That generation has an 18-second
deadline. A generation failure preserves the answer and reports `audio_error`;
replaying the same idempotent turn does not automatically make another paid TTS
request.

The frontend continues to use the `/api` BFF and its HttpOnly session-cookie
pattern. It attempts to play `reply_audio_id` at the learner's selected playback
speed. If Safari blocks autoplay, the interface keeps an accessible player so the
learner can tap to play; playback failure never discards the reply.

## Listening mode

Create a listening session with `POST /sessions` and `{"mode":"listening"}`.
The current tutor question is English audio. `POST /sessions/{id}/listen` accepts
`{"request_id":"<UUID>"}` and is idempotent for that request ID. It generates
or reuses private TTS for the current question, then progressively reveals help:

- listen 1: audio only;
- listen 2: a partial English caption;
- listen 3: the full English caption and Thai translation.

The learner may answer at any point. For listening, `goal_met` means the answer
shows that the learner understood the current question's context. `correct`
continues to represent grammatical correctness, so comprehension and grammar are
reported separately. Typed answers can demonstrate comprehension, but never add
speaking mastery.

## Daily meets

`POST /daily-meets` queues a 202 job. Its body requires a date-only `day`,
`source`, and UUID `request_id`; `title` is optional. Source accepts 3–6,000
Unicode characters and title accepts at most 120 characters. Poll the returned
`job_id` through the normal jobs endpoint, then use `GET /daily-meets` to list
the learner's saved entries.

The generated entry preserves the original source and includes `english`, `thai`,
one `question` and `question_th`, plus 3–6 reusable phrase records (`en`, `th`,
and `note`). `PATCH /daily-meets/{id}` updates the required `title`, `english`,
and `thai` fields. Only the owner can list, edit, or start practice from an entry;
all of these writes invalidate that learner's cached data.

`POST /daily-meets/{id}/sessions` accepts a UUID `request_id` and one of
`free`, `live`, or `listening`. The resulting session begins with the saved
daily-meet question and includes the entry as learner-owned context for replies
and Live. Reusing the same request ID returns the same session.

## Live transcripts

Live uses `LiveSystemPrompt`, a dedicated spoken-conversation instruction that
never includes the structured evaluator JSON contract. Input and output
transcripts (and available audio) are committed before the client receives a
`turnComplete` or interruption event. Each Live client event includes an
authoritative transcript snapshot for the text persisted so far; the client
should treat that snapshot as the current source of truth.

## Durable audio and retention

When `MINIO_ENDPOINT` is configured, MinIO is the durable remote store. The
database acts as the upload outbox: after the audio transaction commits, a worker
runs every five seconds, uploads up to ten assets, and records a one-minute retry
delay on failure. A local file is a durable cache replica, not the only copy. If
it is absent, an authorized audio request downloads the object from MinIO and
repopulates the local cache.

User recordings retain for 30 days. Expiry removes the remote object, local copy,
and audio metadata while leaving transcripts, feedback, and learning records.
Uploaded local replicas older than `AUDIO_LOCAL_CACHE_DAYS` (default three days)
are removed when idle; the remote TTS/lesson object remains reusable. TTS cache
keys are stable for the user, TTS model configuration, voice, and English text.
Lesson TTS has no expiry; private reply audio remains protected by ownership
checks.

Configure object storage through environment variables only:
`MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_BUCKET`,
`MINIO_USE_SSL`, `MINIO_PREFIX_TTS`, and `MINIO_PREFIX_USER_AUDIO`. Never commit
live endpoint credentials, access keys, or secret keys.

## PostgreSQL LAN port and rollout

The deployment defaults map container PostgreSQL port `5432` to
`192.168.1.122:15432` using `DATABASE_BIND_IP` and `DATABASE_HOST_PORT`. The
rollout recreates the database container only when that binding changes, while
preserving its exact volume, image, and credentials. It restores the prior
database container if the replacement fails readiness.

Before the short app-container restart, deployment waits for active metered AI
requests and Live sessions to finish. It retains the prior app container and
restores it if the new release fails readiness or public health verification.

## Validation and testing

The deployed progress/voice/cache release is `20260906-progress-voice-cache`
from backend `265e762` and frontend `bda27a2`. The next release,
`20260906-listening-daily-meet`, has not been deployed.

For safe local validation, fixtures, and the frontend command, follow
[TESTING.md](TESTING.md). Extend coverage with local fakes or fixtures: add API
and state tests under `internal/app`, then add frontend interaction tests in the
frontend test suite. Do not use production accounts, credentials, or a production
database. Automated tests do not cover real-device microphone permission,
backgrounding, Bluetooth, or interruption behavior.

## Product suggestions for language learning

- Show the communication goal and success criteria before each drill, in Thai and
  concise English.
- Fade hints from idea to keyword to pattern as the learner demonstrates success,
  while keeping an explicit “show a hint” control.
- Let learners ask for a brief meaning or comprehension clarification before they
  answer, then return them to the same speaking goal.
- In reviews, show the original context and the learner's recurring error category
  so repetition feels purposeful rather than random.
- After independent tasks, invite a short self-check on clarity and confidence in
  addition to the model's language feedback.
