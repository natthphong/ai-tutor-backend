"""Blind-solve every quiz and listening item without the key, then compare with the key.

A mismatch on a choice, fill, reorder or listening item matters at run time:
those kinds are marked against the key only, so a second valid answer would be
marked wrong.
"""
import json, os, re, sys, time, concurrent.futures as cf
from codex_call import call
HERE=os.path.dirname(os.path.abspath(__file__))
plan={u['ordinal']:u for u in json.load(open(f'{HERE}/units_plan.json'))}
PROMPT=open(f'{HERE}/solve_prompt.md').read()
os.makedirs(f'{HERE}/solves',exist_ok=True)
CONTRACTIONS=[("can't","can not"),("cannot","can not"),("won't","will not"),("shan't","shall not"),("n't"," not"),("i'm","i am"),("'re"," are"),("'ve"," have"),("'ll"," will"),
              ("he's","he is"),("she's","she is"),("it's","it is"),("that's","that is"),("there's","there is"),("what's","what is"),("who's","who is"),("where's","where is"),("let's","let us")]
def key(text):
    """Same surface-form comparison as courseAnswerKey in internal/app/ebook_check.go."""
    s=text.lower().strip().replace("’","'").replace("‘","'")
    s=re.sub(r'[,;:"?!.]',' ',s)
    for a,b in CONTRACTIONS: s=s.replace(a,b)
    return ' '.join(s.split())
def filled(prompt,answer):
    return re.sub(r'\s+([.?!,])',r'\1',' '.join(re.sub(r'\([^)]*\)','',prompt.replace('___',answer,1)).split()))
def compare(u,r):
    out=[]
    if len(r['quiz'])!=len(u['quiz']) or len(r['listening'])!=len(u['listening']): return ['solver returned the wrong number of answers']
    for i,(q,a) in enumerate(zip(u['quiz'],r['quiz']),1):
        k=q['kind']; accepted={key(x) for x in q['answers']}
        if k=='choice':
            if a.strip()!=q['answers'][0]: out.append(f"quiz[{i-1}] (choice) {q['prompt_en']!r} options {q['options']}: a second expert answered {a!r} but the key is {q['answers'][0]!r}. Make the sentence unambiguous so only the keyed option fits, or fix the key.")
        elif k=='fill':
            if key(a) not in accepted and key(a) not in {key(filled(q['prompt_en'],x)) for x in q['answers']}: out.append(f"quiz[{i-1}] (fill) {q['prompt_en']!r}: a second expert filled the blank with {a!r}, which is not in answers {q['answers']}. Add every correct filler to answers, or rewrite the sentence so only the listed fillers fit.")
        elif k=='reorder':
            given=[key(x) for x in a.split('||')]
            if any(g not in accepted for g in given if g): out.append(f"quiz[{i-1}] (reorder) words {q['options']}: a second expert built {a!r}, but answers are {q['answers']}. Add the other correct order to answers or change the words so only one order works.")
        elif k=='fix':
            if key(a)=='no mistake': out.append(f"quiz[{i-1}] (fix) {q['prompt_en']!r}: a second expert found no mistake in this sentence. Use a sentence with one clear mistake in this unit's grammar.")
    for i,(x,a) in enumerate(zip(u['listening'],r['listening']),1):
        if a.strip()!=x['answer_th']: out.append(f"listening[{i-1}] {x['audio_en']!r} question {x['question_th']!r} options {x['options_th']}: a second expert answered {a!r} but the key is {x['answer_th']!r}. Make exactly one option correct.")
    return out
def items(u):
    lines=[]
    for i,q in enumerate(u['quiz'],1):
        extra=''
        if q['kind']=='choice': extra=f" | options: {q['options']}"
        if q['kind']=='reorder': extra=f" | words: {q['options']}"
        lines.append(f"quiz {i} [{q['kind']}] {q['prompt_en']} | Thai shown to the learner: {q['prompt_th']}{extra}")
    for i,x in enumerate(u['listening'],1):
        lines.append(f"listening {i}: sentence: {x['audio_en']} | question: {x['question_th']} | options: {x['options_th']}")
    return '\n'.join(lines)
def run(o):
    src=f'{HERE}/units/ebook-{o:03d}.json'; out=f'{HERE}/solves/ebook-{o:03d}.json'
    if not os.path.exists(src): return o,'nounit'
    if os.path.exists(out) and os.path.getmtime(out)>=os.path.getmtime(src): return o,'ok' if not json.load(open(out))['mismatches'] else 'mismatch'
    u=json.load(open(src))
    for attempt in range(3):
        t=time.time()
        try:
            r=call(PROMPT.replace('{ORDINAL}',str(o)).replace('{TOPIC}',plan[o]['topic']).replace('{LEVEL}',plan[o]['level']).replace('{NQUIZ}',str(len(u['quiz']))).replace('{NLISTEN}',str(len(u['listening']))).replace('{ITEMS}',items(u)),f'{HERE}/solve_schema.json',timeout=900)
        except Exception as ex:
            print(f'solve {o} attempt {attempt} ERROR {str(ex)[:200]}',flush=True); time.sleep(20*(attempt+1)); continue
        mism=compare(u,r)
        json.dump({'ordinal':o,'solver':r,'mismatches':mism},open(out,'w'),ensure_ascii=False,indent=1)
        print(f'solve {o} {plan[o]["level"]} secs {int(time.time()-t)} mismatches {len(mism)}',flush=True)
        return o,'ok' if not mism else 'mismatch'
    return o,'FAILED'
if __name__=='__main__':
    workers=int(sys.argv[1]); todo=[int(x) for x in sys.argv[2:]] or sorted(plan)
    with cf.ThreadPoolExecutor(workers) as pool: res=list(pool.map(run,todo))
    import collections
    print('DONE',collections.Counter(v for _,v in res),'mismatch:',[o for o,v in res if v=='mismatch'],'failed:',[o for o,v in res if v in('FAILED','nounit')],flush=True)
