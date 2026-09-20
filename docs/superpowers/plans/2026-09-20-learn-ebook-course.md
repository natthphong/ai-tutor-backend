# Learn Ebook Course Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the PDF-first Learn Ebook screen with a complete 145-unit, concept-first interactive course that tracks learning and review and uses lesson-derived shadowing.

**Architecture:** Embed a validated original course beside the existing private book manifest, expose compact progress through the current Ebook API, and keep old DB packs/progress readable. Split the Next.js client into a course shell and focused lesson-step components; reuse authenticated audio/session infrastructure for shadowing and speaking.

**Tech Stack:** Go 1.24, Fiber, PostgreSQL JSONB, embedded JSON, Next.js 16, React 19, TypeScript, Playwright.

**Spec:** `docs/superpowers/specs/2026-09-20-learn-ebook-course-design.md`

## Global Constraints

- Preserve all existing users, progress, sessions, review rows, audio, and old Ebook packs.
- Core course content is static and available without a runtime AI authoring call.
- Ship exactly 145 units and exactly 10 case-insensitively unique vocabulary terms inside each unit.
- Ebook shadowing uses three lines found verbatim in that unit's examples and requires clear learner audio.
- Keep the private 392-page book owner-authenticated and optional.
- Keep Toko Loop cream/yellow branding and support desktop, iPhone, iPad portrait, iPad landscape, and Split View.
- Follow test-first red/green cycles and do not call paid providers from automated tests.

## Review Focus

- Old progress JSON without new fields returns `learning` or `unlearned` without panicking; covered in Task 2.
- A unit with 9, 11, or duplicate vocabulary terms fails startup validation; covered in Task 1.
- Typed shadowing attempts never complete a line; covered in Task 3.
- Repeated check/session request IDs return the original result; covered in Tasks 2 and 3.
- Missing book images break only the optional reference drawer; covered in Tasks 1 and 5.

---

### Task 1: Versioned 145-unit course

**Files:**
- Create: `internal/ebook/course.go`
- Create: `internal/ebook/course_lessons.json`
- Create: `internal/ebook/course_test.go`
- Modify: `internal/app/app.go`

**Interfaces:**
- Produces: `ebook.Course`, `ebook.Lesson`, `ebook.Example`, `ebook.Word`, `ebook.QuizItem`, `ebook.ShadowLine`, `ebook.SpeakingTask`.
- Produces: `ebook.LoadCourse() (*Course, error)` and `(*Course).Lesson(id string) (Lesson, bool)`.
- Validation guarantees 145 sequential IDs, ten unique words, five quiz items, and three example-derived shadowing lines.

- [ ] **Step 1: Write failing course validation tests**

Add table tests that load the embedded file and assert 145 IDs, all non-empty teaching fields, ten normalized distinct vocabulary terms, five distinct quiz IDs with answers, and three shadowing lines contained in examples. Mutate fixtures to 9 words, duplicate terms, missing answers, or unrelated shadow lines and assert a validation error.

- [ ] **Step 2: Verify red**

Run `env -u GOROOT go test ./internal/ebook -run 'TestCourse|TestLesson' -count=1` and confirm it fails because the course types/loader do not exist.

- [ ] **Step 3: Implement model, embedded loader, and Luna-authored content**

Use a versioned JSON schema with one entry for every private-manifest unit number/title. Author original goals, patterns, examples, Thai explanations, ten practical terms, five mini quizzes, three shadowing lines, and a speaking prompt. Do not copy exercise or answer-key text.

- [ ] **Step 4: Verify green and complete coverage**

Run `env -u GOROOT go test ./internal/ebook -count=1` and a JSON audit that prints `units=145 vocabulary=1450 quizzes=725 shadowing=435`.

- [ ] **Step 5: Load the course at application startup**

Add `Course *ebook.Course` to `App`; fail startup if embedded content is invalid. Keep private-book loading independent so a missing page is reported only by the reference endpoint unless production sets `EBOOK_REQUIRED=true`.

- [ ] **Step 6: Commit**

Commit backend Task 1 as `Add complete Learn Ebook course content`.

### Task 2: Course API, progress status, and immediate quiz

**Files:**
- Modify: `internal/app/ebook.go`
- Modify: `internal/app/ebook_check.go`
- Modify: `internal/app/ebook_feature_test.go`
- Modify: `internal/storage/schema.sql` only if an index is required
- Modify: `contracts/openapi.json`

