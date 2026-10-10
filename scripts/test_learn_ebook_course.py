#!/usr/bin/env python3
"""Regression tests for the committed Learn Ebook course v2 corpus.

Runs without PostgreSQL, Codex or Gemini. The same structural rules gate the
writer pipeline in scripts/learn_ebook_v2/, so a hand edit of the JSON is held
to the standard the generated lessons had to pass.
"""

import copy
import json
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts" / "learn_ebook_v2"))

import build_course  # noqa: E402
from unit_rules import validate_unit  # noqa: E402

COURSE_PATH = ROOT / "internal" / "ebook" / "learn_ebook_v2.json"


def lesson_problems(lesson):
    plan = {"ordinal": lesson["ordinal"], "level": lesson["level"]}
    syllabus = {"vocabulary": [{"term": v["term"]} for v in lesson["vocabulary"]]}
    return validate_unit(build_course.as_unit(lesson), plan, syllabus)


class LearnEbookCourseTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.course = json.loads(COURSE_PATH.read_text(encoding="utf-8"))

    def test_course_totals_and_global_uniqueness(self):
        totals, problems = build_course.audit(self.course)
        self.assertEqual(problems, [])
        self.assertEqual(totals["lessons"], 145)
        self.assertEqual(totals["examples"], 870)
        self.assertEqual(totals["vocabulary"], 1450)
        self.assertEqual(totals["unique_terms"], 1450)
        self.assertEqual(totals["quiz_items"], 1160)
        self.assertEqual(totals["listening_items"], 435)
        self.assertEqual(totals["review_prompts"], 290)
        self.assertEqual(totals["unique_explanations"], 145)
        self.assertEqual(sorted(totals["levels"]), ["A1", "A2", "B1", "B2"])

    def test_every_lesson_passes_the_writer_rules(self):
        failures = {}
        for lesson in self.course["lessons"]:
            problems = lesson_problems(lesson)
            if problems:
                failures[lesson["id"]] = problems[:3]
        self.assertEqual(failures, {})

    def test_shadowing_lines_are_the_listening_lines(self):
        for lesson in self.course["lessons"]:
            self.assertEqual(lesson["shadowing"], [item["audio_en"] for item in lesson["listening_items"]], lesson["id"])

    def test_path_starts_at_a1_and_never_steps_down(self):
        rank = {"A1": 0, "A2": 1, "B1": 2, "B2": 3}
        path = sorted(self.course["lessons"], key=lambda lesson: lesson["path_order"])
        self.assertEqual(path[0]["level"], "A1")
        for before, after in zip(path, path[1:]):
            self.assertLessEqual(rank[before["level"]], rank[after["level"]], f"{before['id']} -> {after['id']}")

    def assertLessonRejected(self, edit):
        lesson = copy.deepcopy(self.course["lessons"][0])
        edit(lesson)
        self.assertNotEqual(lesson_problems(lesson), [])

    def test_rules_reject_template_explanation(self):
        self.assertLessonRejected(lambda lesson: lesson.update(explanation_th="ใช้โครงสร้างนี้ตามตัวอย่าง"))

    def test_rules_reject_listening_line_already_shown_as_example(self):
        self.assertLessonRejected(lambda lesson: lesson["listening_items"][0].update(audio_en=lesson["examples"][0]["en"]))

    def test_rules_reject_choice_key_outside_options(self):
        self.assertLessonRejected(lambda lesson: lesson["quiz"][0].update(answers=["not an option"]))

    def test_rules_reject_reorder_words_that_cannot_build_the_answer(self):
        self.assertLessonRejected(lambda lesson: lesson["quiz"][4].update(options=lesson["quiz"][4]["options"][1:]))

    def test_rules_reject_fix_item_without_a_mistake(self):
        self.assertLessonRejected(lambda lesson: lesson["quiz"][5].update(prompt_en=lesson["quiz"][5]["answers"][0]))

    def test_audit_rejects_vocabulary_repeated_across_units(self):
        course = copy.deepcopy(self.course)
        course["lessons"][1]["vocabulary"][0]["term"] = course["lessons"][0]["vocabulary"][0]["term"]
        _, problems = build_course.audit(course)
        self.assertTrue(any("repeated" in problem for problem in problems))

    def test_audit_rejects_example_repeated_across_units(self):
        course = copy.deepcopy(self.course)
        course["lessons"][1]["examples"][0] = copy.deepcopy(course["lessons"][0]["examples"][0])
        _, problems = build_course.audit(course)
        self.assertTrue(any("example repeated" in problem for problem in problems))


if __name__ == "__main__":
    unittest.main()
