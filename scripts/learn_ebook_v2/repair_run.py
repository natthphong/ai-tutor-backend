"""Rewrite units that the editor flagged, feeding the editor's errors back to the writer."""
import json, os, sys, time, concurrent.futures as cf
import units_run as U
HERE=U.HERE
def run(o):
    rv=f'{HERE}/reviews/ebook-{o:03d}.json'; src=f'{HERE}/units/ebook-{o:03d}.json'
    if not (os.path.exists(rv) and os.path.exists(src)) or os.path.getmtime(rv)<os.path.getmtime(src): return o,'skip'
    r=json.load(open(rv)); errs=[i for i in r['issues'] if i['severity']=='error']
    sv=f'{HERE}/solves/ebook-{o:03d}.json'
    if os.path.exists(sv) and os.path.getmtime(sv)>=os.path.getmtime(src):
        errs+=[{'field':m.split(' ')[0],'problem':m,'suggestion':'see problem'} for m in json.load(open(sv))['mismatches']]
    if not errs: return o,'pass'
    old=open(src,encoding='utf-8').read()
    p,s=U.plan[o],U.syl[o]
    base=('\nAN EDITOR REVIEWED THE PREVIOUS VERSION OF THIS LESSON (below) AND FOUND THE ERRORS LISTED HERE. This is a minimal patch, not a rewrite: change ONLY the items named in the list (and anything that must change with them, such as a Thai translation or an explanation of the same item) and copy every other field of the previous version character for character. Do not rephrase, reorder or improve anything that is not listed. Return the full lesson JSON with the fixes applied:\n- '
          +'\n- '.join(f"{i['field']}: {i['problem']} -> {i['suggestion']}" for i in errs)+'\n\nPREVIOUS VERSION:\n'+old)
    feedback=base
    for attempt in range(4):
        prompt=(U.PROMPT.replace('{ORDINAL}',str(o)).replace('{TOPIC}',p['topic']).replace('{LEVEL}',p['level']).replace('{THEME_EN}',s['theme_en']).replace('{THEME_TH}',s['theme_th']).replace('{DOMAIN}',s['domain'])
                .replace('{VOCAB}','; '.join(f"{v['term']} ({v['pos']}: {v['meaning_th']})" for v in s['vocabulary'])).replace('{LEVEL_RULE}',U.RULE[p['level']]).replace('{FEEDBACK}',feedback))
        t=time.time()
        try: u=U.call(prompt,f'{HERE}/unit_schema.json',timeout=1200)
        except Exception as ex:
            print(f'repair {o} attempt {attempt} ERROR {str(ex)[:200]}',flush=True); time.sleep(20*(attempt+1)); continue
        u=U.normalize_unit(u); problems=U.validate_unit(u,p,s)
        print(f'repair {o} attempt {attempt} secs {int(time.time()-t)} structural errors {len(problems)}',flush=True)
        if not problems:
            os.makedirs(f'{HERE}/units_before_repair',exist_ok=True)
            if not os.path.exists(f'{HERE}/units_before_repair/ebook-{o:03d}.json'): open(f'{HERE}/units_before_repair/ebook-{o:03d}.json','w',encoding='utf-8').write(old)
            json.dump(u,open(src,'w'),ensure_ascii=False,indent=1); return o,'repaired'
        feedback=base+'\n\nThe automatic checker also rejected your latest rewrite:\n- '+'\n- '.join(problems[:30])
    return o,'FAILED'
if __name__=='__main__':
    workers=int(sys.argv[1]); todo=[int(x) for x in sys.argv[2:]] or sorted(U.plan)
    with cf.ThreadPoolExecutor(workers) as pool: res=list(pool.map(run,todo))
    print('DONE repaired',[o for o,v in res if v=='repaired'],'failed',[o for o,v in res if v=='FAILED'],flush=True)
