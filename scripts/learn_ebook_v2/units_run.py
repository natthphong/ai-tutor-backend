import json, os, sys, time, concurrent.futures as cf
from codex_call import call
from unit_rules import validate_unit, normalize_unit
HERE=os.path.dirname(os.path.abspath(__file__))
plan={u['ordinal']:u for u in json.load(open(f'{HERE}/units_plan.json'))}
import glob
syl={}
for _f in sorted(glob.glob(f'{HERE}/syllabus/chunk_*.json')):
    for _x in json.load(open(_f))['units']: syl[_x['ordinal']]=_x
PROMPT=open(f'{HERE}/unit_prompt.md').read()
RULE={'A1':'A1: sentences of 4-9 words built from the ~800 most common English words; one clause per sentence (two only when the grammar point needs it); no idioms.',
      'A2':'A2: sentences of 5-12 words; common everyday vocabulary (about the 2000 most common words); at most two clauses.',
      'B1':'B1: sentences of 6-16 words; natural everyday and workplace language; up to two clauses.',
      'B2':'B2: sentences of 8-20 words; natural, fluent workplace and discussion language.'}
os.makedirs(f'{HERE}/units',exist_ok=True)
def run(o):
    path=f'{HERE}/units/ebook-{o:03d}.json'
    if os.path.exists(path): return o,'cached',0
    p,s=plan[o],syl[o]; feedback=''; last=None
    for attempt in range(4):
        prompt=(PROMPT.replace('{ORDINAL}',str(o)).replace('{TOPIC}',p['topic']).replace('{LEVEL}',p['level']).replace('{THEME_EN}',s['theme_en']).replace('{THEME_TH}',s['theme_th']).replace('{DOMAIN}',s['domain'])
                .replace('{VOCAB}','; '.join(f"{v['term']} ({v['pos']}: {v['meaning_th']})" for v in s['vocabulary'])).replace('{LEVEL_RULE}',RULE[p['level']]).replace('{FEEDBACK}',feedback))
        t=time.time()
        try: u=call(prompt,f'{HERE}/unit_schema.json',timeout=1200)
        except Exception as ex:
            print(f'unit {o} attempt {attempt} ERROR {str(ex)[:300]}',flush=True); time.sleep(20*(attempt+1)); continue
        u=normalize_unit(u); errs=validate_unit(u,p,s); last=(u,errs)
        print(f'unit {o} {p["level"]} attempt {attempt} secs {int(time.time()-t)} errors {len(errs)}',flush=True)
        if not errs:
            json.dump(u,open(path,'w'),ensure_ascii=False,indent=1); return o,'ok',attempt
        feedback='\nYOUR PREVIOUS ANSWER WAS REJECTED by the automatic checker. Rewrite the whole lesson and fix ALL of these problems:\n- '+'\n- '.join(errs[:40])
    if last: json.dump({'unit':last[0],'errors':last[1]},open(f'{HERE}/units/FAILED-{o:03d}.json','w'),ensure_ascii=False,indent=1)
    return o,'FAILED',4
if __name__=='__main__':
    todo=[int(x) for x in sys.argv[2:]] or sorted(plan)
    workers=int(sys.argv[1]) if len(sys.argv)>1 else 6
    with cf.ThreadPoolExecutor(workers) as pool:
        res=list(pool.map(run,todo))
    bad=[o for o,st,_ in res if st=='FAILED']
    print('DONE ok',sum(1 for _,st,_ in res if st!='FAILED'),'failed',bad,flush=True)
