import json, os, sys, time, concurrent.futures as cf
from codex_call import call
HERE=os.path.dirname(os.path.abspath(__file__))
plan={u['ordinal']:u for u in json.load(open(f'{HERE}/units_plan.json'))}
PROMPT=open(f'{HERE}/review_prompt.md').read()
os.makedirs(f'{HERE}/reviews',exist_ok=True)
def run(o):
    src=f'{HERE}/units/ebook-{o:03d}.json'; out=f'{HERE}/reviews/ebook-{o:03d}.json'
    if not os.path.exists(src): return o,'nounit'
    if os.path.exists(out) and os.path.getmtime(out)>=os.path.getmtime(src): return o,json.load(open(out))['verdict']
    unit=open(src,encoding='utf-8').read()
    for attempt in range(3):
        t=time.time()
        try:
            r=call(PROMPT.replace('{ORDINAL}',str(o)).replace('{TOPIC}',plan[o]['topic']).replace('{LEVEL}',plan[o]['level']).replace('{UNIT}',unit),f'{HERE}/review_schema.json',timeout=900)
        except Exception as ex:
            print(f'review {o} attempt {attempt} ERROR {str(ex)[:200]}',flush=True); time.sleep(20*(attempt+1)); continue
        errs=[i for i in r['issues'] if i['severity']=='error']
        r['verdict']='fix' if errs else 'pass'
        json.dump(r,open(out,'w'),ensure_ascii=False,indent=1)
        print(f'review {o} {plan[o]["level"]} secs {int(time.time()-t)} {r["verdict"]} errors {len(errs)}',flush=True)
        return o,r['verdict']
    return o,'FAILED'
if __name__=='__main__':
    workers=int(sys.argv[1]); todo=[int(x) for x in sys.argv[2:]] or sorted(plan)
    with cf.ThreadPoolExecutor(workers) as pool: res=list(pool.map(run,todo))
    import collections
    print('DONE',collections.Counter(v for _,v in res),'fix:',[o for o,v in res if v=='fix'],'failed:',[o for o,v in res if v in('FAILED','nounit')],flush=True)
