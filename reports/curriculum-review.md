# Curriculum QA: 100 core lessons, 50 work scenes, 20 everyday labs

วันที่ตรวจ: 2026-09-05  
ขอบเขต: อ่าน `internal/content/lessons.json`, `internal/content/scenarios.json`, `scripts/build_content.py` และ `scripts/enrich_content.py` แบบ static เท่านั้น ไม่เรียก API/AI และไม่ทดสอบผลลัพธ์ของผู้เรียน

## คำตัดสิน

ชุดเนื้อหานี้ **ผ่านด้านโครงสร้างและความกว้างของสถานการณ์** และมีแกน progression ที่มองเห็นได้จากวลีเอาตัวรอด ไปสู่งานประจำวัน การอธิบายระบบ/ความเสี่ยง และการเจรจา/นำประชุม แต่ **ยังไม่ผ่านในฐานะเส้นทางเรียนจากศูนย์ที่ฝึกซ้ำและประเมินได้สม่ำเสมอ** เพราะ drill และ active vocabulary ส่วนใหญ่เป็นแม่แบบที่ไม่มีคำตอบ/เกณฑ์เฉพาะบท และฉากหลายรายการไม่เชื่อมกับสิ่งที่สอนก่อนหน้า

ชื่อระดับ `Pre-A1`, `A1`, `A2`, `B1`, `B2` ในข้อมูลควรถือเป็นระดับภายในผลิตภัณฑ์เท่านั้น การตรวจนี้ไม่ได้เทียบมาตรฐานหรือรับรอง CEFR และจำนวน 100 บทไม่ใช่หลักฐานรับรองความคล่องในการสนทนา

## สิ่งที่ผ่านจริง

- มีบทหลัก 100 บทครบ: 5 ระดับ × 4 units × 5 บท ระดับละ 20 บท และ ID ไม่ซ้ำ
- มีฉากงาน 50 ฉาก ครบ Tech, Banking, Business, Interview และ Meeting อย่างละ 10 ฉาก พร้อมอีก 20 everyday labs รวม 70 ฉาก และ ID ไม่ซ้ำ
- ทุกบทมี drill 4 ชนิดครบเชิงโครงสร้าง: `shadowing`, `substitution`, `transformation`, `rapid_response`
- 97/100 patterns มีช่องแทนข้อมูล จึงมีฐานสำหรับฝึกนำไปใช้ใหม่ และทุกบทสั่งให้ทำ personal/everyday transfer, workplace transfer, follow-up และ novel scenario
- ลำดับภาพใหญ่สมเหตุผล: เริ่มจากการแนะนำตัว/ขอความช่วยเหลือ → ถามตอบและเวลา → อัปเดตงาน/ธนาคารพื้นฐาน → ระบบ ธุรกิจ incident และ interview → trade-off, banking risk, facilitation และการป้องกันข้อเสนอ
- Coaching notes แยกตามระดับและลดภาระผู้เริ่มต้นด้วยการแบ่งวลี 2–5 คำ รวมทั้งย้ำความหมายมากกว่าสำเนียง
- ฉากงานมี goal และ roles ทุกฉาก ส่วน everyday labs ระบุบท `Learner` ชัด และครอบคลุมร้านอาหาร การเดินทาง โรงแรม คลินิก การนัดหมาย การซื้อของ ที่อยู่อาศัย และการคุยกับเพื่อน

## ข้อบกพร่องที่กระทบการเรียนจริง

### P0 — drill มีชื่อครบ แต่ฝึก/ตรวจซ้ำไม่ได้อย่างอิสระ

- `substitution.target` และ `rapid_response.target` ว่างใน **100/100 บท**
- `transformation.target` เป็น meta-text ภาษาอังกฤษเดียวกันทั้ง 100 บท ไม่ใช่ตัวอย่างคำตอบหรือเกณฑ์เฉพาะ pattern
- Prompt ของ shadowing, substitution และ transformation เหมือนกันทุกบท ขณะที่ acceptance criteria มีชุดเดียวทั้ง 100 บท แม้ 20 บทที่ตั้ง `assessment=true`
- ผลคือผู้เรียนหรือระบบตรวจไม่รู้ว่า slot ต้องรับชนิดคำใด การเปลี่ยน tense/pronoun ใดถูก และ assessment ปลาย unit ไม่ได้วัดการสะสมทักษะต่างจากบทปกติ

