# Listening and Daily Meet

This guide describes the authenticated API under `/ai-tutor/api/v2`. The exact
schemas remain in `../contracts/openapi.json`.

## Listening

Create a session with `POST /sessions` and `mode: "listening"`. The session
starts with an English question. To play it, call `POST /sessions/{id}/listen`
with a UUID `request_id`:

```json
{"request_id":"<UUID>"}
```

The response returns `audio_id`, `listen_count`, `caption`, and `translation`.
For one model question, the sequence is audio only, partial caption, then full
caption plus Thai translation. Reusing the same request ID returns the saved
result, so client retries do not advance the sequence.

The learner can submit a text or audio answer after any listen count using
`POST /sessions/{id}/turns`. In listening mode, `feedback.goal_met` means the
answer demonstrates contextual understanding; `feedback.correct` remains the
separate grammar judgement. Text can count as understanding but not as speaking
mastery.

## Daily Meet lifecycle

1. Create an entry with `POST /daily-meets`.

   ```json
   {
     "day":"2026-09-06",
     "title":"Optional title",
     "source":"Thai or mixed-language work notes",
     "request_id":"<UUID>"
   }
   ```

   `source` accepts 3–6,000 Unicode characters; `title` is optional and limited
   to 120 characters. The endpoint returns 202 and a `job_id`.

2. Poll `GET /jobs/{job_id}` until the job completes, then list saved entries
   with `GET /daily-meets`. The generated record keeps its original source and
   supplies English, Thai, a question in both languages, and 3–6 reusable
   phrase records.

3. Update an entry with `PATCH /daily-meets/{id}`. Send all required editable
   fields: `title`, `english`, and `thai`.

4. Start a contextual practice session with
   `POST /daily-meets/{id}/sessions`:

   ```json
   {"mode":"listening","request_id":"<UUID>"}
   ```

   Valid modes are `free`, `live`, and `listening`. The saved Daily Meet context
   and its generated question accompany the session. Repeating its request ID
   returns the same session.

Daily Meet records, jobs, and sessions are owner-scoped. Mutations invalidate
only the affected learner's cached data.

## Local test extension

Follow [TESTING.md](TESTING.md) for the isolated database, local-only fixture
account, and frontend validation command. Add backend cases under `internal/app`
with fake providers; add frontend interactions in the frontend test suite. Do not
use production data or credentials. Real-device microphone permissions,
backgrounding, Bluetooth, and interruption behavior still need device testing.
