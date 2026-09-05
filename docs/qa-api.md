# Local API and Gemini QA

Tested the local API at `http://localhost:8080/ai-tutor/api/v2` on 5 September 2026 with synthetic QA users and app-generated speech. The remote deployment was not tested.

## Result

The REST and Gemini flow passed 18 checks. The measured main-flow delta was 10 Gemini calls and 0.2314 THB over 21.7 seconds. The existing focused model evaluation also passed 9/9 cases at 0.1387 THB (`reports/gemini-evaluation.json`).

The flow verified:

- health/readiness and AI configuration
- one-use invitations
- TTS WAV retrieval and owner-only audio access
- lesson valid-alternative assessment (`Hi, my name is Pim.`)
- request-id replay returning the same attempt without adding a usage call
- app TTS WAV uploaded as multipart audio, transcription, feedback, and linked retry
- owner-only access to uploaded attempt audio
- audio review and spaced-review rescheduling
- progress and session persistence after a fresh login
- grammatical but off-task input (`I like coffee.`) producing `correct=true`, `goal_met=false`, and no grammar correction
- custom scenario generation, editing, roleplay, completion, asynchronous post-scene review, and a practiceable retry session

The Live WebSocket flow passed all checks in `reports/live-smoke.json`:

- an invalid origin was rejected with HTTP 403 without consuming the ticket
- the accepted ticket could not be reused and returned HTTP 401
- the backend accepted paced 16 kHz PCM generated from the app TTS fixture
- Gemini returned input transcription, output audio, and output transcription
- explicit stop cleared `live_active`
- a fresh-ticket reconnect became ready and also cleared `live_active` after stop
- the two connections added two usage calls and four Live seconds total
- calls, billed amount, and Live seconds did not change during the three-second check after stop

## Finding

One longer TTS request encountered `Gemini connection interrupted; please retry` on all three attempts and did not finish within the harness's 120-second bound. The job eventually ended as failed. A separate short TTS request (`Hello, I'm Pim.`) completed and produced the WAV used for the successful end-to-end checks.

The running release settled each interrupted TTS attempt with no token/audio usage as the full 3 THB reservation. Because the worker reserves again on each retry, a three-attempt provider interruption can present as 9 THB of estimated usage. This is materially higher than the likely work performed and can exhaust the learner's budget during a provider incident. The implementation owner was notified during QA.

The worker also calls `runJob` synchronously from its one-second loop. A TTS request that waits for three provider timeouts blocks later TTS, scenario, and summary jobs from being claimed during those calls. The short successful TTS request was observed queued behind the longer failing request. A bounded worker pool, separate queues, or a shorter per-attempt timeout would prevent this head-of-line delay.

## Reproduce

Run the deterministic and integration checks against the dedicated test database:

```sh
env -u GOROOT \
  TEST_DATABASE_URL='postgres://toko:toko-local-only@127.0.0.1:55432/toko_loop_test?sslmode=disable' \
  go test -race ./internal/app/... ./internal/learning/... ./internal/gemini/... ./internal/security/... ./internal/content/...
```

Run REST QA with a previously generated app TTS asset to avoid another speech-generation charge:

```sh
QA_TTS_AUDIO_ID='<owned app audio UUID>' python3 scripts/qa_api.py
```

Run Live QA with the WAV downloaded by the REST harness:

```sh
env -u GOROOT QA_TTS_WAV=/tmp/toko-qa-tts.wav go run ./cmd/livecheck
```

The harnesses do not print credentials or bearer tokens. `scripts/qa_api.py` sends the required `TokoLoop-QA/2` user agent and writes a redacted result to `reports/qa-api.json`.
