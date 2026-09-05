"""Deterministic, reviewed retrieval vocabulary and lesson-specific drill rubrics."""
import json,re
from pathlib import Path
out=Path(__file__).resolve().parents[1]/'internal/content'
lessons=json.loads((out/'lessons.json').read_text());scenes=json.loads((out/'scenarios.json').read_text())
terms='''hello=สวัสดี;I'm=ฉันคือ
call me=เรียกฉันว่า;name=ชื่อ
from=มาจาก;place=สถานที่
developer=นักพัฒนา;job=อาชีพ
nice to meet you=ยินดีที่ได้รู้จัก;meet=พบ
repeat=พูดซ้ำ;please=กรุณา
speak=พูด;slowly=ช้า ๆ
understand=เข้าใจ;word=คำ
mean=หมายความว่า;deadline=กำหนดส่ง
think=คิด;moment=สักครู่
designer=นักออกแบบ;our=ของพวกเรา
have=มี;question=คำถาม
need=ต้องการ;help=ความช่วยเหลือ
meeting room=ห้องประชุม;upstairs=ชั้นบน
three=สาม;people=ผู้คน
meeting=การประชุม;ten=สิบ
ready=พร้อม;start=เริ่ม
like=ชอบ;solving problems=แก้ปัญหา
works for me=ฉันสะดวกแบบนั้น;yes=ใช่
right room=ห้องที่ถูกต้อง;this=สิ่งนี้
usually=โดยปกติ;start work=เริ่มงาน
work on=ทำงานเกี่ยวกับ;mobile app=แอปมือถือ
team=ทีม;five=ห้า
explain=อธิบาย;basic flow=ขั้นตอนพื้นฐาน
want to=อยากจะ;confidently=อย่างมั่นใจ
responsible for=รับผิดชอบ;testing=การทดสอบ
report=รายงาน;when=เมื่อไร
join=เข้าร่วม;how=อย่างไร
example=ตัวอย่าง;rule=กฎ
mean=หมายถึง;tomorrow=พรุ่งนี้
yesterday=เมื่อวาน;tested=ทดสอบแล้ว
today=วันนี้;working on=กำลังทำ
check=ตรวจสอบ;results=ผลลัพธ์
finished=ทำเสร็จแล้ว;first draft=ร่างแรก
yet=ยัง;tests=ชุดทดสอบ
problem=ปัญหา;connection=การเชื่อมต่อ
send=ส่ง;link=ลิงก์
available=ว่าง;at two=ตอนบ่ายสองหรือสองนาฬิกาตามบริบท
think=คิดว่า;looks good=ดูดี
thank you for=ขอบคุณสำหรับ;process=ขั้นตอน
finished=ทำเสร็จแล้ว;next=ถัดไป
blocked by=ติดอยู่เพราะ;missing access=สิทธิ์เข้าถึงที่ยังไม่มี
completed=ทำเสร็จแล้ว;remains=ยังเหลือ
help me with=ช่วยฉันเรื่อง;error logs=บันทึกข้อผิดพลาด
should take=น่าจะใช้เวลา;about=ประมาณ
sign in=เข้าสู่ระบบ;choose=เลือก
transfers=โอน;recipient=ผู้รับ
balance=ยอดเงิน;check=ตรวจดู
verify your identity=ยืนยันตัวตน;before=ก่อน
transaction=รายการธุรกรรม;pending=รอดำเนินการ
could=อาจจะ;retry button=ปุ่มลองใหม่
faster=เร็วกว่า;option=ทางเลือก
suggest=เสนอแนะ;affects=ส่งผลต่อ
main benefit=ข้อดีหลัก;manual steps=ขั้นตอนที่คนทำเอง
downside=ข้อเสีย;delivery time=เวลาส่งมอบ
update=ข้อมูลความคืบหน้า;by Friday=ภายในวันศุกร์
agreed to=ตกลงที่จะ;first=ก่อน
clarify=อธิบายให้ชัด;expected result=ผลที่คาดหวัง
away=ไม่อยู่;handle support=ดูแลงานช่วยเหลือ
update on=ความคืบหน้าเกี่ยวกับ;approval=การอนุมัติ
consists of=ประกอบด้วย;database=ฐานข้อมูล
endpoint=จุดเรียกใช้ API;view transactions=ดูรายการธุรกรรม
as I understand it=ตามที่ฉันเข้าใจ;confirm=ยืนยัน
includes=รวมถึง;excludes=ไม่รวม
consider this done=ถือว่างานนี้เสร็จ;failed requests=คำขอที่ล้มเหลว
reduce=ลด;waiting time=เวลารอ
measure success=วัดความสำเร็จ;completion rate=อัตราการทำสำเร็จ
recommend=แนะนำ;given=เมื่อพิจารณาจาก
prioritize=ให้ความสำคัญก่อน;reliability=ความเชื่อถือได้
affect=ส่งผลกระทบ;existing customers=ลูกค้าปัจจุบัน
delays=ความล่าช้า;affecting=ส่งผลต่อ
time out=หมดเวลารอ;overloaded=รับภาระเกินกำลัง
temporary measure=มาตรการชั่วคราว;disabled=ปิดการใช้งานแล้ว
provide=ให้ข้อมูล;another update=ความคืบหน้าครั้งต่อไป
risk=ความเสี่ยง;traffic increases=ปริมาณการใช้งานเพิ่ม
challenge=ความท้าทาย;automated=ทำให้เป็นอัตโนมัติ
contribution=สิ่งที่ตนมีส่วนช่วย;led to=นำไปสู่
similar systems=ระบบที่คล้ายกัน;yet=ยัง
improve=ปรับปรุง;handover=การส่งมอบงาน
concerns=ข้อกังวล;rollout=การทยอยเปิดใช้งาน
caching=การเก็บข้อมูลไว้เรียกซ้ำ;maintenance=การดูแลรักษา
although=แม้ว่า;recovery time=เวลากู้คืน
open to=ยินดีพิจารณา;phases=ระยะต่าง ๆ
scope=ขอบเขต;extend the timeline=ขยายกำหนดเวลา
provided that=โดยมีเงื่อนไขว่า;rollback=การย้อนกลับรุ่นเดิม
reconciliation=การกระทบยอด;ledger entries=รายการในบัญชีแยกประเภท
even if=แม้ว่า;only once=เพียงครั้งเดียว
authorization=การอนุมัติรายการ;settlement=การชำระดุล
retain=เก็บรักษา;traced=ตรวจสอบย้อนหลังได้
verified=ตรวจสอบแล้ว;unconfirmed=ยังไม่ยืนยัน
agree on=ตกลงร่วมกันเรื่อง;outcome=ผลลัพธ์
bring this back to=พาประเด็นกลับมาที่;customer impact=ผลกระทบต่อลูกค้า
perspective=มุมมอง;operations=ฝ่ายปฏิบัติการ
differ on=เห็นต่างเรื่อง;timeline=กำหนดเวลา
to recap=ขอสรุปอีกครั้ง;validate=ตรวจสอบความถูกต้อง
queue=คิว;waiting line=แถวรอ
fair question=คำถามที่สมเหตุสมผล;recovery behavior=ลักษณะการกู้คืน
evidence=หลักฐาน;cause=สาเหตุ
in light of=เมื่อพิจารณาจาก;revise=ปรับแก้
recommendation=ข้อเสนอแนะ;mitigate=ลดผลกระทบ'''.splitlines()
assert len(terms)==100
for l,row in zip(lessons,terms):
 l['vocabulary']=[dict(term=t,meaning=m,example=l['example']) for t,m in (x.split('=',1) for x in row.split(';'))]
 slots=re.findall(r'\[([^]]+)\]',l['pattern'])
 target=f"สื่อเป้าหมาย: {l['objective']} ด้วยโครง {l['pattern']} ยอมรับรายละเอียดและถ้อยคำอื่นที่ถูกความหมาย ไม่บังคับตรงตัวอย่าง"
 l['drills'][1].update(prompt=f"ใช้โครง {l['pattern']} เปลี่ยน {', '.join(slots) if slots else 'บริบทที่นำวลีไปใช้'} เป็นข้อมูลของคุณ",target=target)
 # The communicative function stays fixed. No unrequested question/tense transformation.
 l['drills'][2].update(prompt=f"คุณกำลังคุยกับเพื่อนใหม่แทนผู้ฟังเดิม ใช้ {l['pattern']} เพื่อ{l['objective']} เปลี่ยนรายละเอียดให้เข้ากับผู้ฟัง โดยยังพูดจากมุมของคุณ",target=target+' ผู้พูดยังคงเป็นตัวเอง ไม่บังคับเปลี่ยนเป็นคำถามหรือบุรุษที่สาม')
 l['drills'][3].update(prompt=f"ลองตอบโดยไม่เปิดตัวอย่าง: {l['objective']} ใช้ข้อมูลใหม่หนึ่งอย่าง",target=target)
 if l['assessment']:
  earlier=lessons[max(0,l['ordinal']-5):l['ordinal']-1]
  l['acceptance']+=['ใช้รูปแบบก่อนหน้าอย่างน้อยหนึ่งรูปแบบในโจทย์ใหม่: '+' / '.join(x['pattern'] for x in earlier)]
 l['version']='2026-09-05.3'