**Interfaces:**
- `GET /ebook` adds `status`, `percent`, `current_step`, and `review_due` per unit.
- `GET /ebook/units/:id` returns a sanitized `lesson` and learner `progress` without quiz answers.
- `PATCH /ebook/units/:id/progress` accepts optional `current_step`, `completed_step`, `learned`, `review_requested`, `page`, and draft maps.
- `POST /ebook/units/:id/check` checks one or more stable course quiz IDs idempotently.

- [ ] **Step 1: Write failing API tests**

Cover a new user, legacy page-only JSON, in-progress unit, fully complete unit, due review, owner isolation, invalid step, learned-before-complete rejection, reset-to-learning history preservation, answer redaction, immediate correct/incorrect marks, and replayed request IDs.

- [ ] **Step 2: Verify red**

Run `TEST_DATABASE_URL='postgres://toko:toko-local-only@localhost:55432/toko_loop_test?sslmode=disable' env -u GOROOT go test ./internal/app -run 'TestEbookCourse|TestEbookProgress|TestEbookQuiz' -count=1` and confirm expected missing-field/behavior failures.

- [ ] **Step 3: Serve static lesson content and merge bounded progress**

Prefer `App.Course` over `ebook_packs`; keep legacy pack endpoints compatible. Compute visible status from completed steps, self-review, and due `review_items` keys. Merge only recognized keys into JSONB under a locked user/unit row.

- [ ] **Step 4: Implement quiz checking and review cues**

Check choice items deterministically. Check accepted written variants by normalization first and call the configured tutor only for ambiguous written answers. Add the unit title, learning goal, and full prompt to review items. Preserve request-ID replay.

- [ ] **Step 5: Verify green and contract generation**

Run the focused tests, `env -u GOROOT go test ./internal/app -count=1`, validate `contracts/openapi.json`, then run `npm run generate:api` in the frontend.

- [ ] **Step 6: Commit**

Commit backend Task 2 as `Track Learn Ebook steps and review status`.

### Task 3: Lesson-derived shadowing

**Files:**
- Modify: `internal/app/ebook.go`
- Modify: `internal/app/listening.go`
- Modify: `internal/app/sessions.go`
- Modify: `internal/app/session_features.go`
- Modify: `internal/app/ebook_feature_test.go`
- Modify: `contracts/openapi.json`

**Interfaces:**
- `POST /ebook/units/:id/sessions` accepts `shadowing` and `speak`; `listening` aliases `shadowing` for old Ebook clients.
- Shadowing state contains `ebook_activity`, `shadow_index`, and the three server-selected lines.
- `/sessions/:id/listen` returns the exact current target, Thai meaning, cached TTS audio, and `listen_count` without hiding the target.
- Only a clear audio turn with `goal_met` advances; three distinct lines complete the step.

- [ ] **Step 1: Write failing shadowing tests**

Assert the opening line equals course shadowing line 1, the listen response shows that source, typed input cannot advance, unclear audio cannot advance, a successful audio attempt advances exactly once, retries are idempotent, and three passed targets persist `completed_steps.shadowing=true`.

- [ ] **Step 2: Verify red**

Run `env -u GOROOT go test ./internal/app -run TestEbookShadowing -count=1` and confirm current comprehension behavior fails the assertions.

- [ ] **Step 3: Implement shadowing session behavior**

Build the tutor task from the target line and unit pattern. Override the model reply with the next server-owned line after a successful attempt so arbitrary generated sentences cannot enter the sequence. Keep pronunciation feedback and learner audio storage from the existing attempt path.

- [ ] **Step 4: Verify green and generic-listening non-regression**

Run shadowing tests plus all existing listening and guided lesson tests. Verify generic Free Speak still uses progressive captions and meaning-based answers.

- [ ] **Step 5: Commit**

Commit backend Task 3 as `Teach Ebook shadowing from lesson examples`.

### Task 4: Frontend lesson shell and step components

**Files:**
- Replace: `frontend/src/components/LearnEbook.tsx`
- Create: `frontend/src/components/ebook/EbookTypes.ts`
- Create: `frontend/src/components/ebook/UnitNavigator.tsx`
- Create: `frontend/src/components/ebook/LessonStepper.tsx`
- Create: `frontend/src/components/ebook/LessonConcept.tsx`
- Create: `frontend/src/components/ebook/LessonExamples.tsx`
- Create: `frontend/src/components/ebook/LessonQuiz.tsx`
- Create: `frontend/src/components/ebook/LessonCoach.tsx`
- Create: `frontend/src/components/ebook/OriginalBookDialog.tsx`
- Modify: `frontend/src/app/globals.css`
- Modify: `frontend/tests/practice-flow.test.mjs`

