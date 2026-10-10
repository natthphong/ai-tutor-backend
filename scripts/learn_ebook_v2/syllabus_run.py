"""Theme + 10 globally new vocabulary terms per unit, written by Codex.

A batch is generated once; after that only the individual terms that collide
with an earlier unit are replaced, so a batch always converges.
"""
import json, re, sys, os, time
from codex_call import call
HERE=os.path.dirname(os.path.abspath(__file__))
units=sorted(json.load(open(f'{HERE}/units_plan.json')),key=lambda u:u['path_order'])
PROMPT=open(f'{HERE}/syllabus_prompt.md').read()
RULE={'A1':'A1: only very common words a beginner needs first (roughly the 800 most frequent English words plus basic daily-life words such as breakfast, bus, cheap, tired).',
      'A2':'A2: common words of roughly the 800-2000 frequency band.',
      'B1':'B1: words and collocations of roughly the 2000-3500 frequency band, including common phrasal verbs.',
      'B2':'B2: words and collocations of roughly the 3500-5000 frequency band that are useful at work and in discussion.'}
MIX={'A1':(80,20),'A2':(65,35),'B1':(55,45),'B2':(45,55)}
def norm(t): return re.sub(r'[^a-z ]','',t.lower()).strip()
chunks=[]; cur=[]
for u in units:
    if cur and (cur[0]['level']!=u['level'] or len(cur)>=22): chunks.append(cur); cur=[]
    cur.append(u)
chunks.append(cur)
def collisions(batch,used):
    """(ordinal, index, term, reason) for every term that must be replaced."""
    bad=[]; seen={}
    for x in batch:
        for i,v in enumerate(x['vocabulary']):
            k=norm(v['term'])
            if not k: bad.append((x['ordinal'],i,v['term'],'is empty'))
            elif k in used: bad.append((x['ordinal'],i,v['term'],f'is already taught in unit {used[k]}'))
            elif k in seen: bad.append((x['ordinal'],i,v['term'],f'is also in unit {seen[k]} of this batch'))
            else: seen[k]=x['ordinal']
    return bad
used={}; result={}
for ci,chunk in enumerate(chunks):
    path=f'{HERE}/syllabus/chunk_{ci:02d}.json'; draft=f'{HERE}/syllabus/draft_{ci:02d}.json'
    if os.path.exists(path):
        batch=json.load(open(path))['units']
    else:
        lvl=chunk[0]['level']; topic={u['ordinal']:u['topic'] for u in chunk}
        if os.path.exists(draft): batch=json.load(open(draft))['units']
        else:
            batch=None
            for attempt in range(5):
                prompt=PROMPT.replace('{EVERYDAY_PCT}',str(MIX[lvl][0])).replace('{WORK_PCT}',str(MIX[lvl][1])).replace('{LEVEL_RULE}',RULE[lvl]).replace('{USED}',', '.join(sorted(used)) or '(none yet)').replace('{UNITS}','\n'.join(f"{u['ordinal']} | {u['level']} | {u['topic']}" for u in chunk))
                t=time.time()
                try: data=call(prompt,f'{HERE}/syllabus_schema.json')
                except Exception as e: print('chunk',ci,'ERROR',str(e)[:300],flush=True); time.sleep(15); continue
                got={x['ordinal']:x for x in data['units']}
                if all(o in got and len(got[o]['vocabulary'])==10 for o in topic):
                    batch=[got[u['ordinal']] for u in chunk]; break
                print('chunk',ci,'attempt',attempt,'incomplete batch, retrying',flush=True)
            if batch is None: sys.exit(f'chunk {ci} failed to generate')
            print('chunk',ci,lvl,len(chunk),'units generated in',int(time.time()-t),'s',flush=True)
            json.dump({'units':batch},open(draft,'w'),ensure_ascii=False,indent=1)
        for rnd in range(25):
            bad=collisions(batch,used)
            print('chunk',ci,'round',rnd,'terms to replace',len(bad),flush=True)
            if not bad: break
            by={x['ordinal']:x for x in batch}
            taken=set(used)|{norm(v['term']) for x in batch for v in x['vocabulary']}
            lines=[]
            for o,i,term,why in bad:
                x=by[o]; keep=[v['term'] for j,v in enumerate(x['vocabulary']) if j!=i]
                lines.append(f"- unit {o} | grammar: {topic[o]} | theme: {x['theme_en']} | other words of this unit: {', '.join(keep)} | REPLACE \"{term}\" ({why})")
            prompt=("You are an expert EFL curriculum designer for Thai adult learners. Do not run shell commands or read files; reply with the final JSON only.\n\n"
                    f"Replace single vocabulary terms in a {lvl} course syllabus. For each line give ONE new term that fits the unit's theme and grammar point, is a frequent, useful word or collocation at this level ({RULE[lvl]}), in dictionary form (lowercase, no article, no 'to'), with pos (noun, verb, adjective, adverb, phrase, phrasal verb) and a short Thai meaning. "
                    "The new term must NOT be any word in the TAKEN list, must not repeat another replacement in your answer, and must not be an inflected form of a taken word. Copy 'ordinal' and 'old' exactly from the line.\n\nLINES:\n"+'\n'.join(lines)+"\n\nTAKEN (never use): "+', '.join(sorted(taken)))
            try: data=call(prompt,f'{HERE}/replace_schema.json')
            except Exception as e: print('chunk',ci,'replace ERROR',str(e)[:300],flush=True); time.sleep(15); continue
            want={(o,norm(term)):i for o,i,term,_ in bad}
            for r in data['replacements']:
                i=want.get((r['ordinal'],norm(r['old'])))
                if i is None or not norm(r['term']) or norm(r['term']) in taken: continue
                taken.add(norm(r['term']))
                by[r['ordinal']]['vocabulary'][i]={'term':r['term'],'pos':r['pos'],'meaning_th':r['meaning_th']}
            json.dump({'units':batch},open(draft,'w'),ensure_ascii=False,indent=1)
        else: sys.exit(f'chunk {ci} did not converge')
        json.dump({'units':batch},open(path,'w'),ensure_ascii=False,indent=1)
        if os.path.exists(draft): os.remove(draft)
    for x in batch:
        result[x['ordinal']]=x
        for v in x['vocabulary']: used[norm(v['term'])]=x['ordinal']
json.dump(result,open(f'{HERE}/syllabus.json','w'),ensure_ascii=False,indent=1)
print('DONE units',len(result),'terms',len(used),flush=True)
