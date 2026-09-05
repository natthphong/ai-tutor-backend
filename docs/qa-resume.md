# Lesson resume and completion QA

Run the focused backend check only against the dedicated local test database:

```sh
TEST_DATABASE_URL='postgres://toko:toko-local-only@127.0.0.1:55432/toko_loop_test?sslmode=disable' \
  env -u GOROOT go test ./internal/app -run TestLessonResume -count=1
```

`TestLessonResume` starts its own fake Gemini server. It makes no request to Gemini and has no AI cost.

The check verifies that:

- an empty lesson cannot complete and remains active;
- a hinted typed response can mark lesson 001 as `studied` without marking it `completed` (mastered), and the daily plan advances to lesson 002;
- completing the same session twice is idempotent;
- a repeated start of active lesson 002 resumes the identical session, state, and turns; the daily plan and a fresh login retain that session;
- completion of lesson 002 makes the daily plan select lesson 003;
- a free-talk session and an older parallel lesson do not replace the curriculum cursor;
- replaying completed lesson 001 creates a new session while retaining its `studied` history;
- legacy completed lesson history with attempts is inferred as `studied` without a cursor or mastery reset; and
- sessions and cursor state remain isolated per learner.

For this QA scope, a curriculum `unit` is treated as the `unit` on an individual lesson card. Product-level unit completion rules are pending clarification.
