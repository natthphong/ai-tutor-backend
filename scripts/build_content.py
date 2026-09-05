"""Versioned original curriculum. Rebuild with python3 scripts/build_content.py."""
import json
from pathlib import Path
units = [
('Pre-A1','เริ่มพูดคำแรก', '''ทักทาย|Hello, I’m [name].|Hello, I’m Pim.|สวัสดี ฉันชื่อพิม
บอกชื่อเล่น|You can call me [name].|You can call me Aom.|เรียกฉันว่าออมได้
บอกที่มา|I’m from [place].|I’m from Bangkok.|ฉันมาจากกรุงเทพ
บอกอาชีพ|I’m a [job].|I’m a developer.|ฉันเป็นนักพัฒนา
จบบทสนทนา|Nice to meet you, [name].|Nice to meet you, Sam.|ยินดีที่ได้รู้จักแซม'''),
('Pre-A1','ขอความช่วยเหลือ', '''ขอให้พูดซ้ำ|Could you repeat [that], please?|Could you repeat that, please?|ช่วยพูดอีกครั้งได้ไหม
ขอให้พูดช้า|Could you speak more slowly?|Could you speak more slowly?|ช่วยพูดช้าลงได้ไหม
บอกว่ายังไม่เข้าใจ|I don’t understand [word].|I don’t understand this word.|ฉันไม่เข้าใจคำนี้
ถามความหมาย|What does [word] mean?|What does deadline mean?|deadline แปลว่าอะไร
ขอเวลาคิด|Let me think for a moment.|Let me think for a moment.|ขอคิดสักครู่'''),
('Pre-A1','คนและสิ่งรอบตัว', '''แนะนำเพื่อนร่วมงาน|This is [name], our [role].|This is Ben, our designer.|นี่คือเบน นักออกแบบของเรา
บอกสิ่งที่มี|I have [thing].|I have a question.|ฉันมีคำถาม
บอกสิ่งที่ต้องการ|I need [thing].|I need some help.|ฉันต้องการความช่วยเหลือ
บอกตำแหน่ง|The [thing] is [place].|The meeting room is upstairs.|ห้องประชุมอยู่ชั้นบน
บอกจำนวน|There are [number] [things].|There are three people.|มีคนสามคน'''),
('Pre-A1','วันทำงานแรก', '''บอกเวลา|The meeting is at [time].|The meeting is at ten.|ประชุมตอนสิบโมง
บอกความพร้อม|I’m ready to [action].|I’m ready to start.|ฉันพร้อมเริ่มแล้ว
บอกความชอบ|I like [activity].|I like solving problems.|ฉันชอบแก้ปัญหา
ตอบรับ|Yes, that works for me.|Yes, that works for me.|ได้ แบบนั้นสะดวกสำหรับฉัน
ถามอย่างง่าย|Is this [thing]?|Is this the right room?|นี่คือห้องที่ถูกต้องไหม'''),
('A1','เล่าเรื่องตัวเอง', '''กิจวัตร|I usually [action] at [time].|I usually start work at nine.|ฉันมักเริ่มงานเก้าโมง
งานที่รับผิดชอบ|I work on [area].|I work on the mobile app.|ฉันทำงานด้านแอปมือถือ
ทีมของฉัน|Our team has [number] [roles].|Our team has five developers.|ทีมเรามีนักพัฒนาห้าคน
ความสามารถ|I can [action].|I can explain the basic flow.|ฉันอธิบายขั้นตอนพื้นฐานได้
เป้าหมายส่วนตัว|I want to [goal].|I want to speak more confidently.|ฉันอยากพูดมั่นใจขึ้น'''),
('A1','ถามตอบในที่ทำงาน', '''ถามว่าใคร|Who is responsible for [area]?|Who is responsible for testing?|ใครรับผิดชอบการทดสอบ
ถามกำหนดส่ง|When do we need [thing]?|When do we need the report?|เราต้องใช้รายงานเมื่อไหร่
ถามวิธีทำ|How do I [action]?|How do I join the meeting?|ฉันเข้าประชุมอย่างไร
ขอตัวอย่าง|Can you give me an example of [thing]?|Can you give me an example of this rule?|ยกตัวอย่างกฎนี้ได้ไหม
เช็กความเข้าใจ|Do you mean [idea]?|Do you mean we start tomorrow?|หมายถึงเริ่มพรุ่งนี้ใช่ไหม'''),
('A1','พูดเรื่องเวลา', '''เมื่อวาน|Yesterday, I [past action].|Yesterday, I tested the login page.|เมื่อวานฉันทดสอบหน้าเข้าสู่ระบบ
วันนี้|Today, I’m working on [task].|Today, I’m working on the report.|วันนี้ฉันกำลังทำรายงาน
พรุ่งนี้|Tomorrow, I’ll [action].|Tomorrow, I’ll check the results.|พรุ่งนี้ฉันจะตรวจผล
สิ่งที่เสร็จแล้ว|I’ve finished [task].|I’ve finished the first draft.|ฉันทำร่างแรกเสร็จแล้ว
สิ่งที่ยังไม่เสร็จ|I haven’t finished [task] yet.|I haven’t finished the tests yet.|ฉันยังทดสอบไม่เสร็จ'''),
('A1','เริ่มคุยเรื่องงาน', '''บอกปัญหา|There’s a problem with [thing].|There’s a problem with the connection.|มีปัญหาการเชื่อมต่อ
ขอสิ่งของอย่างสุภาพ|Could you send me [thing]?|Could you send me the link?|ส่งลิงก์ให้หน่อยได้ไหม
นัดเวลา|Are you available at [time]?|Are you available at two?|คุณว่างตอนบ่ายสองไหม
แสดงความคิดเห็นง่ายๆ|I think [opinion].|I think this looks good.|ฉันคิดว่าแบบนี้ดูดี
ขอบคุณสำหรับความช่วยเหลือ|Thank you for [action].|Thank you for explaining the process.|ขอบคุณที่อธิบายขั้นตอน'''),
('A2','อัปเดตงานอย่างชัดเจน', '''Standup สามส่วน|I finished [done]. Next, I’ll [next].|I finished the form. Next, I’ll test it.|ทำแบบฟอร์มเสร็จ ต่อไปจะทดสอบ
บอกสิ่งที่ติดขัด|I’m blocked by [blocker].|I’m blocked by the missing access.|ฉันติดเพราะยังไม่มีสิทธิ์
อธิบายความคืบหน้า|We’ve completed [part], but [remaining].|We’ve completed the design, but testing remains.|ออกแบบเสร็จ แต่ยังต้องทดสอบ
ขอความช่วยเหลือเฉพาะจุด|Could you help me with [specific task]?|Could you help me with the error logs?|ช่วยดู error logs ได้ไหม
ประมาณเวลา|It should take about [duration].|It should take about two days.|น่าจะใช้ประมาณสองวัน'''),
('A2','อธิบายขั้นตอนและธนาคารพื้นฐาน', '''ลำดับขั้นตอน|First, [step]. Then, [step].|First, sign in. Then, choose an account.|เข้าสู่ระบบก่อน แล้วเลือกบัญชี
อธิบายการโอน|The customer transfers [amount] to [recipient].|The customer transfers money to another account.|ลูกค้าโอนเงินไปบัญชีอื่น
เช็กยอดเงิน|You can check [information] in [place].|You can check your balance in the app.|ดูยอดเงินได้ในแอป
บอกเงื่อนไข|You need to [action] before [action].|You need to verify your identity before opening an account.|ต้องยืนยันตัวตนก่อนเปิดบัญชี
แจ้งสถานะรายการ|The transaction is [status].|The transaction is still pending.|รายการยังรอดำเนินการ'''),
('A2','เสนอและเปรียบเทียบ', '''เสนอวิธีแก้|We could [option].|We could add a retry button.|เราอาจเพิ่มปุ่มลองใหม่
เปรียบเทียบสองทาง|[A] is [comparison] than [B].|This option is faster than the old process.|ทางนี้เร็วกว่าขั้นตอนเดิม
ให้เหตุผลง่ายๆ|I suggest [action] because [reason].|I suggest more testing because the change affects payments.|เสนอทดสอบเพิ่มเพราะกระทบการชำระเงิน
บอกข้อดี|The main benefit is [benefit].|The main benefit is fewer manual steps.|ข้อดีหลักคือลดขั้นตอนด้วยมือ
บอกข้อเสีย|The downside is [cost].|The downside is a longer delivery time.|ข้อเสียคือส่งมอบช้าลง'''),
('A2','คุยเพื่อให้งานเดินต่อ', '''ตกลงงานถัดไป|I’ll [action] by [time].|I’ll send the update by Friday.|จะส่งอัปเดตภายในวันศุกร์
ทวนสิ่งที่ตกลง|So, we agreed to [decision].|So, we agreed to test this first.|เราตกลงทดสอบสิ่งนี้ก่อน
ขอความชัดเจน|Could you clarify [point]?|Could you clarify the expected result?|อธิบายผลที่คาดหวังให้ชัดได้ไหม
แจ้งลางานและส่งต่อ|While I’m away, [person] will [action].|While I’m away, May will handle support.|ระหว่างไม่อยู่เมย์จะดูแลงานช่วยเหลือ
ติดตามอย่างสุภาพ|Do you have an update on [topic]?|Do you have an update on the approval?|มีความคืบหน้าเรื่องอนุมัติไหม'''),
('B1','อธิบายระบบและ requirement', '''อธิบายสถาปัตยกรรม|The system consists of [parts].|The system consists of an app, an API, and a database.|ระบบประกอบด้วยแอป API และฐานข้อมูล
อธิบาย API|This endpoint allows [user] to [action].|This endpoint allows customers to view transactions.|endpoint นี้ให้ลูกค้าดูรายการได้
ทวน requirement|As I understand it, [requirement].|As I understand it, users need to confirm before submitting.|ตามที่เข้าใจผู้ใช้ต้องยืนยันก่อนส่ง
ระบุขอบเขต|This includes [in], but excludes [out].|This includes transfers, but excludes international payments.|รวมโอนเงินแต่ไม่รวมจ่ายต่างประเทศ
Acceptance criteria|We’ll consider this done when [condition].|We’ll consider this done when all failed requests can be retried.|เสร็จเมื่อคำขอที่ล้มเหลวลองใหม่ได้ทั้งหมด'''),
('B1','ธุรกิจและการตัดสินใจ', '''เชื่อมงานกับเป้าหมาย|This helps us [goal] by [method].|This helps us reduce waiting time by automating checks.|ลดเวลารอด้วยการตรวจอัตโนมัติ
อธิบายตัวชี้วัด|We measure success by [metric].|We measure success by the completion rate.|วัดความสำเร็จด้วยอัตราทำรายการเสร็จ
เสนอทางเลือกพร้อมเหตุผล|I recommend [option] given [constraint].|I recommend a smaller release given the deadline.|แนะนำปล่อยชุดเล็กตามข้อจำกัดเวลา
จัดลำดับความสำคัญ|We should prioritize [item] over [item].|We should prioritize reliability over new features.|ควรให้ความสำคัญความเสถียรมากกว่าฟีเจอร์ใหม่
ถามผลกระทบ|How would this affect [stakeholder]?|How would this affect existing customers?|จะกระทบลูกค้าเดิมอย่างไร'''),
('B1','รับมือ incident และความเสี่ยง', '''แจ้งเหตุอย่างกระชับ|We’re seeing [issue] affecting [scope].|We’re seeing delays affecting mobile transfers.|พบความล่าช้ากระทบโอนผ่านมือถือ
แยกข้อเท็จจริงกับสมมติฐาน|We know [fact]. We’re checking whether [hypothesis].|We know requests time out. We’re checking whether the database is overloaded.|รู้ว่าคำขอหมดเวลา กำลังตรวจฐานข้อมูล
อธิบายการบรรเทาปัญหา|As a temporary measure, we’ve [action].|As a temporary measure, we’ve disabled the new flow.|ปิดขั้นตอนใหม่ชั่วคราว
ให้เวลารายงานครั้งถัดไป|We’ll provide another update by [time].|We’ll provide another update by three.|จะอัปเดตอีกครั้งภายในบ่ายสาม
ระบุความเสี่ยง|There’s a risk that [event] if [condition].|There’s a risk that transactions fail if traffic increases.|มีความเสี่ยงทำรายการล้มเหลวหากคนใช้เพิ่ม'''),
('B1','สัมภาษณ์และสื่อสารร่วมทีม', '''เล่าประสบการณ์ STAR|The challenge was [problem], so I [action].|The challenge was slow delivery, so I automated the tests.|ปัญหาคือส่งมอบช้า จึงทำทดสอบอัตโนมัติ
อธิบายผลงาน|My contribution was [action], which led to [result].|My contribution was simplifying the flow, which led to fewer errors.|ฉันทำขั้นตอนให้ง่ายจึงเกิดข้อผิดพลาดน้อยลง
ยอมรับสิ่งที่ยังไม่รู้|I haven’t worked with [topic] yet, but [related experience].|I haven’t worked with that tool yet, but I’ve used similar systems.|ยังไม่ใช้เครื่องมือนั้นแต่เคยใช้ระบบคล้ายกัน
ให้ feedback อย่างสุภาพ|One thing we could improve is [area].|One thing we could improve is the handover process.|สิ่งที่ปรับปรุงได้คือขั้นตอนส่งต่องาน
ขอความคิดเห็น|What concerns do you have about [proposal]?|What concerns do you have about this rollout?|มีข้อกังวลอะไรกับการปล่อยระบบนี้'''),
('B2','เจรจา trade-off', '''ชั่งน้ำหนักทางเลือก|While [A] offers [benefit], [B] would [benefit].|While caching offers speed, a simpler design would reduce maintenance.|cache เร็วแต่แบบง่ายดูแลง่ายกว่า
ค้านอย่างสร้างสรรค์|I see the benefit, although I’m concerned about [risk].|I see the benefit, although I’m concerned about recovery time.|เห็นข้อดีแต่กังวลเวลากู้คืน
เสนอข้อตกลงกลาง|Would you be open to [compromise]?|Would you be open to releasing this in two phases?|พิจารณาปล่อยสองระยะได้ไหม
เจรจาขอบเขตและเวลา|If we keep [scope], we’ll need to [trade-off].|If we keep the full scope, we’ll need to extend the timeline.|ถ้าคงขอบเขตเต็มต้องขยายเวลา
ระบุเงื่อนไขตัดสินใจ|I’d support [option], provided that [condition].|I’d support the rollout, provided that we have a tested rollback.|เห็นด้วยหากมี rollback ที่ทดสอบแล้ว'''),
('B2','Banking และการสื่อสารความเสี่ยง', '''อธิบาย reconciliation|Reconciliation ensures that [records] match [records].|Reconciliation ensures that ledger entries match settlement records.|reconciliation ตรวจให้รายการบัญชีตรงกับรายการชำระดุล
อธิบาย idempotency|Even if [event], the system should [invariant].|Even if a request is retried, the system should process the payment only once.|ลองคำขอใหม่ก็ต้องตัดเงินครั้งเดียว
แยก authorization กับ settlement|[Stage] confirms [meaning], whereas [stage] [meaning].|Authorization confirms approval, whereas settlement moves the funds.|authorization อนุมัติ ส่วน settlement เคลื่อนเงิน
อธิบาย audit trail|We need to retain [evidence] so that [purpose].|We need to retain approval records so that changes can be traced.|เก็บหลักฐานอนุมัติเพื่อตรวจย้อนการเปลี่ยนแปลง
สื่อสารข้อจำกัดอย่างระมัดระวัง|Based on what we’ve verified, [fact]; [unknown] remains unconfirmed.|Based on what we’ve verified, balances are correct; the delay remains unexplained.|ตรวจแล้วว่ายอดถูก แต่สาเหตุล่าช้ายังไม่ยืนยัน'''),
('B2','นำประชุมและนำเสนอ', '''เปิดประชุมด้วยผลลัพธ์|By the end of this meeting, we need to agree on [outcome].|By the end of this meeting, we need to agree on the release scope.|จบประชุมต้องตกลงขอบเขตปล่อยระบบ
ดึงกลับเข้าประเด็น|Could we bring this back to [decision]?|Could we bring this back to the customer impact?|กลับมาเรื่องผลกระทบลูกค้าได้ไหม
ชวนคนอื่นแสดงความเห็น|We haven’t heard from [role] yet; what’s your perspective?|We haven’t heard from operations yet; what’s your perspective?|ยังไม่ฟังฝ่ายปฏิบัติการ คิดเห็นอย่างไร
สรุปประเด็นต่างกัน|It sounds like we agree on [point], but differ on [point].|It sounds like we agree on the goal, but differ on the timeline.|เห็นตรงกันเรื่องเป้าแต่ต่างกันเรื่องเวลา
ปิดประชุมพร้อม owner|To recap, [person] will [action] by [date].|To recap, Ben will validate the numbers by Thursday.|สรุปเบนตรวจตัวเลขภายในพฤหัส'''),
('B2','อธิบายเรื่องซับซ้อนและป้องกันข้อเสนอ', '''อธิบายให้ผู้ฟังต่างสาย|A useful way to think about [concept] is [analogy].|A useful way to think about a queue is a waiting line.|นึกถึง queue เป็นแถวรอ
รับคำถามท้าทาย|That’s a fair question. The reason we chose [option] is [evidence].|That’s a fair question. The reason we chose this design is its recovery behavior.|คำถามสมเหตุผล เลือกแบบนี้เพราะการกู้คืน
ตอบเมื่อข้อมูลไม่พอ|I don’t have enough evidence to confirm [claim], but I can [next step].|I don’t have enough evidence to confirm the cause, but I can review the logs.|ยังยืนยันสาเหตุไม่ได้แต่จะตรวจ log
ปรับข้อเสนอตามข้อมูล|In light of [evidence], I’d revise [proposal] to [change].|In light of the test results, I’d revise the rollout to start with a smaller group.|ตามผลทดสอบควรเริ่มกลุ่มเล็กลง
สรุปข้อเสนอแบบผู้บริหาร|The recommendation is [action]. The trade-off is [cost], and we’ll mitigate it by [action].|The recommendation is a phased release. The trade-off is a slower launch, and we’ll mitigate it by prioritizing core features.|แนะนำปล่อยเป็นระยะ ช้าขึ้นแต่เน้นฟีเจอร์หลักก่อน'''),
]
lessons=[]
for ui,(level,unit,rows) in enumerate(units):
 for li,row in enumerate(rows.splitlines()):
  title,pattern,example,thai=row.split('|');n=len(lessons)+1
  lessons.append(dict(id=f'lesson-{n:03}',ordinal=n,level=level,unit=ui%4+1,unit_title=unit,title=title,objective=f'พูดเรื่อง{title}ได้ด้วยตัวเอง และตอบคำถามต่อยอดได้',pattern=pattern,example=example,meaning=thai,explanation=f'ใช้ “{pattern}” เพื่อ{title} เริ่มจากฟังตัวอย่าง แล้วแทนข้อมูลให้เป็นเรื่องของคุณ ฝึกพูดทั้งประโยคโดยไม่อ่านคำตอบ',vocabulary=[dict(term=example,meaning=thai,example=example)],drills=[dict(kind='shadowing',prompt='ฟังแล้วพูดตามให้สื่อความหมายชัดเจน',target=example),dict(kind='substitution',prompt='ใช้ pattern เดิม เปลี่ยนข้อมูลให้เป็นสถานการณ์ของคุณ',target=''),dict(kind='transformation',prompt='เปลี่ยนประโยคตัวอย่างเป็นคำถามที่ใช้ถามเพื่อนร่วมงาน โดยรักษาหัวข้อเดิม',target=''),dict(kind='rapid_response',prompt=f'เพื่อนร่วมงานขอให้คุณ{title} ตอบทันทีด้วยข้อมูลของคุณ',target='')],conversation_prompt=f'Guide the learner to practice {title} in an original everyday/workplace exchange. Start with one short question, adapt to {level}. Require the target pattern naturally: {pattern}.',acceptance=['สื่อสารเป้าหมายได้โดยไม่อ่านเฉลย','ใช้ pattern ถูกบริบท','ตอบโจทย์ใหม่ได้ด้วยเสียงอย่างน้อยสองครั้ง'],version='2026-09-05.1'))
