#!/usr/bin/env python3
"""Import the supplied book locally. Generated copyrighted pages/text stay out of Git."""
import argparse, hashlib, json, re, subprocess
from pathlib import Path
import pdfplumber
p=argparse.ArgumentParser();p.add_argument('pdf');p.add_argument('--output',default='.ebook');p.add_argument('--skip-images',action='store_true');a=p.parse_args()
src=Path(a.pdf);out=Path(a.output);out.mkdir(parents=True,exist_ok=True)
version=hashlib.sha256(src.read_bytes()).hexdigest()[:16]
subprocess.run(['pdftotext','-layout',str(src),str(out/'source.txt')],check=True)
texts=(out/'source.txt').read_text().split('\f')
book=pdfplumber.open(src)
# Key columns start at the coloured UNIT labels, rather than one-third of the media box.
keys={};key_pages={};current=None
for number in range(347,379):
 page=book.pages[number];words=page.extract_words()
 starts=sorted({round(w['x0'],0) for w in words if w['text']=='UNIT'})
 # Standard layout uses three columns at about 66, 225 and 383 pt.
 boundaries=[55,214,369,page.width-19]
 for col in range(3):
  text=page.crop((boundaries[col],115 if number==347 else 45,boundaries[col+1],page.height-33)).extract_text(x_tolerance=1) or ''
  for line in text.splitlines():
   m=re.match(r'^UNIT\s+(\d+)\s*$',line)
   if m:current=int(m[1])
   if current and current<=145:
    keys.setdefault(current,[]).append(line);key_pages.setdefault(current,set()).add(number+1)
units=[]
for n in range(1,146):
 lesson_page=12+2*n;exercise_page=lesson_page+1
 page=book.pages[exercise_page-1];words=page.extract_words(extra_attrs=['non_stroking_color'])
 headings=sorted([w for w in words if re.fullmatch(fr'{n}\.\d+',w['text'])],key=lambda w:w['top'])
 groups=[]
 for i,h in enumerate(headings):
  bottom=headings[i+1]['top'] if i+1<len(headings) else page.height-32
  nums=set()
  for w in words:
   color=w.get('non_stroking_color') or ()
   if h['top']<w['top']<bottom and re.fullmatch(r'\d{1,2}',w['text']) and len(color)==4 and .4<float(color[0])<.7 and float(color[1])<.05 and .2<float(color[2])<.4:
    nums.add(int(w['text']))
  groups.append({'id':h['text'],'item_ids':[f"{h['text']}:{x}" for x in sorted(nums)] or [h['text']+':all']})
 title=re.search(fr'^\s*{n}\s+(.+)$',texts[lesson_page-1],re.M)
 units.append({'id':str(n),'number':n,'title':title.group(1).strip() if title else f'Unit {n}', 'lesson_page':lesson_page,'exercise_page':exercise_page,'answer_pages':sorted(key_pages.get(n,set())), 'sections':groups,'lesson_text':re.sub(r' +',' ',texts[lesson_page-1]),'exercise_text':re.sub(r' +',' ',texts[exercise_page-1]),'answer_text':'\n'.join(keys.get(n,[]))})
manifest={'id':'grammar-in-use','title':'English Grammar in Use · Learn Ebook','version':version,'page_count':len(book.pages),'units':units}
assert len(units)==145 and all(u['sections'] and u['answer_text'] for u in units), 'incomplete source extraction: '+str([(u['id'],bool(u['sections']),bool(u['answer_text'])) for u in units if not u['sections'] or not u['answer_text']])
(out/'manifest.json').write_text(json.dumps(manifest,ensure_ascii=False,separators=(',',':')))
if not a.skip_images:
 subprocess.run(['pdftoppm','-scale-to','1400','-jpeg','-jpegopt','quality=80',str(src),str(out/'page')],check=True)
print(json.dumps({'pages':len(book.pages),'units':len(units),'questions':sum(len(s['item_ids']) for u in units for s in u['sections']),'version':version}))
