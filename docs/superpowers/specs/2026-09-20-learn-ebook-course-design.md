# Learn Ebook Course Redesign

## Intent

Learn Ebook should teach a learner, rather than display a PDF with controls around it. The 145 grammar units remain aligned with the supplied book's unit order, but the primary experience is original Toko Loop teaching content. The private book is an optional authenticated reference.

The learner should always know what to do next, how much is left, whether a unit is still being learned, learned, or due for review, and why an answer was accepted or needs work.

## Product flow

Each unit is an 8–12 minute lesson with five ordered steps:

1. **Understand** — one Thai explanation, one practical goal, and a reusable grammar pattern.
2. **Examples** — three short everyday/work examples with Thai meaning and audio.
3. **Quick practice** — five focused mini-quiz items shown one at a time with immediate feedback and retry.
4. **Listen and shadow** — three simple sentences taken verbatim from the lesson examples. The learner hears and repeats each sentence; the app evaluates actual audio.
5. **Use it** — a two-round guided speaking exchange that asks the learner to apply the pattern to their own situation.

The primary actions are **Explain**, **Practice**, and **Next**. The original lesson and exercise pages live in an **Original Book** drawer. Opening or closing the drawer does not change the current step.

## Course content

The application ships a versioned, static course authored for all 145 units. Core lessons do not wait for runtime AI generation. Each unit contains:

- a Thai goal and concise explanation;
- one primary pattern plus supporting notes;
- three practical examples with Thai translations;
- exactly ten non-empty, case-insensitively unique vocabulary terms within that unit, each with Thai meaning and a useful example;
- five mini-quiz items with stable IDs, instructions, choices or accepted written answers, and Thai explanations;
- three shadowing lines that exactly match example sentences from the same unit;
- one practical speaking goal and opening question.

Vocabulary may intentionally reappear in a later unit when repetition supports learning; “unique” means no duplicate term inside one unit. Content validation rejects missing units, duplicate IDs, unsafe empty fields, quiz items without answers, vocabulary counts other than ten, or shadowing lines not found in the unit examples.

The existing generated `ebook_packs` rows and learner progress remain untouched for rollback and compatibility. The new static course takes precedence for the redesigned UI. Existing page cursors, draft answers, review items, and oral-session history remain readable.

## Progress and review

Progress is stored per learner and unit in the existing `ebook_progress.state` JSON so the migration is additive. New fields are:

- `current_step`: integer 1–5;
- `completed_steps`: object keyed by `understand`, `examples`, `quiz`, `shadowing`, and `speaking`;
- `quiz_answers` and `quiz_scores` keyed by stable quiz ID;
- `vocabulary_saved` keyed by normalized term;
- `learned`: boolean;
- `completed_at`: ISO timestamp when all five steps pass;
- `review_requested`: boolean when the learner marks the unit “ยังไม่แม่น”.

The API derives one visible status:

- `unlearned`: no activity;
- `learning`: activity exists but the five steps are incomplete;
- `learned`: all five steps are complete and no review is due;
- `review`: the learner requested review or a due review item exists for that unit.

Marking a unit “เรียนแล้ว” is allowed only after all five steps pass. “เรียนอีกครั้ง” returns it to learning while keeping history. Quiz mistakes create contextual review items. A successful retry updates the unit score but does not delete history.

## Listening and speaking

Generic Free Speak listening keeps its current three-listen comprehension flow. Ebook listening becomes shadowing:

- the source English sentence and Thai meaning are always shown;
- TTS speaks exactly the current unit's shadowing line;
- the learner repeats using recorded audio;
- transcript, pronunciation, grammar, and naturalness feedback come from the existing audio evaluation path;
- a clear, goal-matching audio attempt advances to the next line;
- typed input may help accessibility but never counts as shadowing mastery;
- three passed lines complete the shadowing step.

The final speaking step retains two independent audio rounds. Tutor context is limited to the unit goal, pattern, examples, and speaking prompt.

## API and compatibility

The existing `/ai-tutor/api/v2/ebook` namespace remains. Responses add versioned lesson content and derived progress; legacy fields remain optional through the transition.

- `GET /ebook` returns all 145 units with compact derived status, percent, current step, and review due state.
- `GET /ebook/units/:id` returns sanitized lesson content and learner progress. Quiz answers and explanations are withheld.
- `PATCH /ebook/units/:id/progress` accepts bounded step and self-review updates as well as legacy page/draft fields.
- `POST /ebook/units/:id/check` evaluates redesigned quiz IDs, returns immediate marks, and remains request-ID idempotent.
- `POST /ebook/units/:id/reveal` remains explicit and records help usage.
- `POST /ebook/units/:id/sessions` accepts `shadowing` and `speak`; `listening` remains an alias for old clients.
- `GET /ebook/pages/:page` remains owner-authenticated and private.

No existing table or user row is dropped or rewritten. New clients work with the new course immediately; an older client can still open the reference book and existing sessions.

## Layout and responsive behavior

Desktop uses three columns inside the existing app shell:

- left: short searchable unit navigation with All, Learning, Learned, and Review filters;
- center: one lesson step at a time, progress “Step N of 5 · about M min”, goal, content, feedback, and the three primary actions;
- right: Loop Coach explanation, exactly ten vocabulary items, useful phrases, and unit progress.

On iPad landscape, all three areas remain visible with a narrower coach. On iPad portrait, the coach moves below the lesson and the unit list becomes a compact drawer. On iPhone, content is one column with a sticky step strip and bottom action bar; touch targets are at least 44 px and no horizontal scroll is allowed.

Status is never communicated by color alone. Step navigation uses `aria-current`, progress uses `role=progressbar`, feedback uses a polite live region, and dialogs return focus to their trigger.

The visual language keeps the current cream/yellow Toko Loop palette, rounded cards, duck mascot, typography, and existing navigation. The supplied reference image informs information hierarchy only.

## Errors and loading

Course content is available with the binary, so a unit never enters an AI-generation polling state. Progress writes are serialized; an unsent draft remains in local storage and retries after reload. Quiz evaluation failures keep the learner's answer and do not create a result. TTS failure leaves the sentence readable and offers retry. Missing private book pages affect only the optional drawer, never the lesson.

## Verification

Backend validation covers all 145 units, exactly ten unique words per unit, lesson/example/shadowing consistency, answer redaction, ownership, idempotent checks, additive progress state, status derivation, and three audio-only shadowing passes.

Frontend tests cover step navigation, immediate correct/incorrect feedback, persistence after reload, learned/learning/review filters, optional book drawer, exactly ten vocabulary cards, shadowing source and audio-only mastery, and the speaking handoff. Playwright projects cover desktop 1440×1000, iPad portrait 820×1180, iPad landscape 1180×820, and iPhone 390×844 with screenshots and clipping checks.

Deployment requires complete automated validation, backend migration/readiness/HTTPS checks, frontend main push, and production smoke checks through the authenticated BFF.
