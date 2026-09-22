#!/usr/bin/env python3
"""Regression tests for the generated Learn Ebook corpus contract."""

import copy
import importlib.util
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("generate_learn_ebook_course.py")
SPEC = importlib.util.spec_from_file_location("learn_ebook_generator", SCRIPT)
generator = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(generator)


class LearnEbookCorpusTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.course = generator.build()

    def assertAuditRejects(self, edit):
        course = copy.deepcopy(self.course)
        edit(course)
        with self.assertRaises(AssertionError):
            generator.audit_course(course)

    def test_generated_course_passes_full_audit(self):
        totals = generator.audit_course(self.course)
        self.assertEqual(
            totals,
            {"lessons": 145, "vocabulary": 1450, "quiz_items": 725, "shadowing_lines": 435},
        )

    def test_audit_rejects_generic_range_example_output(self):
        self.assertAuditRejects(
            lambda course: course["lessons"][19].update(
                examples=copy.deepcopy(course["lessons"][18]["examples"])
            )
        )

    def test_audit_rejects_duplicate_vocabulary_in_one_lesson(self):
        self.assertAuditRejects(
            lambda course: course["lessons"][0]["vocabulary"][1].update(
                term=course["lessons"][0]["vocabulary"][0]["term"]
            )
        )

    def test_audit_rejects_duplicate_examples(self):
        self.assertAuditRejects(
            lambda course: course["lessons"][0]["examples"].__setitem__(
                1, copy.deepcopy(course["lessons"][0]["examples"][0])
            )
        )

    def test_audit_rejects_shadowing_line_that_is_not_an_example(self):
        self.assertAuditRejects(
            lambda course: course["lessons"][0]["shadowing"].__setitem__(
                0, "This shadowing line was invented."
            )
        )

    def test_audit_rejects_generic_example_translation(self):
        self.assertAuditRejects(
            lambda course: course["lessons"][0]["examples"][0].update(
                th="ประโยคนี้ใช้ในสถานการณ์ทั่วไป"
            )
        )

    def test_audit_rejects_generic_vocabulary_translation(self):
        self.assertAuditRejects(
            lambda course: course["lessons"][0]["vocabulary"][0].update(
                example_th="เราพูดถึงกำหนดการระหว่างการส่งต่องานให้ลูกค้า"
            )
        )

    def test_choice_answers_are_rotated_and_unique(self):
        for quiz_id in ("quiz-01", "quiz-02", "quiz-04"):
            positions = []
            for lesson in self.course["lessons"]:
                item = next(q for q in lesson["quiz"] if q["id"] == quiz_id)
                positions.append(item["options"].index(item["answers"][0]))
                self.assertEqual(len(item["answers"]), 1)
                self.assertEqual(len(set(item["options"])), 3)
            self.assertEqual(set(positions), {0, 1, 2}, quiz_id)


if __name__ == "__main__":
    unittest.main()
