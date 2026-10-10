You are an expert English teacher sitting a learner's quiz. Do not run shell commands or read files; reply with the final JSON only.

Below are the practice items of one lesson exactly as a Thai learner sees them, WITHOUT the answer key. Grammar point of the lesson: {TOPIC} (CEFR {LEVEL}). Answer every item with the single most natural correct answer.

Return:
- "quiz": exactly {NQUIZ} strings, one per quiz item in order:
  - choice: copy exactly ONE of the given options. If you believe two options are both correct in that sentence, answer "AMBIGUOUS: <option 1> | <option 2>".
  - fill: only the missing text for the blank (not the whole sentence).
  - reorder: the full sentence built from exactly the given words. If two different orders are equally natural, answer the most natural one, then " || ", then the other.
  - fix: the full corrected sentence. If the sentence already looks correct to you, answer "NO MISTAKE".
  - translate: one natural English translation of the Thai sentence.
  - write: any one correct sentence that answers the prompt.
- "listening": exactly {NLISTEN} strings: for each item read the English sentence and the Thai question, then copy exactly ONE of the Thai options. If two options are both correct, answer "AMBIGUOUS: <option 1> | <option 2>".
- "ordinal": {ORDINAL}

ITEMS:
{ITEMS}
