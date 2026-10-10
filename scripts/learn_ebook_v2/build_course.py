"""Assemble the generated units into the embedded Learn Ebook course JSON and audit it."""
import collections, glob, json, os, re, sys
from unit_rules import validate_unit, normalize_unit, norm
HERE = os.path.dirname(os.path.abspath(__file__))
VERSION = "2026-10-10.v2"

def slug(text):
    return re.sub(r"[^a-z0-9]+", "-", text.lower()).strip("-")

def load():
    plan = {u["ordinal"]: u for u in json.load(open(f"{HERE}/units_plan.json"))}
    syl = {}
    for f in sorted(glob.glob(f"{HERE}/syllabus/chunk_*.json")):
        for x in json.load(open(f))["units"]:
            syl[x["ordinal"]] = x
    return plan, syl

def build(units_dir=None):
    plan, syl = load()
    units_dir = units_dir or f"{HERE}/units"
    lessons, problems = [], []
    for o in sorted(plan):
        path = f"{units_dir}/ebook-{o:03d}.json"
        if not os.path.exists(path):
            problems.append(f"unit {o}: not generated"); continue
        u = normalize_unit(json.load(open(path))); p = plan[o]; s = syl[o]
        for err in validate_unit(u, p, s):
            problems.append(f"unit {o}: {err}")
        vocab = [{"id": f"v-{i:02d}", "term": v["term"], "meaning_th": v["meaning_th"], "example_en": v["example_en"], "example_th": v["example_th"]} for i, v in enumerate(u["vocabulary"], 1)]
        quiz = [{"id": f"quiz-{i:02d}", "kind": q["kind"], "prompt_en": q["prompt_en"], "prompt_th": q["prompt_th"], **({"options": q["options"]} if q["options"] else {}), "answers": q["answers"], "explanation_th": q["explanation_th"]} for i, q in enumerate(u["quiz"], 1)]
        listening = [{"id": f"listen-{i:02d}", "audio_en": x["audio_en"], "meaning_th": x["meaning_th"], "question_th": x["question_th"], "options_th": x["options_th"], "answer_th": x["answer_th"]} for i, x in enumerate(u["listening"], 1)]
        lessons.append({
            "id": f"ebook-{o:03d}", "unit_id": str(o), "ordinal": o, "path_order": p["path_order"], "level": p["level"],
            "title": u["title"], "title_th": u["title_th"], "theme_en": s["theme_en"], "theme_th": s["theme_th"], "domain": s["domain"],
            "goal_th": u["goal_th"], "explanation_th": u["explanation_th"], "pattern": u["pattern"],
            "usage_th": u["usage_th"], "forms": u["forms"], "signal_words": u["signal_words"], "common_mistakes": u["common_mistakes"], "compare_th": u["compare_th"],
            "examples": u["examples"], "vocabulary": vocab, "quiz": quiz, "listening_items": listening,
            "shadowing": [x["audio_en"] for x in listening],
            "speaking": {"prompt_en": u["speaking"]["prompt_en"], "prompt_th": u["speaking"]["prompt_th"], "target_vocabulary": ["v-01", "v-02", "v-03"], "success_criteria_th": u["speaking"]["success_criteria_th"]},
            "review": [{"id": f"review-{i:02d}", "prompt_th": r["prompt_th"], "target_en": r["target_en"], "target_th": r["target_th"]} for i, r in enumerate(u["review"], 1)],
            "concept_tags": [p["level"], slug(p["topic"])[:60], s["domain"], f"unit-{o}"],
        })
    course = {"id": "learn-ebook", "title": "Learn Ebook · Practical English Grammar A1–B2", "version": VERSION,
              "reference": {"id": "private-source", "title": "Private 145-unit grammar reference", "unit_count": 145}, "lessons": lessons}
    return course, problems

def audit(course):
    problems = []
    L = course["lessons"]
    terms = collections.Counter(norm(v["term"]) for l in L for v in l["vocabulary"])
    examples = collections.defaultdict(list)
    for l in L:
        for e in l["examples"]: examples[norm(e["en"])].append(l["id"])
    for k, ids in examples.items():
        if len(ids) > 1: problems.append(f"example repeated across units {ids}: {k!r}")
    for t, n in terms.items():
        if n > 1: problems.append(f"vocabulary term repeated {n}x: {t!r}")
    expl = collections.Counter(norm(l["explanation_th"]) for l in L)
    if len(expl) != len(L): problems.append("explanations are not unique per unit")
    orders = sorted(l["path_order"] for l in L)
    if orders != list(range(1, len(L) + 1)): problems.append("path_order is not a permutation")
    totals = {"lessons": len(L), "examples": sum(len(l["examples"]) for l in L), "vocabulary": sum(terms.values()), "unique_terms": len(terms),
              "quiz_items": sum(len(l["quiz"]) for l in L), "listening_items": sum(len(l["listening_items"]) for l in L),
              "review_prompts": sum(len(l["review"]) for l in L), "unique_explanations": len(expl),
              "levels": dict(collections.Counter(l["level"] for l in L)), "domains": dict(collections.Counter(l["domain"] for l in L))}
    return totals, problems

def as_unit(lesson):
    """Map an assembled lesson back to the writer's unit shape."""
    unit = {key: lesson[key] for key in (
        "ordinal", "title", "title_th", "goal_th", "explanation_th", "pattern", "usage_th", "forms",
        "signal_words", "common_mistakes", "compare_th", "examples")}
    unit["vocabulary"] = [{k: v[k] for k in ("term", "meaning_th", "example_en", "example_th")} for v in lesson["vocabulary"]]
    unit["quiz"] = [{"kind": q["kind"], "prompt_en": q["prompt_en"], "prompt_th": q["prompt_th"], "options": q.get("options", []),
                     "answers": q["answers"], "explanation_th": q["explanation_th"]} for q in lesson["quiz"]]
    unit["listening"] = [{k: x[k] for k in ("audio_en", "meaning_th", "question_th", "options_th", "answer_th")} for x in lesson["listening_items"]]
    unit["speaking"] = {k: lesson["speaking"][k] for k in ("prompt_en", "prompt_th", "success_criteria_th")}
    unit["review"] = [{k: r[k] for k in ("prompt_th", "target_en", "target_th")} for r in lesson["review"]]
    return unit

def restore_units(course_path):
    """Recreate units/ from the committed course so one unit can be rewritten."""
    os.makedirs(f"{HERE}/units", exist_ok=True)
    course = json.load(open(course_path, encoding="utf-8"))
    for lesson in course["lessons"]:
        path = f"{HERE}/units/{lesson['id']}.json"
        if not os.path.exists(path):
            json.dump(as_unit(lesson), open(path, "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    print("restored", len(course["lessons"]), "units")

if __name__ == "__main__":
    if len(sys.argv) > 2 and sys.argv[1] == "--restore-units":
        restore_units(sys.argv[2]); sys.exit(0)
    course, problems = build()
    totals, more = audit(course)
    problems += more
    print(json.dumps(totals, ensure_ascii=False))
    for p in problems[:80]: print("PROBLEM", p)
    print("problems:", len(problems))
    if len(sys.argv) > 1 and not problems:
        json.dump(course, open(sys.argv[1], "w", encoding="utf-8"), ensure_ascii=False, indent=1)
        print("written", sys.argv[1], os.path.getsize(sys.argv[1]))
    sys.exit(1 if problems else 0)