ตัวอย่างที่เห็นชัด: `lesson-041` ชื่อ “Standup สามส่วน” แต่ pattern และ example มีเพียง done + next ไม่มี blocker; `lesson-046`, `lesson-049`, `lesson-069`, `lesson-081`, `lesson-086`, `lesson-088`, `lesson-094`, `lesson-100` ใช้ชื่อ slot ซ้ำจนแยกค่าคนละตำแหน่งไม่ได้

### P0 — `vocabulary` ยังไม่ใช่ active vocabulary

ทุกบทมี vocabulary เพียง 1 รายการ และใน **100/100 บท** `term` กับ `example` เป็นประโยคตัวอย่างเต็มประโยคเดียวกัน ไม่มีคำ/วลีใช้งาน ชนิดคำ รูปคำ หรือ collocation ให้เรียกคืน เช่น `lesson-001` เก็บทั้ง “Hello, I’m Pim.” เป็นคำศัพท์หนึ่งคำ และ `lesson-087` เก็บประโยค idempotency ทั้งประโยค

สิ่งนี้ทำให้ไม่สามารถวาง spaced retrieval, จำกัดศัพท์ใหม่ต่อบท หรือเช็กว่าผู้เรียนมีคำพอเติม pattern ได้จริง

### P1 — everyday labs ส่วนใหญ่เป็นเนื้อหาใหม่ มากกว่า transfer

มีเพียง `everyday-01` ที่ `starter_pattern` ตรงกับ core pattern แบบ exact match; อีก 19/20 labs นำ pattern ใหม่เข้ามา แม้หลายอันจะใช้โครงสร้างใกล้เคียงกัน ตัวอย่างสำคัญคือ:

- `everyday-02` ระดับ Pre-A1 ต้องใช้ “I’d like [drink], please.” เลือกขนาด ตอบรับราคา แต่ไม่มี core Pre-A1 บทใดสอน pattern การสั่งของ ขนาด ราคา หรือ `How much` ก่อน
- `everyday-08` ระดับ Pre-A1 ใช้ “Please take me to [place].” และให้เช็กค่าโดยสาร/จุดลง ซึ่งยังไม่มี scaffold ใน 20 core Pre-A1
- `everyday-03`–`everyday-06` ระดับ A1 เพิ่ม pattern ร้านอาหาร ถามทาง ซื้อตั๋ว และซื้อเสื้อผ้าใหม่พร้อมกัน

ดังนั้น breadth ชีวิตจริงดี แต่เส้นทาง “เรียนแล้วนำไปใช้” ยังขาด prerequisite mapping และ pre-lab rehearsal

### P1 — ระดับของฉากงานถูกกำหนดตามลำดับ ไม่ใช่ภาระภาษา

`build_content.py` กำหนดฉากลำดับ 1–3 ของทุกหมวดเป็น A2, 4–7 เป็น B1, 8–10 เป็น B2 โดยอัตโนมัติ ทำให้บางฉากขัดกับบทหลัก:

- `banking-03` เป็น A2 แต่ให้ “อธิบาย idempotency” ซึ่งเพิ่งสอนตรง ๆ ใน `lesson-087` ระดับ B2
- `tech-03` เป็น A2 แต่ให้เสนอ design และชั่ง trade-off ซึ่ง core วางไว้ชัดใน `lesson-081`–`lesson-085` ระดับ B2
- `business-02` เป็น A2 แต่ให้เจรจาขอบเขตกับ stakeholder ซึ่งตรงกับ `lesson-084` ระดับ B2
- `interview-03` เป็น A2 แต่ต้องเล่าความขัดแย้งและการหาข้อตกลง โดยไม่มี story scaffold ในระดับก่อน `lesson-076`

ผู้เรียนที่เลือกตามระดับจะเจอทั้งศัพท์เฉพาะและ discourse task ก่อนบทเตรียม

### P1 — work scenes มีบทบาท แต่ไม่บอกว่าผู้เรียนเล่นบทใด และ opening/criteria เป็นแม่แบบเดียว

ฉากงานทั้ง 50 ฉากไม่มี `learner_role` ต่างจาก everyday labs และฉากสามบทบาท เช่น `meeting-01`, `meeting-04`, `meeting-07`, `meeting-10` ยิ่งกำกวมว่า AI จะสวมบทใด

Opening ทั้ง 50 ฉากใช้ “Let’s discuss &lt;title&gt;. Could you start with a brief update?” และ success criteria สองข้อท้ายเหมือนกันทุกฉาก จึงไม่ตรงธรรมชาติของงานบางชนิด:

