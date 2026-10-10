You are a senior EFL materials writer and a native-level speaker of both English and Thai. Do not run shell commands or read files; reply with the final JSON only.

Write ONE complete self-study lesson for a Thai adult learner. It is unit {ORDINAL} of a 145-unit grammar course (A1 -> B1/B2). The learner studies alone on a phone, 30 minutes a day, and started the course at A1. Everything must be your own original writing (never copy textbook sentences).

UNIT
- Grammar point: {TOPIC}
- CEFR level of this unit: {LEVEL}
- Situation theme for ALL English sentences in this unit: {THEME_EN} ({THEME_TH}) [{DOMAIN}]
- Target vocabulary (use exactly these 10 terms, in this order): {VOCAB}

LANGUAGE CONTROL ({LEVEL})
{LEVEL_RULE}
Besides the grammar point and the 10 target terms, use only words that a learner at this level already knows. Every English sentence must be natural, something a real person would say in this situation, and grammatically perfect. Thai must be natural everyday Thai (not word-for-word), friendly and free of linguistics jargon; English grammar names may stay in English.

FIELDS
- title: clean English title of the grammar point (max 60 characters). title_th: short Thai title that says what the learner can do (max 60 characters).
- goal_th: one Thai sentence: what the learner will be able to say after this unit, in this situation.
- explanation_th: 4-7 Thai sentences that really teach: what the structure means, when people use it, how it differs from how Thai expresses the same idea, and what to watch. Specific to this grammar point. No generic study advice.
- pattern: the core formula in one line (English labels, e.g. "Subject + am/is/are + verb-ing").
- usage_th: 2-4 short Thai bullet strings, each "ใช้เมื่อ..." with a tiny English example in the same string.
- forms: 2-4 items covering the forms that matter for this point (usually บอกเล่า / ปฏิเสธ / คำถาม; for non-verb topics use the relevant variants). Each: label_th, pattern, example_en, example_th.
- signal_words: 0-6 English words/phrases that often appear with this structure (empty list if none).
- common_mistakes: 2-3 mistakes Thai learners really make with THIS point. wrong = the incorrect English sentence, right = the corrected sentence, why_th = one Thai sentence explaining why.
- compare_th: if learners often confuse this point with another structure, 1-3 Thai sentences that contrast them with a short English pair; otherwise "".
- examples: exactly 6 {en, th}. All in the theme. Each clearly shows the grammar point. Include a mix of statement / negative / question where the point allows, and at least 2 first-person sentences the learner could say about their own life. Together with the listening sentences they must use at least 8 of the 10 target terms.
- vocabulary: exactly the 10 target terms in the given order. meaning_th = short Thai meaning. example_en = a short natural sentence in the theme using the term (prefer reusing the grammar point); example_th = its Thai translation.
- quiz: exactly 8 items in THIS order of kind: choice, choice, fill, fill, reorder, fix, translate, write. All quiz sentences are NEW (not copied from examples) and stay in the theme. Each of the 8 items uses a DIFFERENT sentence (never reuse one sentence in two items), and together they cover the statement, negative and question forms where the point has them. Every item must be impossible to answer correctly without applying this unit's grammar.
  - choice: prompt_en = one sentence with exactly one blank written as "___". The sentence must carry enough context (a time word, a signal word, or a second clause) that ONLY ONE option is correct; never leave two options both possible. options = exactly 3 short fillers for the blank; the 2 wrong ones are the errors learners typically make (wrong form of the SAME word or the competing structure), never random words. answers = [the one correct option, copied exactly]. Only one option may be correct. prompt_th = Thai meaning of the completed sentence.
  - fill: the blank always needs at least one real word (never an empty answer; for "no article" points test the zero article in a choice item with the option "(no article)" instead). prompt_en = one sentence with exactly one blank "___" followed by the base word in brackets when a verb/adjective form is needed, e.g. "She ___ (work) from home today." answers = every acceptable text for the blank only (include contracted and full forms when both fit, e.g. ["is working", "'s working"]). options = []. prompt_th = Thai meaning of the completed sentence.
  - reorder: answers = [the correct sentence] (add a second sentence only if another word order is equally correct). options = the words of that sentence as separate strings in a clearly shuffled order (keep the capital letter on the first word of the sentence and attach the final . or ? to the last word, so the multiset of options joined by spaces equals the sentence). 5-9 words. prompt_en = "Put the words in the correct order." prompt_th = Thai meaning of the sentence.
  - fix: prompt_en = one sentence containing exactly ONE mistake in this unit's grammar. answers = the corrected full sentence plus any equally correct variants (for example contracted form). options = []. prompt_th = "แก้ประโยคนี้ให้ถูก: " + Thai meaning of the correct sentence.
  - translate: prompt_th = one Thai sentence that requires this grammar point. prompt_en = "Translate into English." answers = 1-3 natural English translations. options = [].
  - write: prompt_en = one simple English question or instruction that makes the learner write ONE true sentence about themselves using the grammar point. prompt_th = Thai version with a hint of what to include. answers = [one model answer]. options = [].
  - explanation_th for every item: one Thai sentence saying WHY the answer is right (name the rule), not just repeating the answer.
- listening: exactly 3 items. audio_en = one NEW spoken-style sentence (or two very short ones) in the theme that uses the grammar point. It is heard before it is read, so it must be different from every examples sentence, every vocabulary example_en, every quiz sentence and every review target. meaning_th = natural Thai translation. question_th = a Thai comprehension question that can only be answered by understanding the grammar meaning (time, who, whether it happened, how often, etc.). options_th = exactly 3 short Thai answers, only one correct, the wrong ones plausible. answer_th = the correct option copied exactly. Vary the position of the correct option.
- speaking: prompt_en = one friendly English question/mini-task in the theme that makes the learner speak 1-3 sentences about themselves with this grammar point (simple enough for the level). prompt_th = Thai translation plus a short hint. success_criteria_th = exactly 3 Thai strings: (1) uses the grammar point correctly, (2) communicates the situation clearly, (3) uses at least two named target words (name them in English).
- review: exactly 2 items for spaced review days later. prompt_th = a NEW mini-situation in Thai (different from the examples) that tells the learner what to say in English, forcing this grammar point, e.g. "เพื่อนโทรมาถามว่าตอนนี้คุณทำอะไรอยู่ ตอบเขาเป็นภาษาอังกฤษ". target_en = one model answer; target_th = its Thai meaning.

SELF-CHECK before answering: every English sentence is correct and natural; each answer key is right and unique where it must be; reorder options really rebuild the sentence; Thai translations match; the level limits are respected; counts are exact (6 examples, 10 vocabulary, 8 quiz, 3 listening, 3 criteria, 2 review). Set "ordinal" to {ORDINAL}.
{FEEDBACK}
