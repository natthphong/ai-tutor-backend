"""Original everyday transfer labs and deliberate practice prompts; no runtime AI seed generation."""
import json,re
from pathlib import Path
out=Path(__file__).resolve().parents[1]/'internal/content'
lessons=json.loads((out/'lessons.json').read_text());scenes=[s for s in json.loads((out/'scenarios.json').read_text()) if s['category']!='Everyday']
everyday=[
('First hello','Pre-A1','แนะนำตัวและถามชื่อคนที่เพิ่งเจอ','New neighbor','Hi! I’m Alex. What’s your name?','Hello, I’m [name].','ทักทาย บอกชื่อ และถามชื่อกลับ'),
('A coffee please','Pre-A1','สั่งเครื่องดื่ม ระบุขนาด และขอบคุณ','Barista','Hello! What would you like?','I’d like [drink], please.','สั่งเครื่องดื่มหนึ่งอย่าง เลือกขนาด และตอบรับราคา'),
('At the restaurant','A1','สั่งอาหารและบอกสิ่งที่ไม่กิน','Server','Welcome! Are you ready to order?','Could I have [dish] without [ingredient]?','สั่งอาหาร ถามส่วนผสม และขอเช็กบิล'),
('Ask for directions','A1','ถามทางแล้วทวนเส้นทางให้ถูก','Local resident','Hello! Do you need help finding somewhere?','How do I get to [place]?','บอกจุดหมาย ถามจุดสังเกต และทวนทาง'),
('Buying a train ticket','A1','บอกปลายทาง เวลา และจำนวนตั๋ว','Ticket clerk','Where would you like to go?','I need [number] tickets to [place].','เลือกปลายทาง ถามเวลาออกและราคา'),
('Shopping for a size','A1','ถามขนาด ราคา และตัดสินใจซื้อ','Shop assistant','Hi! Can I help you find anything?','Do you have this in [size/color]?','ถามขนาดที่ต้องการ ลองสินค้า และตอบว่ารับหรือไม่รับ'),
('Checking in','A2','เช็กอินโรงแรมและถามข้อมูลที่พัก','Receptionist','Welcome! Do you have a reservation?','I have a reservation under [name].','ให้ชื่อ ถามเวลาอาหารเช้า และแจ้งคำขอหนึ่งอย่าง'),
('A taxi ride','Pre-A1','บอกจุดหมายและจุดลง','Driver','Hello! Where are you going?','Please take me to [place].','บอกจุดหมาย ขอเช็กค่าโดยสาร และจุดลง'),
('Weekend plans','A1','ชวนเพื่อนและนัดเวลา','Friend','Do you have any plans for Saturday?','Would you like to [activity] on [day]?','ชวนทำกิจกรรม ถามเวลาว่าง และยืนยันสถานที่'),
('Tell me about yesterday','A2','เล่าเหตุการณ์ที่ผ่านมาเป็นลำดับ','Friend','How was your day yesterday?','First, I [past action]. Then, I [past action].','เล่าอย่างน้อยสองเหตุการณ์และความรู้สึก'),
('Making an appointment','A2','นัดหมายและขอเปลี่ยนเวลา','Receptionist','Good morning! How can I help?','Could I make an appointment for [day/time]?','ขอเวลานัด รับหรือปฏิเสธตัวเลือก และทวนเวลา'),
('At the clinic','A2','ฝึกภาษาเล่าอาการและถามให้พูดซ้ำ','Clinic receptionist','Hello. What brings you in today?','I’ve had [symptom] for [duration].','บอกอาการและระยะเวลา ถามให้ชัด ห้ามให้คำวินิจฉัยหรือการรักษา'),
('Calling for help','A2','บอกปัญหา ตำแหน่ง และขอความช่วยเหลือ','Help desk operator','Hello. Where are you, and what happened?','I’m at [location]. I need help with [problem].','บอกตำแหน่ง ปัญหา และทวนสิ่งที่ผู้ช่วยถาม ใช้เป็นฉากฝึกภาษาเท่านั้น'),
('Returning a purchase','B1','อธิบายข้อบกพร่องและขอทางแก้สุภาพ','Customer service agent','What seems to be the problem with your purchase?','I bought [item], but [problem]. Could I [request]?','เล่าข้อเท็จจริง อธิบายผลกระทบ และตกลงทางแก้'),
('A delayed flight','B1','ถามทางเลือกเมื่อแผนเดินทางเปลี่ยน','Airline agent','Your flight has been delayed. How can I help?','If [option] isn’t available, could we [alternative]?','ถามเวลาที่ทราบ เปรียบเทียบตัวเลือก และยืนยันแผนใหม่'),
('Getting to know a friend','A2','ถามความสนใจและคุยต่อไม่จบแค่ yes/no','New friend','What do you enjoy doing outside work?','I enjoy [activity] because [reason].','เล่าความสนใจ ถามต่อ และเชื่อมเรื่องของตัวเอง'),
('Choosing a place to live','B1','เปรียบเทียบที่พักและอธิบายความสำคัญ','Friend','What matters most to you when choosing an apartment?','[A] is [comparison], but [B] offers [benefit].','เปรียบเทียบราคา ทำเล และข้อจำกัด โดยใช้ตัวเลขสมมติ'),
('A polite disagreement','B2','คุยความเห็นต่างโดยไม่เสียความสัมพันธ์','Friend','I think we should book the expensive trip now. What do you think?','I see your point, although I’m concerned about [reason].','ยอมรับมุมอีกฝ่าย อธิบายข้อกังวล และเสนอทางกลาง'),
('Explaining Thai culture','B2','เล่าเรื่องคุ้นเคยให้คนต่างวัฒนธรรมเข้าใจ','International friend','Could you tell me about a Thai celebration you enjoy?','A useful way to think about [concept] is [comparison].','อธิบายบริบท ยกตัวอย่าง และรับคำถามโดยไม่เหมารวมทุกคน'),
('A difficult phone call','B1','อธิบายปัญหาผ่านโทรศัพท์และทวนข้อมูล','Service agent','Hello, customer support. How may I help you?','Let me confirm: [details]. Is that correct?','เล่าปัญหา ทวนตัวเลขสมมติ และตกลงขั้นตอนถัดไป'),
]
for i,(title,level,goal,role,opening,pattern,criteria) in enumerate(everyday):
 scenes.append(dict(id=f'everyday-{i+1:02}',title=title,category='Everyday',level=level,goal=goal,roles=['Learner',role],brief=f'คุณกำลังฝึก{goal} กับ {role} ใช้ชื่อ ตัวเลข และข้อมูลสมมติ ผู้สอนถามทีละข้อและปรับความเร็วตามคุณ',opening=opening,success_criteria=criteria.split(' และ'),minutes=7,starter_pattern=pattern))