# Ground scenario roles and bridge untaught everyday language before assessment.
for s in scenes:
 if s['category']=='Everyday':
  s['brief']+=' ก่อนเริ่มฉาก สอน starter pattern เป็นวลีสั้นพร้อมคำแปลไทย ให้ตัวเลือกคำศัพท์ 3 คำและซ้อมพูดหนึ่งครั้งโดยยังไม่นับ mastery แล้วจึงเริ่มโจทย์ใหม่'
 else:
  role={'Tech':'Engineer','Banking':'Banking specialist','Business':'Project owner','Interview':'Candidate','Meeting':'Meeting facilitator'}[s['category']]
  s['brief']+=f' คุณเล่นบท {role}; AI เล่นบทอื่นใน roles ทีละคนและระบุชื่อผู้พูด เป้าหมายของคุณ: '+s['goal']
  if s['category']=='Interview':s['opening']='Welcome. '+('What would you like to ask about our team?' if s['id']=='interview-10' else 'Please tell me about your experience relevant to '+s['title']+'.')
 if s['id'] in ['banking-03','tech-03','business-02']:s['level']='B2'
 if s['id']=='interview-03':s['level']='B1'
assert len({s['id'] for s in scenes})==70
for name,data in [('lessons',lessons),('scenarios',scenes)]: (out/(name+'.json')).write_text(json.dumps(data,ensure_ascii=False,indent=2)+'\n')
print('Refined 100 lesson rubrics, 200 vocabulary items, 70 scenario scaffolds')
