"""Structural audit for one generated Learn Ebook v2 unit. Returns a list of problems."""
import re
THAI = re.compile(r'[฀-๿]')
KINDS = ["choice", "choice", "fill", "fill", "reorder", "fix", "translate", "write"]
CAP = {"A1": 11, "A2": 14, "B1": 18, "B2": 23}

def norm(s):
    return re.sub(r"\s+", " ", re.sub(r"[^a-z0-9' ]", " ", s.lower().replace("’", "'"))).strip()

def exact(s):
    return " ".join(s.lower().replace("’", "'").split())

def words(s):
    return re.findall(r"[A-Za-z0-9'’-]+", s)

def stem(term):
    head = norm(term).split(" ")[0]
    if len(head) > 4 and head.endswith("e"): head = head[:-1]
    if len(head) > 4 and head.endswith("y"): head = head[:-1]
    return head[: max(3, min(len(head), 6))]

IRREGULAR = {
    "be": "am is are was were been being", "become": "became", "begin": "began begun", "break": "broke broken", "bring": "brought",
    "build": "built", "buy": "bought", "catch": "caught", "choose": "chose chosen", "come": "came", "do": "did does done",
    "draw": "drew drawn", "drink": "drank drunk", "drive": "drove driven", "eat": "ate eaten", "fall": "fell fallen", "feel": "felt",
    "fight": "fought", "find": "found", "fly": "flew flown", "forget": "forgot forgotten", "get": "got gotten", "give": "gave given",
    "go": "went gone goes", "grow": "grew grown", "have": "had has", "hear": "heard", "hide": "hid hidden", "hold": "held", "keep": "kept",
    "know": "knew known", "lay": "laid", "lead": "led", "leave": "left", "lend": "lent", "lie": "lay lain lying", "lose": "lost",
    "make": "made", "mean": "meant", "meet": "met", "pay": "paid", "ride": "rode ridden", "ring": "rang rung", "rise": "rose risen",
    "run": "ran", "say": "said", "see": "saw seen", "sell": "sold", "send": "sent", "shake": "shook shaken", "shoot": "shot",
    "sing": "sang sung", "sit": "sat", "sleep": "slept", "speak": "spoke spoken", "spend": "spent", "stand": "stood", "steal": "stole stolen",
    "swim": "swam swum", "take": "took taken", "teach": "taught", "tear": "tore torn", "tell": "told", "think": "thought",
    "throw": "threw thrown", "understand": "understood", "wake": "woke woken", "wear": "wore worn", "win": "won", "write": "wrote written",
    "bite": "bit bitten", "blow": "blew blown", "deal": "dealt", "dig": "dug", "feed": "fed", "freeze": "froze frozen", "hang": "hung",
    "seek": "sought", "shine": "shone", "sink": "sank sunk", "stick": "stuck", "strike": "struck", "swear": "swore sworn",
    "sweep": "swept", "swing": "swung", "weep": "wept", "bend": "bent", "bind": "bound", "bleed": "bled", "breed": "bred", "creep": "crept",
    "flee": "fled", "forbid": "forbade forbidden", "forgive": "forgave forgiven", "kneel": "knelt", "light": "lit", "slide": "slid",
    "spin": "spun", "spring": "sprang sprung", "sting": "stung", "undergo": "underwent undergone", "withdraw": "withdrew withdrawn",
    "good": "better best", "bad": "worse worst", "far": "further farther", "child": "children", "person": "people", "man": "men",
    "woman": "women", "foot": "feet", "tooth": "teeth", "mouse": "mice", "life": "lives", "knife": "knives", "leaf": "leaves",
    "shelf": "shelves", "wife": "wives", "half": "halves", "thief": "thieves",
}

def normalize_unit(u):
    """Make mechanical fields exact, so the writer is judged on language only.

    The word bank of a reorder item is always the words of its answer in a
    stable shuffled order; the writer's own split is discarded.
    """
    import hashlib, random
    for q in u.get("quiz", []):
        if q.get("kind") == "reorder" and q.get("answers"):
            words = q["answers"][0].split()
            rng = random.Random(int(hashlib.sha256(q["answers"][0].encode()).hexdigest(), 16))
            shuffled = words[:]
            for _ in range(20):
                rng.shuffle(shuffled)
                if shuffled != words: break
            q["options"] = shuffled
    return u

def has_term(term, text):
    t = norm(text)
    head = norm(term).split(" ")[0] if norm(term) else ""
    if head in IRREGULAR and any(re.search(r"\b" + re.escape(f) + r"\b", t) for f in IRREGULAR[head].split()):
        rest = [p for p in norm(term).split(" ")[1:] if p not in ("a", "an", "the", "to", "of", "sb", "sth", "somebody", "something")]
        if all(re.search(r"\b" + re.escape(p[: max(2, min(len(p), 4))]), t) for p in rest): return True
    parts = [p for p in norm(term).split(" ") if p not in ("a", "an", "the", "to", "of", "sb", "sth", "somebody", "something")]
    if not parts: return False
    first = stem(parts[0])
    if not re.search(r"\b" + re.escape(first), t): return False
    return all(re.search(r"\b" + re.escape(p[: max(2, min(len(p), 4))]), t) for p in parts[1:])