- `interview-01`: “Let’s discuss tell me about yourself” ไม่ใช่คำถามสัมภาษณ์ธรรมชาติ
- `interview-10`: ขอ “brief update” ทั้งที่เป้าหมายคือให้ candidate ถามทีม
- `meeting-04`: ไม่ระบุทางเลือกหรือ decision ที่ facilitator ต้องพาไปให้ถึง
- `meeting-10`: เริ่มจาก update แทนข้อมูลมติ/action ที่ต้องสรุป

### P1 — scaffolding สำหรับผู้เริ่มจากศูนย์ยังบาง

Pre-A1 ใช้วิธีจำ chunk ซึ่งเหมาะกับการให้พูดเร็ว แต่ core 20 บทเน้นวันทำงานมาก และยังไม่มีชุดพื้นฐานที่ everyday labs ต้องพึ่ง เช่น yes/no + polite refusal, ตัวเลข/ราคา, คำถาม `what/where/how much`, food/drink/transport nouns, singular/plural และตัวเลือกคำตอบสั้น ๆ

Coaching note บอกให้แบ่งวลี 2–5 คำ แต่ข้อมูลไม่มี chunk boundaries, comprehension check, controlled choices หรือ minimal response สำหรับแต่ละบท ผู้เริ่มจากศูนย์จึงอาจได้รับคำสั่ง “ใช้ข้อมูลของคุณ” ก่อนมีคำให้เลือก

### P2 — follow-up/transfer มีเจตนาดี แต่ยังเป็นคำสั่งให้ tutor สร้างเอง

ทุก lesson มีคำสั่งให้ถาม follow-up และทำ transfer สองบริบท แต่ไม่มี prompt, expected function หรือ rubric เฉพาะบทในข้อมูล จึงเสี่ยงต่อความยากและคุณภาพไม่คงที่ เช่น `lesson-007`, `lesson-010`, `lesson-019` ไม่มี slot แต่ substitution ยังคงสั่งให้ “เปลี่ยนข้อมูล”

### P2 — build pipeline ทำซ้ำไม่ได้อย่างปลอดภัย

`build_content.py` เขียน lessons และฉากงาน 50 ฉากใหม่ ส่วน `enrich_content.py` append everyday 20 ฉากโดยไม่ลบ/แทน ID เดิม ถ้ารัน enrich ซ้ำโดยไม่รัน build ก่อน จะได้ everyday ID ซ้ำและจำนวนฉากเพิ่ม อีกด้านหนึ่งถ้ารัน build อย่างเดียวจะทำให้ enrichment และ everyday labs หาย ควรมีคำสั่ง build เดียวและ assert จำนวน/ID หลังจบ

## ข้อความแก้ 5 จุดแรกที่เสนอให้รีวิว

### 1. ทำ drill ให้เฉพาะบทและตรวจได้ — `lesson-041`

แก้ชื่อ/pattern ให้ตรง “สามส่วน” และใส่ target ที่ใช้เป็นตัวอย่างหรือ rubric:

```json
{
  "title": "Standup สามส่วน",
  "pattern": "I finished [done]. Next, I’ll [next]. I’m blocked by [blocker].",
  "example": "I finished the form. Next, I’ll test it. I’m blocked by missing test data.",
  "drills": [
    {"kind":"shadowing","prompt":"ฟังเป็น 3 ช่วง: done → next → blocker แล้วพูดตาม","target":"I finished the form. Next, I’ll test it. I’m blocked by missing test data."},
    {"kind":"substitution","prompt":"เปลี่ยนเป็น report → send it → manager approval","target":"I finished the report. Next, I’ll send it. I’m blocked by manager approval."},
    {"kind":"transformation","prompt":"พูด standup ของอีกคน โดยเปลี่ยน I เป็น May/She","target":"May finished the form. Next, she’ll test it. She’s blocked by missing test data."},
    {"kind":"rapid_response","prompt":"What did you finish, what will you do next, and what is blocking you?","target":"ตอบครบ done, next และ blocker ภายใน 12 วินาที; ใช้รายละเอียดใหม่ ไม่ท่อง example"}
  ]
}
```

จากนั้นใช้หลักเดียวกันเติม target/rubric ให้ครบ 100 บท และทำ assessment ปลาย unit ให้ดึงอย่างน้อย 2–3 patterns ก่อนหน้า

### 2. เปลี่ยนประโยคซ้ำให้เป็น active vocabulary — `lesson-001` เป็นแม่แบบ

```json
"vocabulary": [
  {"term":"hello","meaning":"สวัสดี","example":"Hello!"},
  {"term":"I’m","meaning":"ฉันคือ / ฉันชื่อ","example":"I’m Pim."},
  {"term":"name","meaning":"ชื่อ","example":"My name is Pim."}
]
```