# Make core lessons transferable beyond their original work examples.
for l in lessons:
 level=l['level'];slots=re.findall(r'\[([^]]+)\]',l['pattern'])
 l['slots']=[dict(name=x,explanation='แทนด้วยข้อมูลจริงหรือข้อมูลสมมติของคุณ โดยคงส่วนอื่นของประโยค') for x in slots]
 l['coaching_notes']={'Pre-A1':'เริ่มทีละวลี 2–5 คำ ฟัง → พูดตาม → ปิดข้อความแล้วพูดใหม่ ไม่ต้องเลียนสำเนียงเจ้าของภาษา เน้นสื่อความหมาย', 'A1':'เชื่อมสองประโยคด้วย and หรือ because แล้วถามกลับหนึ่งข้อ เน้นเวลา บุคคล และลำดับคำให้ชัด','A2':'ตอบเป็นสามส่วน: ใจความ → รายละเอียด → คำถามกลับ หากติดให้ใช้ Let me think หรือ Could you clarify?','B1':'ให้เหตุผลพร้อมตัวอย่าง แยกข้อมูลที่รู้กับสมมติฐาน และเช็กว่าผู้ฟังเข้าใจ','B2':'สรุปประเด็นก่อน เปรียบเทียบทางเลือก ระบุข้อจำกัด พร้อมตอบคำถามท้าทายและหาข้อตกลง'}[level]
 if l['coaching_notes'] not in l['explanation']: l['explanation']+=' '+l['coaching_notes']
 l['drills'][2]['prompt']='เปลี่ยนจากข้อมูลของตัวอย่างเป็นเรื่องของอีกคนหรืออีกสถานที่ ใช้เป้าหมายการสื่อสารเดิม ปรับสรรพนามและเวลาเมื่อจำเป็น'
 l['drills'][2]['target']='A semantically valid sentence in a changed context; do not force statements into unnatural questions.'
 l['drills'][3]['time_goal_seconds']={'Pre-A1':20,'A1':15,'A2':12,'B1':10,'B2':8}[level]
 l['conversation_prompt']=l['conversation_prompt'].split(' Start from the lesson communication goal')[0]+f' Start from the lesson communication goal, not an unrelated generic question. First transfer task: a new personal/everyday context. Second task: workplace context with different details. Ask one follow-up requiring the learner to expand or clarify; do not accept repeating the sample as transfer mastery. Level {level}: use simple Thai framing at beginner levels, give choices of ideas only if requested. After two independent successes ask a novel scenario question.'
 l['assessment']=l['unit']*5==((l['ordinal']-1)%20+1)
 l['version']='2026-09-05.2'
(out/'lessons.json').write_text(json.dumps(lessons,ensure_ascii=False,indent=2)+'\n')
(out/'scenarios.json').write_text(json.dumps(scenes,ensure_ascii=False,indent=2)+'\n')
print('100 core lessons + 50 work scenarios + 20 everyday scenarios')