**Interfaces:**
- `LearnEbook` owns selected unit and persisted step.
- Focused children receive immutable lesson/progress props and callback actions.
- The center column renders only one step; the right column renders coach/vocabulary/progress.

- [ ] **Step 1: Write failing component/source behavior tests**

Assert the five step labels, three primary actions, status filters, exactly ten vocabulary entries from fixtures, original-book dialog default closed, question-scoped live feedback, and legacy-progress defaults.

- [ ] **Step 2: Verify red**

Run `npm test -- --test-name-pattern='Learn Ebook course shell'` and confirm the current PDF-first component fails.

- [ ] **Step 3: Implement the split components**

Use the generated API types. Render compact unit navigation, one active card, visible step/time progress, Loop Coach, ten vocabulary cards, useful phrases, and the optional authenticated book dialog. Serialize progress writes and retain failed drafts locally.

- [ ] **Step 4: Add responsive and accessible CSS**

Use three columns at desktop, reduced three-column layout at iPad landscape, two/one-column layouts at tablet/phone, sticky phone actions, 44 px targets, `aria-current`, progress semantics, focus-visible styles, and no color-only status.

- [ ] **Step 5: Verify green**

Run `npm run typecheck`, `npm test`, and `npm run build`.

- [ ] **Step 6: Commit**

Commit frontend Task 4 as `Redesign Learn Ebook around lesson steps`.

### Task 5: Shadowing UI, speaking handoff, and multi-device E2E

**Files:**
- Create: `frontend/src/components/EbookShadowingPractice.tsx`
- Modify: `frontend/src/components/Practice.tsx`
- Modify: `frontend/e2e/learner-flows.spec.ts`
- Modify: `frontend/playwright.config.ts`
- Modify: `frontend/docs/TESTING.md`

**Interfaces:**
- `Practice` selects `EbookShadowingPractice` when `session.state.ebook_activity === "shadowing"`.
- The component displays target English/Thai, audio controls, microphone, retry feedback, and line N/3.
- Completion returns to `/?view=ebook` and restores the selected unit/step.

- [ ] **Step 1: Write failing mocked Playwright flows**

Cover learned/learning/review filters, step persistence after reload, immediate quiz feedback and retry, optional book open/close, ten words, shadowing target source, typed non-mastery, three audio passes, speaking launch, keyboard navigation, and missing book-image recovery.

- [ ] **Step 2: Verify red on four viewports**

Run `npm run test:e2e` with projects Desktop 1440×1000, iPad portrait 820×1180, iPad landscape 1180×820, and iPhone 390×844. Confirm current UI fails the new expectations.

- [ ] **Step 3: Implement shadowing UI and return path**

Reuse `VoiceRecorder`, the BFF, and existing feedback cards. Never count text as mastery. Preserve `from=ebook`, selected unit, and current step on return.

- [ ] **Step 4: Verify green and capture screenshots**

Run `npm run validate`. Save deterministic screenshots under `frontend/docs/screenshots/ebook-<viewport>-<state>.jpg` for desktop, iPad portrait/landscape, and iPhone. Inspect for clipping, overlap, focus, and readable hierarchy.

- [ ] **Step 5: Commit**

Commit frontend Task 5 as `Add Ebook shadowing and responsive course QA`.

### Task 6: Review, deploy, and production smoke

**Files:**
- Modify: `backend/README.md`
- Modify: `backend/docs/TESTING.md`
- Modify: `frontend/README.md`
- Create/update: `/Users/AR697030/Desktop/ai-tutor-loop/details.md`

- [ ] **Step 1: Run independent code review**

Give the reviewer the spec, plan, base SHAs, and final SHAs. Fix every critical or important finding and rerun affected tests.

- [ ] **Step 2: Run fresh full verification**

Backend: `bash scripts/validate.sh`. Frontend: `npm run validate`. Confirm all 145 content audit totals and inspect all screenshots.

- [ ] **Step 3: Commit and push both main branches**

Confirm clean worktrees, commit documentation, push backend `main`, and push frontend `main`.

- [ ] **Step 4: Deploy backend safely**

Run `RELEASE_ID=20260920-ebook-course TEST_RUN='^$' bash deploy_local.sh`, verify migration/readiness/rollback output, and check authenticated catalog/unit/shadowing endpoints over HTTPS.

- [ ] **Step 5: Verify Vercel**

Wait for the frontend main deployment and use the production BFF to verify login, 145-unit catalog, a lesson, progress update, quiz, optional book page, and shadowing-session creation.

- [ ] **Step 6: Publish the completion report**

Write the no-secret report, publish it to Outline, and send the requested LINE notification with the production URL, release IDs, test counts, screenshots, and report link.