แต่ละบทควรมีคำ/วลีที่จำเป็นต่อ slot 3–6 รายการ และมีอย่างน้อยหนึ่งรายการทบทวนจากบทก่อนหน้า ไม่ควรใช้ประโยคตัวอย่างเต็มประโยคเป็น `term`

### 3. ทำ interview track ให้เป็นบทสนทนาจริง — `interview-01` และ `lesson-076`

```json
{
  "id":"interview-01",
  "learner_role":"Candidate",
  "opening":"Thanks for joining us. Could you introduce yourself and explain how your experience relates to this role?",
  "success_criteria":["แนะนำตัวสั้น ๆ","เชื่อมประสบการณ์หนึ่งเรื่องกับงาน","ตอบ follow-up โดยยกตัวอย่างจริงหรือสมมติอย่างชัดเจน"]
}
```

และแก้ pattern ของ `lesson-076` จาก challenge + action ซึ่งยังไม่ครบ STAR เป็น:

```text
The situation was [context]. My task was [task]. I [action], which led to [result].
```

### 4. แก้ level/context mismatch — `banking-03`

ทางเลือกที่ตรงกับ core มากที่สุดคือเปลี่ยนเป็น B2 และทำ opening ให้มี facts สำหรับอธิบาย idempotency:

```json
{
  "level":"B2",
  "learner_role":"Payment engineer",
  "opening":"The customer tapped Pay twice after a timeout, but the ledger shows one debit. Explain what we should verify and how idempotency should behave.",
  "success_criteria":["อธิบาย retry กับการประมวลผลครั้งเดียว","แยกสิ่งที่รู้จากสิ่งที่ต้องตรวจ","สรุปหลักฐานและขั้นตอนถัดไป"]
}
```

ถ้าต้องคง A2 ให้ตัดคำอธิบาย idempotency ออก และเปลี่ยน goal เป็นแจ้งว่าพบรายการซ้ำ/กำลังตรวจ/จะอัปเดตเมื่อใด

### 5. เติม pre-lab scaffold สำหรับผู้เริ่มจากศูนย์ — `everyday-02`

คงระดับ Pre-A1 ได้เมื่อ brief ให้ rehearsals ที่สอดคล้องกับเกณฑ์ครบ:

```json
{
  "brief":"ก่อนเริ่ม role-play ให้ฟังและพูดตามทีละวลี: ‘I’d like coffee, please.’ → ‘Small, please.’ → ‘How much is it?’ → ‘Thank you.’ จากนั้นปิดตัวอย่างและสั่งเครื่องดื่มใหม่หนึ่งรายการ ผู้สอนถามทีละข้อและให้ตัวเลือก 2 คำเมื่อผู้เรียนติด",
  "success_criteria":["สั่งเครื่องดื่มหนึ่งอย่าง","เลือกขนาด","ถามหรือรับรู้ราคา","กล่าวขอบคุณ"]
}
```

ใช้แนวเดียวกันกับ `everyday-08` โดย rehearse จุดหมาย, ค่าโดยสาร และ “Please stop here.” ก่อน role-play

## ลำดับแก้ที่แนะนำ

1. เติม drill targets/rubrics และ active vocabulary เพราะกระทบทุกบทและ assessment
2. ทำ prerequisite map ระหว่าง core lesson กับ scenario แล้วแก้ระดับ `banking-03`, `tech-03`, `business-02`, `interview-03`
3. เพิ่ม `learner_role`, role-specific opening และ role-specific success criteria ให้ work scenes โดยเริ่ม Interview/Meeting
4. เติม pre-lab rehearsal ให้ everyday labs ระดับ Pre-A1/A1 และระบุ core patterns ที่เป็น prerequisite
5. รวม build + enrich เป็นคำสั่ง idempotent พร้อม assert 100 lessons, 70 unique scenarios, 4 drill kinds และ non-empty targets

## ข้อจำกัดของผลตรวจ

รายงานนี้ยืนยันได้เฉพาะคุณภาพและความสอดคล้องของข้อมูลที่เขียนไว้ ไม่ได้ยืนยันว่า runtime tutor ทำตาม prompt, ระบบเสียงเหมาะกับผู้เริ่มต้น, rubric ถูกนำไปใช้จริง หรือผู้เรียนบรรลุผลลัพธ์หลังเรียน ต้องใช้ learner testing และผลการใช้งานจริงเพื่อประเมินสิ่งเหล่านั้น
