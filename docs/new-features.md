# Cache, session and durable audio

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

A lesson progresses through four drills and then two independent conversations.
The two conversation successes are distinct steps; after the sixth required step,
the session completes automatically. `POST /sessions/{id}/complete` remains
available for the existing manual-finish flow. An unfinished active lesson still
resumes with its saved history and settings.

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

## Validation status

Backend tests have passed. Frontend typechecking, six tests, and the production
build have passed. Deployment verification is still pending.

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