def thai(s): return bool(THAI.search(s or ""))

def validate_unit(u, plan, syl):
    e = []
    lvl = plan["level"]
    if u.get("ordinal") != plan["ordinal"]: e.append(f"ordinal must be {plan['ordinal']}")
    for f in ["title", "title_th", "goal_th", "explanation_th", "pattern"]:
        if not str(u.get(f, "")).strip(): e.append(f"{f} is empty")
    if len(u.get("title", "")) > 75: e.append("title longer than 75 characters")
    if THAI.search(u.get("title", "")): e.append("title must be English")
    for f in ["title_th", "goal_th", "explanation_th"]:
        if not thai(u.get(f)): e.append(f"{f} must be Thai")
    if len(u.get("explanation_th", "")) < 220: e.append("explanation_th is too short to teach the point (write 4-7 full Thai sentences)")
    if not 2 <= len(u.get("usage_th", [])) <= 4: e.append("usage_th needs 2-4 items")
    if not 2 <= len(u.get("forms", [])) <= 4: e.append("forms needs 2-4 items")
    for f in u.get("forms", []):
        if not (f["label_th"].strip() and f["pattern"].strip() and f["example_en"].strip() and thai(f["example_th"])): e.append("forms item incomplete")
        if thai(f["example_en"]): e.append("forms.example_en must be English")
    if len(u.get("signal_words", [])) > 6: e.append("signal_words max 6")
    if not 2 <= len(u.get("common_mistakes", [])) <= 3: e.append("common_mistakes needs 2-3 items")
    for m in u.get("common_mistakes", []):
        # Punctuation can be the mistake (commas in non-defining clauses), so compare the raw text.
        if exact(m["wrong"]) == exact(m["right"]) or not thai(m["why_th"]): e.append(f"common_mistakes item invalid (wrong and right are the same): {m['wrong']!r}")
    ex = u.get("examples", [])
    if len(ex) != 6: e.append(f"examples must be exactly 6, got {len(ex)}")
    exn = set()
    for x in ex:
        if thai(x["en"]) or not thai(x["th"]): e.append(f"example language wrong: {x['en']!r}")
        if not x["en"].strip().endswith((".", "?", "!")): e.append(f"example must end with punctuation: {x['en']!r}")
        if len(words(x["en"])) > CAP[lvl]: e.append(f"example too long for {lvl} ({len(words(x['en']))} words): {x['en']!r}")
        if norm(x["en"]) in exn: e.append(f"duplicate example: {x['en']!r}")
        exn.add(norm(x["en"]))
    want = [norm(v["term"]) for v in syl["vocabulary"]]
    got = [norm(v["term"]) for v in u.get("vocabulary", [])]
    if got != want: e.append(f"vocabulary terms must be exactly, in order: {[v['term'] for v in syl['vocabulary']]}")
    for v in u.get("vocabulary", []):
        if not thai(v["meaning_th"]) or not thai(v["example_th"]) or thai(v["example_en"]): e.append(f"vocabulary language wrong for {v['term']!r}")
        elif not has_term(v["term"], v["example_en"]): e.append(f"vocabulary example_en does not contain the term {v['term']!r}: {v['example_en']!r}")
        if len(words(v["example_en"])) > CAP[lvl] + 1: e.append(f"vocabulary example too long for {lvl}: {v['example_en']!r}")
    lis = u.get("listening", [])
    # Target words must be practised in the unit's own sentences: examples,
    # listening lines, quiz sentences and review targets all count.
    practice = [x["en"] for x in ex] + [x["audio_en"] for x in lis] + [x.get("target_en", "") for x in u.get("review", [])]
    for x in u.get("quiz", []):
        practice += [x.get("prompt_en", "")] + list(x.get("answers", [])) + list(x.get("options", []))
    body = " . ".join(practice)
    covered = [v["term"] for v in syl["vocabulary"] if has_term(v["term"], body)]
    if len(covered) < 8:
        missing = [v["term"] for v in syl["vocabulary"] if v["term"] not in covered]
        e.append(f"examples, listening and quiz sentences use only {len(covered)} of the 10 target terms; work more of these in: {missing}")
    q = u.get("quiz", [])
    if [x.get("kind") for x in q] != KINDS: e.append(f"quiz kinds must be exactly in this order: {KINDS}")
    else:
        for i, x in enumerate(q, 1):
            tag = f"quiz {i} ({x['kind']})"
            if not thai(x["explanation_th"]): e.append(f"{tag}: explanation_th must be Thai")
            if not thai(x["prompt_th"]): e.append(f"{tag}: prompt_th must be Thai")
            if not x["answers"] or any(not a.strip() for a in x["answers"]): e.append(f"{tag}: answers missing"); continue
            if any(thai(a) for a in x["answers"]): e.append(f"{tag}: answers must be English")
            k = x["kind"]
            if k in ("choice", "fill"):
                if x["prompt_en"].count("___") != 1: e.append(f"{tag}: prompt_en must contain exactly one ___")
                full = x["prompt_en"].replace("___", x["answers"][0])
                full = re.sub(r"\s*\([^)]*\)", "", full)
                if norm(full) in exn: e.append(f"{tag}: sentence is copied from the examples")
            if k == "choice":
                opts = [norm(o) for o in x["options"]]
                if len(opts) != 3 or len(set(opts)) != 3: e.append(f"{tag}: needs exactly 3 distinct options")
                if len(x["answers"]) != 1 or x["answers"][0] not in x["options"]: e.append(f"{tag}: answers must be exactly one of the options, copied exactly")
            elif x["options"] and k != "reorder": e.append(f"{tag}: options must be []")
            if k == "reorder":
                target = x["answers"][0].split()
                if sorted(x["options"]) != sorted(target): e.append(f"{tag}: options must be exactly the words of answers[0] ({x['answers'][0]!r}), got {x['options']}")
                elif x["options"] == target: e.append(f"{tag}: options are not shuffled")
                if not 4 <= len(target) <= 10: e.append(f"{tag}: sentence must be 5-9 words")
                if norm(x["answers"][0]) in exn: e.append(f"{tag}: sentence is copied from the examples")
            if k == "fix":
                if exact(x["prompt_en"]) in [exact(a) for a in x["answers"]]: e.append(f"{tag}: prompt_en must contain a mistake and differ from the answers")
                if norm(x["answers"][0]) in exn: e.append(f"{tag}: sentence is copied from the examples")
            if k == "translate" and norm(x["answers"][0]) in exn: e.append(f"{tag}: sentence is copied from the examples")
    quiz_sentences = {}
    for i, x in enumerate(q, 1):
        if not x.get("answers"): continue
        sentence = x["answers"][0]
        if x.get("kind") in ("choice", "fill"): sentence = re.sub(r"\s*\([^)]*\)", "", x["prompt_en"].replace("___", x["answers"][0]))
        if x.get("kind") == "write": continue
        key = norm(sentence)
        if key in quiz_sentences: e.append(f"quiz {i} repeats the sentence of quiz {quiz_sentences[key]} ({sentence!r}); every quiz item needs a different sentence")
        quiz_sentences[key] = i
    if len(lis) != 3: e.append(f"listening must be exactly 3, got {len(lis)}")
    seen_elsewhere = {norm(v["example_en"]) for v in u.get("vocabulary", [])} | {norm(x["target_en"]) for x in u.get("review", [])}
    for x in q:
        for a in x.get("answers", []):
            seen_elsewhere.add(norm(a))
            if x.get("kind") in ("choice", "fill"): seen_elsewhere.add(norm(re.sub(r"\s*\([^)]*\)", "", x["prompt_en"].replace("___", a))))
    for i, x in enumerate(lis, 1):
        if norm(x["audio_en"]) in seen_elsewhere: e.append(f"listening {i}: audio_en {x['audio_en']!r} repeats a vocabulary example, quiz sentence or review target; write a new sentence")
    if len({norm(x["audio_en"]) for x in lis}) != len(lis): e.append("listening: audio_en sentences must be different from each other")
    pos = []
    for i, x in enumerate(lis, 1):
        if thai(x["audio_en"]) or not thai(x["meaning_th"]) or not thai(x["question_th"]): e.append(f"listening {i}: language wrong")
        if norm(x["audio_en"]) in exn: e.append(f"listening {i}: audio_en is copied from the examples")
        if len(x["options_th"]) != 3 or len(set(x["options_th"])) != 3: e.append(f"listening {i}: needs exactly 3 distinct options_th")
        if x["answer_th"] not in x["options_th"]: e.append(f"listening {i}: answer_th must be one of options_th, copied exactly")
        else: pos.append(x["options_th"].index(x["answer_th"]))
        if len(words(x["audio_en"])) > CAP[lvl] + 6: e.append(f"listening {i}: audio_en too long for {lvl}")
    if len(pos) == 3 and len(set(pos)) == 1: e.append("listening: the correct option is in the same position in all 3 items; vary it")
    sp = u.get("speaking", {})
    if not sp.get("prompt_en", "").strip() or thai(sp.get("prompt_en", "")) or not thai(sp.get("prompt_th", "")): e.append("speaking prompt language wrong")
    if len(sp.get("success_criteria_th", [])) != 3: e.append("speaking.success_criteria_th must be exactly 3")
    rv = u.get("review", [])
    if len(rv) != 2: e.append("review must be exactly 2")
    for x in rv:
        if not thai(x["prompt_th"]) or thai(x["target_en"]) or not x["target_en"].strip() or not thai(x["target_th"]): e.append("review item language wrong")
        if norm(x["target_en"]) in exn: e.append(f"review target is copied from the examples: {x['target_en']!r}")
    return e
