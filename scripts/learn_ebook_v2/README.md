# Learn Ebook course v2 — content pipeline

This folder is how `internal/ebook/learn_ebook_v2.json` was written. The 145
lessons are authored by the Codex CLI with model `gpt-6-luna` at reasoning
effort `high`. Gemini is not used for content. Nothing here reads the private
source book: the input is `units_plan.json`, a hand-made list of the grammar
point, CEFR level and learning-path position of each unit.

The app never runs this pipeline. It only embeds the committed JSON, which
`internal/ebook/course.go` validates at startup.

## Steps

```sh
cd scripts/learn_ebook_v2
python3 syllabus_run.py                 # 1. theme + 10 globally new words per unit -> syllabus/chunk_*.json (committed)
python3 units_run.py 6                  # 2. write lessons, 6 at a time -> units/ebook-NNN.json
./round_run.sh 8                        # 3. one quality round: blind solve, editor review, repair
./round_run.sh 8                        #    repeat until solve_run and review_run report no mismatch / no fix
python3 build_course.py ../../internal/ebook/learn_ebook_v2.json   # 4. assemble + audit
cd ../.. && python3 scripts/test_learn_ebook_course.py && env -u GOROOT go test ./internal/ebook -count=1
```

Every runner is resumable: finished or unchanged units are skipped, so a rerun
only does what is missing. Pass unit numbers after the worker count to limit a
run, for example `python3 units_run.py 4 7 109`.

## What gates a lesson

1. `unit_rules.py` — structure and answer keys: exact counts, the fixed quiz
   kinds, one blank per choice/fill, a choice key that is one of three options,
   a fix item that really differs from its answer, no quiz sentence used
   twice, listening lines that appear nowhere else in the lesson, sentence
   length per level, Thai/English in the right fields, and at least 8 of the
   10 target words practised in the unit's own sentences. The word bank of a
   reorder item is always rebuilt from its answer (`normalize_unit`). A
   rejected lesson is rewritten with the problems as feedback.
2. `solve_run.py` — a second Codex call sits the quiz and the listening
   questions without the key. Its answers are compared with the key using the
   same surface-form rule as the app (`courseAnswerKey`). A mismatch on a
   choice, fill, reorder or listening item means a learner could give a valid
   answer and be marked wrong, so the item is sent back.
3. `review_run.py` — a third Codex call acts as editor (`review_prompt.md`)
   and reports only real defects: wrong or unnatural English, an example that
   does not show the grammar point, a wrong rule, a wrong key, a
   mistranslation, a "wrong" sentence that is actually acceptable, text far
   above the level.
4. `repair_run.py` — feeds 2 and 3 back to the writer as a minimal patch.
5. `build_course.py` — course-wide audit: every vocabulary term and every
   example is unique across all 145 units, explanations are unique, and
   `path_order` is a permutation.

The committed course was released with no open finding from steps 1-3 and 5:
121 of the 145 lessons were patched at least once on the way, and the last
seven lessons were fixed by hand. These gates prove structure, answer-key
consistency and one AI editor's judgement. They do not replace a teacher
reading every sentence.

## Changing content

Small fixes: edit the JSON and run the two test commands above. The per-unit
working files are not committed; to rewrite one unit from a fresh clone, run
`python3 build_course.py --restore-units ../../internal/ebook/learn_ebook_v2.json`,
delete `units/ebook-NNN.json`, and rerun steps 2-4. A change to the quiz kinds, counts or lesson
shape needs a new course version in `internal/ebook/course.go`, with the old
version appended to `LearnEbookPreviousCourseVersions` so learner progress is
carried over.