scenes={
'Tech': [('Daily standup','อัปเดตงาน สิ่งที่จะทำต่อ และ blocker','Developer, Scrum master'),('API walkthrough','อธิบาย input output และ error ของ API','Developer, Integration engineer'),('Design review','เสนอแบบระบบและชั่ง trade-off','Engineer, Architect'),('Production incident','แจ้งขอบเขตปัญหา ข้อเท็จจริง และการบรรเทา','Engineer, Incident lead'),('Code review discussion','อธิบายเหตุผลการแก้โค้ดและรับข้อเสนอ','Developer, Reviewer'),('Release readiness','ยืนยันผลทดสอบ ความเสี่ยง และ rollback','Engineer, Release manager'),('Performance investigation','แยกหลักฐานจากสมมติฐานของระบบช้า','Engineer, SRE'),('Database migration','อธิบายการย้ายข้อมูลและตรวจความถูกต้อง','Engineer, DBA'),('Security finding','อธิบายผลกระทบและแผนแก้ช่องโหว่','Developer, Security reviewer'),('Technical handover','ส่งต่องานและชี้สิ่งที่ต้องติดตาม','Engineer, New teammate')],
'Banking':[('Account opening','อธิบายขั้นตอนเปิดบัญชีและยืนยันตัวตน','Product engineer, Operations'),('Transfer pending','อธิบายรายการโอนที่ยังรอและสิ่งที่ตรวจได้','Support engineer, Customer support'),('Duplicate payment','อธิบาย idempotency และการตรวจรายการซ้ำ','Payment engineer, Banking operations'),('Ledger reconciliation','สรุปยอดไม่ตรงและแผนตรวจ reconciliation','Ledger engineer, Finance'),('Settlement delay','แยก authorization และ settlement ให้ชัด','Payment engineer, Business lead'),('Interest calculation','อธิบายสมมติฐานการคำนวณโดยไม่เดากฎจริง','Engineer, Product analyst'),('Transaction limits','ทวน requirement วงเงินและกรณีขอบ','Engineer, BA'),('Audit evidence','อธิบายหลักฐานและการตรวจย้อนรายการ','Engineer, Auditor'),('Banking outage update','แจ้งผลกระทบผู้ใช้โดยไม่สรุปเกินหลักฐาน','Incident lead, Operations'),('Core banking integration','อธิบายสัญญา API ความผิดพลาด และการกู้คืน','Integration engineer, Vendor')],
'Business':[('Product proposal','เสนอคุณค่า ปัญหาผู้ใช้ และวิธีวัดผล','Product contributor, Sponsor'),('Scope negotiation','เจรจาขอบเขตให้เหมาะกับเวลา','Team lead, Stakeholder'),('Requirements workshop','ถามข้อมูลที่ขาดและทวน acceptance criteria','BA, Product owner'),('Project delay','อธิบายเหตุล่าช้าและเสนอทางเลือก','Project contributor, Manager'),('Priority discussion','จัดลำดับงานตามผลกระทบและต้นทุน','Team member, Product manager'),('Customer journey','อธิบายขั้นตอนผู้ใช้และจุดติดขัด','Analyst, Designer'),('Metric review','ตีความตัวชี้วัดโดยไม่สรุปเหตุผลเกินข้อมูล','Analyst, Business lead'),('Vendor evaluation','เปรียบเทียบผู้ให้บริการด้วยเกณฑ์เดียวกัน','Evaluator, Procurement'),('Stakeholder update','สรุปความคืบหน้าและสิ่งที่ต้องตัดสินใจ','Project lead, Sponsor'),('Cost-benefit discussion','อธิบายข้อดี ค่าใช้จ่าย และสมมติฐาน','Contributor, Finance manager')],
'Interview':[('Tell me about yourself','แนะนำตัวโดยเชื่อมประสบการณ์กับงาน','Candidate, Interviewer'),('A difficult bug','เล่าปัญหา วิธีคิด และผลลัพธ์แบบ STAR','Candidate, Engineer'),('Team disagreement','เล่าความขัดแย้งและการหาข้อตกลง','Candidate, Hiring manager'),('System design interview','ถาม constraint ก่อนเสนอแบบ','Candidate, Architect'),('A project you led','ระบุบทบาทจริงและผลกระทบ','Candidate, Manager'),('Learning something new','อธิบายวิธีเรียนและประยุกต์ใช้','Candidate, Interviewer'),('Handling failure','รับผิดชอบและเล่าบทเรียนจากความผิดพลาด','Candidate, Manager'),('Banking domain experience','อธิบายประสบการณ์โดยแยกสิ่งที่รู้กับสิ่งที่ยังไม่รู้','Candidate, Banking lead'),('Why this role','เชื่อมแรงจูงใจกับบทบาทและการพัฒนา','Candidate, Recruiter'),('Questions for the team','ถามทีมเรื่องงาน ความคาดหวัง และวิธีทำงาน','Candidate, Team lead')],
'Meeting':[('Kickoff meeting','กำหนดเป้าหมาย ขอบเขต และเจ้าของงาน','Facilitator, Engineer, Product owner'),('Sprint planning','ชี้ dependency และตกลงงานที่ทำได้','Developer, Scrum master, BA'),('Cross-team dependency','ตกลงสัญญาส่งต่องานและเวลา','Team representative, Partner team'),('Decision meeting','ชวนเปรียบเทียบทางเลือกและบันทึกมติ','Facilitator, Architect, Business lead'),('Retrospective','ให้ feedback โดยมุ่งปรับกระบวนการ','Team member, Facilitator'),('Executive update','สรุปสั้นพร้อมการตัดสินใจที่ต้องการ','Project lead, Executive'),('Risk review','จัดความเสี่ยงพร้อม owner และวิธีลด','Engineer, Risk owner, Product lead'),('Demo and questions','สาธิตงานและตอบข้อสงสัย','Presenter, Stakeholder'),('Difficult stakeholder','รับความกังวลแล้วนำกลับสู่ข้อเท็จจริง','Engineer, Stakeholder'),('Meeting wrap-up','สรุปมติ action owner และ deadline','Facilitator, Engineer, Business lead')]
}
scenarios=[]
for category,rows in scenes.items():
 for i,(title,goal,roles) in enumerate(rows):
  scenarios.append(dict(id=f'{category.lower()}-{i+1:02}',title=title,category=category,level='A2' if i<3 else 'B1' if i<7 else 'B2',goal=goal,roles=roles.split(', '),brief=f'ฝึก{goal} ในสถานการณ์สมมติที่ไม่มีข้อมูลลูกค้าจริง',opening=f'Let’s discuss {title.lower()}. Could you start with a brief update?',success_criteria=[goal,'ถามเพื่อยืนยันความเข้าใจ','สรุปสิ่งที่ตกลงและขั้นตอนถัดไป'],minutes=10))
assert len(lessons)==100 and len(scenarios)==50
out=Path(__file__).resolve().parents[1]/'internal/content'
(out/'lessons.json').write_text(json.dumps(lessons,ensure_ascii=False,indent=2)+'\n')
(out/'scenarios.json').write_text(json.dumps(scenarios,ensure_ascii=False,indent=2)+'\n')
print(f'{len(lessons)} lessons, {len(scenarios)} scenarios')
