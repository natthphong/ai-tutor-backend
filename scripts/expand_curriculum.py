#!/usr/bin/env python3
"""Build the additive 425-lesson curriculum supplement.

This script never reads or rewrites lessons.json. Its output is deterministic and
can be regenerated with: python3 scripts/expand_curriculum.py
"""
from __future__ import annotations

import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "internal/content/expansion.json"
VERSION = "2026-09-06.1"


def rows(block: str) -> list[tuple[str, str, str, str, str]]:
    parsed = []
    for line in block.strip().splitlines():
        parts = [part.strip() for part in line.split("|")]
        if len(parts) != 5:
            raise ValueError(f"expected five fields: {line}")
        parsed.append(tuple(parts))
    if len(parsed) != 5:
        raise ValueError("each unit must contain exactly five lessons")
    return parsed


PRACTICAL_UNITS: list[tuple[str, int, str, str]] = [
    # A2: everyday independence and clear workplace participation.
    ("A2", 5, "จัดกิจวัตรและเวลา", """
เล่ากิจวัตรวันทำงาน|I usually [activity] before [time/event].|I usually check my calendar before breakfast.|ปกติฉันเช็กปฏิทินก่อนอาหารเช้า|usually=โดยปกติ;calendar=ปฏิทิน;before=ก่อน
บอกความถี่|I [activity] [frequency].|I work from home twice a week.|ฉันทำงานจากบ้านสัปดาห์ละสองครั้ง|twice=สองครั้ง;week=สัปดาห์;work from home=ทำงานจากบ้าน
จัดเวลานัด|Are you free [day/time] for [activity]?|Are you free Tuesday afternoon for coffee?|บ่ายวันอังคารว่างไปดื่มกาแฟไหม|free=ว่าง;afternoon=ช่วงบ่าย;coffee=กาแฟ
เลื่อนนัดอย่างสุภาพ|Could we move [event] to [new time]?|Could we move lunch to one o'clock?|เราเลื่อนมื้อกลางวันไปบ่ายโมงได้ไหม|move=เลื่อน;lunch=มื้อกลางวัน;o'clock=นาฬิกา
ยืนยันกำหนดการ|That works for me. I'll see you [time/place].|That works for me. I'll see you at the station at six.|เวลานั้นฉันสะดวก เจอกันที่สถานีหกโมง|works for me=ฉันสะดวก;station=สถานี;at six=ตอนหกโมง
"""),
    ("A2", 6, "เดินทางในเมือง", """
ถามทาง|How do I get to [place] from here?|How do I get to the city library from here?|จากตรงนี้ไปห้องสมุดกลางอย่างไร|city library=ห้องสมุดกลาง;from here=จากตรงนี้;get to=ไปถึง
บอกเส้นทางเป็นขั้น|Go straight, then turn [direction] at [landmark].|Go straight, then turn left at the pharmacy.|ตรงไปแล้วเลี้ยวซ้ายที่ร้านขายยา|straight=ตรงไป;turn left=เลี้ยวซ้าย;pharmacy=ร้านขายยา
ถามสายรถ|Which bus goes to [place]?|Which bus goes to the weekend market?|รถเมล์สายไหนไปตลาดนัดสุดสัปดาห์|which bus=รถสายไหน;weekend market=ตลาดนัดสุดสัปดาห์;goes to=ไปยัง
ซื้อตั๋ว|I'd like a [ticket type] ticket to [place], please.|I'd like a return ticket to Ayutthaya, please.|ขอตั๋วไปกลับอยุธยาหนึ่งใบ|return ticket=ตั๋วไปกลับ;single ticket=ตั๋วเที่ยวเดียว;platform=ชานชาลา
แก้เมื่อหลงทาง|I think I'm lost. Is [place] near here?|I think I'm lost. Is the river pier near here?|ฉันน่าจะหลง ท่าเรืออยู่ใกล้ตรงนี้ไหม|lost=หลงทาง;river pier=ท่าเรือ;near=ใกล้
"""),
    ("A2", 7, "ซื้อของและใช้บริการ", """
ถามราคาและขนาด|How much is this in [size/color]?|How much is this shirt in medium?|เสื้อตัวนี้ไซซ์กลางราคาเท่าไร|medium=ขนาดกลาง;price=ราคา;shirt=เสื้อเชิ้ต
ขอลองสินค้า|Could I try this on?|Could I try these shoes on?|ขอลองรองเท้าคู่นี้ได้ไหม|try on=ลองสวม;shoes=รองเท้า;fitting room=ห้องลองเสื้อ
เปรียบเทียบตัวเลือก|This one is [comparison], but that one is [comparison].|This one is cheaper, but that one is more durable.|อันนี้ถูกกว่า แต่อันนั้นทนกว่า|cheaper=ถูกกว่า;durable=ทนทาน;option=ตัวเลือก
แจ้งปัญหาสินค้า|I bought this [time], but [problem].|I bought this yesterday, but the charger doesn't work.|ฉันซื้อเมื่อวานแต่ที่ชาร์จใช้ไม่ได้|charger=ที่ชาร์จ;doesn't work=ใช้ไม่ได้;receipt=ใบเสร็จ
ขอคืนหรือเปลี่ยน|Could I exchange this for [alternative]?|Could I exchange this for a larger size?|ขอเปลี่ยนเป็นไซซ์ใหญ่กว่าได้ไหม|exchange=เปลี่ยนสินค้า;larger=ใหญ่กว่า;refund=คืนเงิน
"""),
    ("A2", 8, "สุขภาพประจำวัน", """
บอกอาการ|I've got [symptom] and it started [time].|I've got a sore throat and it started yesterday.|ฉันเจ็บคอ เริ่มเป็นเมื่อวาน|sore throat=เจ็บคอ;symptom=อาการ;started=เริ่ม
บอกความรุนแรง|It hurts when I [action], but I can still [action].|It hurts when I walk, but I can still stand.|เจ็บตอนเดินแต่ยังยืนได้|hurts=เจ็บ;walk=เดิน;stand=ยืน
ขอนัดพบแพทย์|I'd like to make an appointment for [day/reason].|I'd like to make an appointment for Friday morning.|อยากนัดพบแพทย์เช้าวันศุกร์|appointment=นัดหมาย;clinic=คลินิก;Friday morning=เช้าวันศุกร์
ถามวิธีใช้ยา|How often should I take [medicine]?|How often should I take these tablets?|ควรกินยาเม็ดนี้บ่อยแค่ไหน|tablet=ยาเม็ด;take medicine=กินยา;how often=บ่อยแค่ไหน
ลางานเพราะป่วย|I'm not well enough to [duty], so I need [request].|I'm not well enough to work, so I need a sick day.|ฉันไม่สบายจนทำงานไม่ไหวจึงขอลาป่วย|well enough=แข็งแรงพอ;sick day=วันลาป่วย;rest=พักผ่อน
"""),
    ("A2", 9, "วางแผนการเดินทาง", """
ถามข้อมูลที่พัก|Does the room include [service]?|Does the room include breakfast and Wi-Fi?|ห้องพักรวมอาหารเช้าและไวไฟไหม|include=รวม;breakfast=อาหารเช้า;Wi-Fi=ไวไฟ
จองที่พัก|I'd like to book [room] for [dates].|I'd like to book a double room for two nights.|ต้องการจองห้องคู่สองคืน|book=จอง;double room=ห้องคู่;two nights=สองคืน
เช็กอิน|I have a reservation under [name].|I have a reservation under Narin Chai.|ฉันจองไว้ในชื่อนรินทร์ ชัย|reservation=การจอง;under a name=ในชื่อ;passport=หนังสือเดินทาง
ถามคำแนะนำท้องถิ่น|Could you recommend a good place to [activity]?|Could you recommend a good place to eat nearby?|ช่วยแนะนำร้านอาหารดีๆ แถวนี้ได้ไหม|recommend=แนะนำ;nearby=ใกล้ๆ;local=ท้องถิ่น
แจ้งปัญหาห้องพัก|There seems to be a problem with [thing].|There seems to be a problem with the air conditioner.|เครื่องปรับอากาศน่าจะมีปัญหา|seems=ดูเหมือน;air conditioner=เครื่องปรับอากาศ;problem=ปัญหา
"""),
    ("A2", 10, "สังคมและความสัมพันธ์", """
ชวนทำกิจกรรม|Would you like to [activity] with us [time]?|Would you like to have dinner with us tonight?|คืนนี้อยากกินข้าวกับเราไหม|would like=อยาก;have dinner=กินมื้อเย็น;tonight=คืนนี้
ตอบรับพร้อมถามรายละเอียด|I'd love to. What time should we [action]?|I'd love to. What time should we meet?|ยินดีเลย เราควรเจอกันกี่โมง|I'd love to=ยินดีมาก;meet=พบกัน;details=รายละเอียด
ปฏิเสธอย่างเป็นมิตร|Thanks for asking, but I can't because [reason].|Thanks for asking, but I can't because I'm working late.|ขอบคุณที่ชวน แต่ไปไม่ได้เพราะทำงานดึก|thanks for asking=ขอบคุณที่ชวน;working late=ทำงานดึก;another time=ครั้งหน้า
แสดงความสนใจ|That sounds [reaction]. How did you [follow-up]?|That sounds exciting. How did you find that place?|ฟังดูน่าตื่นเต้น คุณเจอที่นั่นได้อย่างไร|exciting=น่าตื่นเต้น;find=ค้นพบ;follow-up=คำถามต่อยอด
ขอโทษเรื่องเล็ก|I'm sorry I [mistake]. I'll [repair].|I'm sorry I'm late. I'll message earlier next time.|ขอโทษที่มาสาย คราวหน้าจะส่งข้อความให้เร็วขึ้น|late=สาย;earlier=เร็วขึ้น;next time=ครั้งหน้า
"""),
    ("A2", 11, "วางแผนและแบ่งงาน", """
เสนอแผนสั้น|Let's [action] first, then [action].|Let's list the tasks first, then choose owners.|เราลิสต์งานก่อน แล้วค่อยเลือกผู้รับผิดชอบ|task=งาน;owner=ผู้รับผิดชอบ;first=ก่อน
รับงาน|I can take care of [task] by [time].|I can take care of the slides by Wednesday.|ฉันดูแลสไลด์ให้เสร็จวันพุธได้|take care of=รับผิดชอบ;slides=สไลด์;by Wednesday=ภายในวันพุธ
ขอแบ่งงาน|Could you help me with [part] while I [task]?|Could you help me with the numbers while I draft the report?|ช่วยดูตัวเลขระหว่างที่ฉันร่างรายงานได้ไหม|numbers=ตัวเลข;draft=ร่าง;report=รายงาน
แจ้งสิ่งที่ต้องพึ่งพา|I need [input] before I can [action].|I need the final price before I can update the page.|ต้องได้ราคาสุดท้ายก่อนจึงอัปเดตหน้าได้|final price=ราคาสุดท้าย;update=อัปเดต;input=ข้อมูลที่ต้องใช้
ทวนเจ้าของและเวลา|So, [person] will [task] by [deadline], right?|So, Mali will call the supplier by noon, right?|สรุปว่ามะลิจะโทรหาผู้ขายภายในเที่ยงใช่ไหม|supplier=ผู้ขาย;deadline=กำหนดส่ง;by noon=ภายในเที่ยง
"""),
    ("A2", 12, "อัปเดตงาน", """
บอกสิ่งที่เสร็จ|I've finished [task], and the result is [result].|I've finished the homepage draft, and the result is ready for review.|ร่างหน้าแรกเสร็จแล้ว พร้อมให้ตรวจ|finished=เสร็จแล้ว;draft=ฉบับร่าง;review=ตรวจทาน
บอกงานถัดไป|Next, I'm going to [action] so that [goal].|Next, I'm going to test the form so that we can release it.|ต่อไปจะทดสอบฟอร์มเพื่อให้ปล่อยงานได้|next=ต่อไป;test=ทดสอบ;release=ปล่อยงาน
แจ้งตัวติดขัด|I'm waiting for [input], so [task] is blocked.|I'm waiting for access, so the data check is blocked.|กำลังรอสิทธิ์เข้าถึง งานตรวจข้อมูลจึงติดขัด|access=สิทธิ์เข้าถึง;blocked=ติดขัด;data check=ตรวจข้อมูล
ขอคำตัดสินใจ|I need a decision on [choice] before [time].|I need a decision on the color before tomorrow.|ต้องได้ข้อสรุปเรื่องสีก่อนพรุ่งนี้|decision=การตัดสินใจ;choice=ตัวเลือก;before tomorrow=ก่อนพรุ่งนี้
คาดการณ์เวลาจบ|I expect to finish [task] by [time].|I expect to finish the test by four o'clock.|คาดว่าจะทดสอบเสร็จภายในสี่โมง|expect=คาดว่า;finish=เสร็จ;by four=ภายในสี่โมง
"""),
    ("A2", 13, "ร่วมประชุม", """
ขอเริ่มประเด็น|Could we start with [topic]?|Could we start with the customer feedback?|เราเริ่มจากความคิดเห็นลูกค้าได้ไหม|start with=เริ่มจาก;customer feedback=ความคิดเห็นลูกค้า;topic=หัวข้อ
ขอให้ขยายความ|Could you explain what you mean by [term]?|Could you explain what you mean by urgent?|ช่วยอธิบายว่าเร่งด่วนหมายถึงอะไร|explain=อธิบาย;mean=หมายถึง;urgent=เร่งด่วน
แสดงความเห็น|I think [idea] because [reason].|I think option A is clearer because it has fewer steps.|ฉันคิดว่าตัวเลือกเอชัดกว่าเพราะมีขั้นตอนน้อยกว่า|option=ตัวเลือก;clearer=ชัดกว่า;step=ขั้นตอน
เห็นด้วยบางส่วน|I agree about [point], but I'm not sure about [point].|I agree about the goal, but I'm not sure about the date.|เห็นด้วยเรื่องเป้าหมาย แต่ไม่แน่ใจเรื่องวันที่|agree=เห็นด้วย;goal=เป้าหมาย;not sure=ไม่แน่ใจ
สรุปสิ่งที่ได้ยิน|If I understand correctly, we will [action].|If I understand correctly, we will test with ten users.|ถ้าเข้าใจถูก เราจะทดสอบกับผู้ใช้สิบคน|correctly=อย่างถูกต้อง;test=ทดสอบ;user=ผู้ใช้
"""),
    ("A2", 14, "อีเมลและข้อความงาน", """
เปิดข้อความพร้อมจุดประสงค์|I'm writing to [purpose].|I'm writing to confirm tomorrow's delivery.|เขียนมาเพื่อยืนยันการส่งของพรุ่งนี้|confirm=ยืนยัน;delivery=การส่งของ;purpose=จุดประสงค์
ขอข้อมูล|Could you send me [information] by [time]?|Could you send me the invoice by Friday?|ช่วยส่งใบแจ้งหนี้ภายในวันศุกร์ได้ไหม|invoice=ใบแจ้งหนี้;send=ส่ง;by Friday=ภายในวันศุกร์
แนบไฟล์|I've attached [file] for [purpose].|I've attached the updated plan for your review.|แนบแผนฉบับปรับปรุงมาให้ตรวจ|attached=แนบ;updated=ปรับปรุงแล้ว;review=ตรวจทาน
ตามงานสุภาพ|I'm following up on [request] from [time].|I'm following up on my access request from Monday.|ขอติดตามคำขอสิทธิ์เข้าถึงที่ส่งวันจันทร์|follow up=ติดตาม;request=คำขอ;access=สิทธิ์เข้าถึง
ปิดพร้อมขั้นตอนต่อไป|Please let me know if [condition].|Please let me know if you need any changes.|แจ้งฉันได้หากต้องการแก้ไข|let me know=แจ้งให้ทราบ;change=การแก้ไข;need=ต้องการ
"""),
    ("A2", 15, "ช่วยเหลือลูกค้า", """
ถามปัญหาหลัก|Could you tell me what happened when you [action]?|Could you tell me what happened when you paid?|ช่วยเล่าว่าเกิดอะไรขึ้นตอนชำระเงิน|happened=เกิดขึ้น;paid=ชำระแล้ว;problem=ปัญหา
ยืนยันความเข้าใจ|So the [item] arrived, but [problem]. Is that right?|So the package arrived, but one item was missing. Is that right?|พัสดุมาถึงแต่ของขาดหนึ่งชิ้น ถูกไหม|package=พัสดุ;missing=หายไป;item=สินค้า
ขอข้อมูลตรวจสอบ|May I have [detail] so I can [action]?|May I have your order number so I can check it?|ขอเลขคำสั่งซื้อเพื่อตรวจสอบได้ไหม|order number=เลขคำสั่งซื้อ;check=ตรวจสอบ;detail=รายละเอียด
เสนอทางแก้|I can [solution] or [alternative]. Which would you prefer?|I can resend the item or issue a refund. Which would you prefer?|ส่งสินค้าใหม่หรือคืนเงิน คุณสะดวกแบบไหน|resend=ส่งใหม่;issue a refund=คืนเงิน;prefer=ต้องการมากกว่า
กำหนดความคาดหวัง|You should receive [result] within [time].|You should receive the refund within three days.|คุณควรได้รับเงินคืนภายในสามวัน|receive=ได้รับ;within=ภายใน;refund=เงินคืน
"""),
    ("A2", 16, "ช่วยแก้ปัญหาเทคโนโลยี", """
ระบุอุปกรณ์และอาการ|I'm using [device], and [problem].|I'm using an Android phone, and the app keeps closing.|ฉันใช้โทรศัพท์แอนดรอยด์และแอปปิดเองบ่อยๆ|device=อุปกรณ์;app=แอป;keeps closing=ปิดเองซ้ำๆ
ถามว่าเริ่มเมื่อไร|When did the problem start?|When did the login problem start?|ปัญหาเข้าสู่ระบบเริ่มเมื่อไร|login=เข้าสู่ระบบ;start=เริ่ม;problem=ปัญหา
ให้ขั้นตอนเดียว|First, please [action], then tell me what you see.|First, please restart the app, then tell me what you see.|ก่อนอื่นรีสตาร์ตแอป แล้วบอกว่าเห็นอะไร|restart=เริ่มใหม่;first=ก่อนอื่น;screen=หน้าจอ
รายงานผลการลอง|I tried [action], but [result].|I tried resetting my password, but the link expired.|ลองรีเซ็ตรหัสผ่านแล้ว แต่ลิงก์หมดอายุ|reset=รีเซ็ต;expired=หมดอายุ;password=รหัสผ่าน
ส่งต่อพร้อมข้อมูล|I'll send this to [team] with [evidence].|I'll send this to support with the error screenshot.|จะส่งให้ฝ่ายสนับสนุนพร้อมภาพข้อผิดพลาด|support=ฝ่ายสนับสนุน;screenshot=ภาพหน้าจอ;evidence=หลักฐาน
"""),
    ("A2", 17, "พูดเรื่องข้อมูล", """
บอกตัวเลขหลัก|The total is [number], which is [change] from [period].|The total is 240, which is up from last week.|ยอดรวม 240 เพิ่มจากสัปดาห์ก่อน|total=ยอดรวม;up from=เพิ่มจาก;last week=สัปดาห์ก่อน
เปรียบเทียบสองกลุ่ม|[Group A] has more [measure] than [Group B].|Mobile has more visits than desktop.|มือถือมีจำนวนเข้าชมมากกว่าเดสก์ท็อป|visits=จำนวนเข้าชม;desktop=คอมพิวเตอร์;more than=มากกว่า
บอกแนวโน้มง่าย|[Measure] increased between [time] and [time].|Sales increased between April and June.|ยอดขายเพิ่มระหว่างเมษายนถึงมิถุนายน|increased=เพิ่มขึ้น;sales=ยอดขาย;between=ระหว่าง
ชี้ข้อจำกัดข้อมูล|We don't have enough data for [period/group].|We don't have enough data for new customers.|เรามีข้อมูลลูกค้าใหม่ไม่เพียงพอ|enough data=ข้อมูลเพียงพอ;new customer=ลูกค้าใหม่;group=กลุ่ม
ถามที่มาของตัวเลข|Where does this [number/measure] come from?|Where does this conversion rate come from?|อัตราเปลี่ยนเป็นลูกค้านี้มาจากไหน|conversion rate=อัตราเปลี่ยนเป็นลูกค้า;come from=มาจาก;source=แหล่งข้อมูล
"""),
    ("A2", 18, "เก็บความต้องการ", """
ถามผู้ใช้หลัก|Who will use [feature], and what do they need to do?|Who will use this dashboard, and what do they need to do?|ใครจะใช้แดชบอร์ดและต้องทำอะไร|dashboard=แดชบอร์ด;feature=ฟีเจอร์;user=ผู้ใช้
ถามผลลัพธ์ที่ต้องการ|What should happen after the user [action]?|What should happen after the user submits the form?|หลังผู้ใช้ส่งฟอร์มควรเกิดอะไรขึ้น|submit=ส่งข้อมูล;form=แบบฟอร์ม;happen=เกิดขึ้น
ขอตัวอย่างจริง|Could you give me an example of [case]?|Could you give me an example of an urgent request?|ช่วยยกตัวอย่างคำขอเร่งด่วนได้ไหม|example=ตัวอย่าง;urgent request=คำขอเร่งด่วน;case=กรณี
ทวนขอบเขต|So this includes [item], but not [item], correct?|So this includes email alerts, but not text messages, correct?|ขอบเขตรวมอีเมลแจ้งเตือนแต่ไม่รวมข้อความใช่ไหม|includes=รวม;email alert=อีเมลแจ้งเตือน;correct=ถูกต้อง
ถามเกณฑ์สำเร็จ|How will we know that [feature] works?|How will we know that the search works well?|เราจะรู้ได้อย่างไรว่าการค้นหาทำงานดี|search=การค้นหา;works well=ทำงานดี;success=ความสำเร็จ
"""),
    ("A2", 19, "ทำงานเป็นทีม", """
เสนอความช่วยเหลือ|Do you need a hand with [task]?|Do you need a hand with the event setup?|ต้องการให้ช่วยจัดงานไหม|need a hand=ต้องการความช่วยเหลือ;setup=การจัดเตรียม;event=งานกิจกรรม
ขอความช่วยเหลือเฉพาะจุด|Could you check [item] for me?|Could you check these figures for me?|ช่วยตรวจตัวเลขเหล่านี้ให้ฉันได้ไหม|check=ตรวจ;figure=ตัวเลข;for me=ให้ฉัน
ขอบคุณพร้อมบอกผล|Thanks for [help]. It helped me [result].|Thanks for checking the file. It helped me find an error.|ขอบคุณที่ตรวจไฟล์ ทำให้ฉันเจอข้อผิดพลาด|error=ข้อผิดพลาด;find=พบ;checking=การตรวจ
ให้ข้อเสนอแนะง่าย|The [part] is clear. Maybe we could [improvement].|The introduction is clear. Maybe we could add an example.|บทนำชัดเจน เราอาจเพิ่มตัวอย่าง|introduction=บทนำ;clear=ชัดเจน;add=เพิ่ม
รับข้อเสนอแนะ|Thanks, I'll [action] and send it back by [time].|Thanks, I'll add the labels and send it back by noon.|ขอบคุณ ฉันจะเพิ่มป้ายชื่อและส่งกลับภายในเที่ยง|label=ป้ายกำกับ;send back=ส่งกลับ;by noon=ภายในเที่ยง
"""),
    ("A2", 20, "สัมภาษณ์งานเบื้องต้น", """
แนะนำประสบการณ์|I have [duration] of experience in [area].|I have two years of experience in customer support.|ฉันมีประสบการณ์ฝ่ายบริการลูกค้าสองปี|experience=ประสบการณ์;customer support=ฝ่ายบริการลูกค้า;two years=สองปี
อธิบายหน้าที่|In my current role, I [responsibility].|In my current role, I prepare weekly sales reports.|ในงานปัจจุบันฉันเตรียมรายงานยอดขายรายสัปดาห์|current role=งานปัจจุบัน;prepare=จัดทำ;weekly=รายสัปดาห์
เล่าทักษะพร้อมตัวอย่าง|I'm good at [skill]. For example, I [evidence].|I'm good at organizing work. For example, I plan team schedules.|ฉันจัดงานเก่ง เช่นวางตารางทีม|organizing=การจัดระเบียบ;schedule=ตาราง;skill=ทักษะ
บอกเหตุผลที่สมัคร|I'm interested in this role because [reason].|I'm interested in this role because I enjoy solving customer problems.|สนใจงานนี้เพราะชอบแก้ปัญหาให้ลูกค้า|interested in=สนใจ;solving=การแก้;role=ตำแหน่ง
ถามผู้สัมภาษณ์|Could you tell me more about [topic]?|Could you tell me more about the first three months?|ช่วยเล่าเพิ่มเกี่ยวกับสามเดือนแรกได้ไหม|tell me more=เล่าเพิ่ม;first three months=สามเดือนแรก;topic=หัวข้อ
"""),
    ("A2", 21, "ธนาคารในชีวิตประจำวัน", """
ถามค่าธรรมเนียม|Is there a fee for [service]?|Is there a fee for transferring money abroad?|โอนเงินไปต่างประเทศมีค่าธรรมเนียมไหม|fee=ค่าธรรมเนียม;transfer=โอนเงิน;abroad=ต่างประเทศ
แจ้งรายการไม่รู้จัก|I don't recognize this [amount] payment to [merchant].|I don't recognize this 900-baht payment to ABC Shop.|ฉันไม่รู้จักรายการ 900 บาทที่จ่ายให้ร้านเอบีซี|recognize=จำได้;payment=รายการชำระ;merchant=ร้านค้า
ถามสถานะโอนเงิน|I transferred [amount] on [day], but it hasn't arrived.|I transferred 2,000 baht on Monday, but it hasn't arrived.|โอนสองพันบาทวันจันทร์แต่เงินยังไม่ถึง|transferred=โอนแล้ว;arrived=ถึงแล้ว;amount=จำนวนเงิน
ขออายัดบัตร|I've lost my card. Could you block it?|I've lost my debit card. Could you block it now?|ฉันทำบัตรเดบิตหาย ช่วยอายัดตอนนี้ได้ไหม|lost=ทำหาย;debit card=บัตรเดบิต;block=อายัด
ยืนยันขั้นตอนถัดไป|What do I need to do to [goal]?|What do I need to do to get a replacement card?|ต้องทำอะไรเพื่อรับบัตรใหม่|replacement card=บัตรทดแทน;need to=จำเป็นต้อง;get=ได้รับ
"""),
    ("A2", 22, "ความปลอดภัยและกฎงาน", """
ถามกฎ|Are we allowed to [action] here?|Are we allowed to take photos here?|ที่นี่อนุญาตให้ถ่ายภาพไหม|allowed=ได้รับอนุญาต;take photos=ถ่ายภาพ;rule=กฎ
บอกข้อห้าม|You mustn't [action] because [reason].|You mustn't share your password because it protects your account.|ห้ามแชร์รหัสผ่านเพราะช่วยปกป้องบัญชี|mustn't=ห้าม;share=แบ่งปัน;protect=ปกป้อง
บอกสิ่งที่ต้องทำ|We have to [action] before [event].|We have to wear badges before entering the office.|ต้องติดบัตรก่อนเข้าสำนักงาน|have to=ต้อง;badge=บัตรพนักงาน;enter=เข้า
รายงานอันตราย|There's [hazard] near [place].|There's water on the floor near the stairs.|มีน้ำบนพื้นใกล้บันได|hazard=อันตราย;floor=พื้น;stairs=บันได
ขอคำแนะนำฉุกเฉิน|What should I do if [event]?|What should I do if the fire alarm rings?|ควรทำอย่างไรหากสัญญาณไฟไหม้ดัง|fire alarm=สัญญาณไฟไหม้;rings=ดัง;emergency=เหตุฉุกเฉิน
"""),
    ("A2", 23, "นำเสนอแบบสั้น", """
เปิดหัวข้อ|Today I'd like to talk about [topic].|Today I'd like to talk about our customer survey.|วันนี้อยากนำเสนอผลสำรวจลูกค้า|talk about=พูดเรื่อง;survey=แบบสำรวจ;customer=ลูกค้า
บอกโครงเรื่อง|First I'll show [part], then [part].|First I'll show the problem, then our solution.|ก่อนอื่นจะให้ดูปัญหา แล้วจึงเสนอทางแก้|first=ก่อน;solution=ทางแก้;then=จากนั้น
ชี้ข้อมูลบนภาพ|This chart shows [finding].|This chart shows that weekend sales are higher.|กราฟนี้แสดงว่ายอดขายวันหยุดสูงกว่า|chart=กราฟ;weekend sales=ยอดขายวันหยุด;higher=สูงกว่า
เน้นข้อความหลัก|The main point is [message].|The main point is that customers want faster delivery.|ประเด็นหลักคือลูกค้าต้องการส่งของเร็วขึ้น|main point=ประเด็นหลัก;faster=เร็วกว่า;delivery=การส่งของ
เชิญถาม|Do you have any questions about [topic]?|Do you have any questions about the timeline?|มีคำถามเกี่ยวกับกำหนดการไหม|question=คำถาม;timeline=กำหนดการ;about=เกี่ยวกับ
"""),
    ("A2", 24, "ต่อรองเรื่องง่าย", """
บอกสิ่งที่ต้องการ|We're looking for [quantity/service] by [date].|We're looking for fifty chairs by next Friday.|เราต้องการเก้าอี้ห้าสิบตัวภายในศุกร์หน้า|looking for=กำลังหา;quantity=จำนวน;by next Friday=ภายในศุกร์หน้า
ถามความยืดหยุ่น|Is there any flexibility on [price/date/term]?|Is there any flexibility on the delivery date?|วันส่งของพอปรับได้ไหม|flexibility=ความยืดหยุ่น;delivery date=วันส่งของ;term=เงื่อนไข
เสนอแลกเปลี่ยน|If you can [request], we can [offer].|If you can deliver earlier, we can confirm today.|ถ้าส่งเร็วขึ้นได้ เราจะยืนยันวันนี้|deliver earlier=ส่งเร็วขึ้น;confirm=ยืนยัน;offer=ข้อเสนอ
ขอเวลาพิจารณา|I need to check [detail] before I agree.|I need to check the budget before I agree.|ต้องตรวจงบประมาณก่อนตกลง|budget=งบประมาณ;agree=ตกลง;check=ตรวจ
สรุปข้อตกลง|We've agreed on [term], with [condition].|We've agreed on the price, with delivery on Friday.|เราตกลงราคาและส่งของวันศุกร์|agreed=ตกลงแล้ว;price=ราคา;condition=เงื่อนไข
    """),
    # B1: independent problem solving and collaborative work.
    ("B1", 5, "ดูแลความสัมพันธ์", """
อธิบายความรู้สึกโดยไม่กล่าวโทษ|I felt [feeling] when [event] because [reason].|I felt left out when the plan changed because nobody told me.|ฉันรู้สึกถูกกันออกเมื่อแผนเปลี่ยนเพราะไม่มีใครบอก|left out=ถูกกันออก;plan changed=แผนเปลี่ยน;feeling=ความรู้สึก
ตรวจสอบความเข้าใจ|Have I understood correctly that [interpretation]?|Have I understood correctly that you need more space?|ฉันเข้าใจถูกไหมว่าคุณต้องการพื้นที่ส่วนตัวมากขึ้น|understood=เข้าใจ;space=พื้นที่ส่วนตัว;correctly=อย่างถูกต้อง
ยอมรับส่วนของตน|I should have [action], and I'm sorry that I didn't.|I should have called earlier, and I'm sorry that I didn't.|ฉันควรโทรให้เร็วกว่านี้และขอโทษที่ไม่ได้ทำ|should have=ควรจะได้;called earlier=โทรเร็วกว่านี้;sorry=ขอโทษ
ตั้งขอบเขตสุภาพ|I'm happy to [offer], but I can't [limit].|I'm happy to listen, but I can't answer calls during work.|ฉันพร้อมรับฟังแต่รับสายระหว่างงานไม่ได้|listen=รับฟัง;during work=ระหว่างงาน;limit=ขอบเขต
เสนอวิธีซ่อมความสัมพันธ์|Could we [repair] so that [shared goal]?|Could we check in weekly so that small problems don't grow?|เราคุยกันรายสัปดาห์เพื่อไม่ให้ปัญหาเล็กโตขึ้นได้ไหม|check in=พูดคุยติดตาม;weekly=รายสัปดาห์;shared goal=เป้าหมายร่วม
"""),
    ("B1", 6, "แก้ปัญหาระหว่างเดินทาง", """
รายงานเที่ยวเดินทางผิดปกติ|My [journey] was due to [event], but [problem].|My flight was due to leave at nine, but it has been cancelled.|เที่ยวบินควรออกเก้าโมงแต่ถูกยกเลิก|due to leave=มีกำหนดออก;cancelled=ถูกยกเลิก;flight=เที่ยวบิน
ถามทางเลือกและเงื่อนไข|What alternatives are available, and would I need to [condition]?|What alternatives are available, and would I need to pay extra?|มีทางเลือกใดบ้างและต้องจ่ายเพิ่มไหม|alternative=ทางเลือก;available=ที่มี;pay extra=จ่ายเพิ่ม
ขอความช่วยเหลือเรื่องสัมภาระ|My [baggage] hasn't arrived; it has [identifying detail].|My suitcase hasn't arrived; it has a red strap.|กระเป๋าเดินทางยังไม่มาและมีสายรัดสีแดง|suitcase=กระเป๋าเดินทาง;strap=สายรัด;arrived=มาถึง
ต่อรองทางแก้ที่พัก|Since [service failure], could you [remedy]?|Since the room isn't ready, could you store our bags?|เพราะห้องยังไม่พร้อม ช่วยเก็บกระเป๋าให้ได้ไหม|store=เก็บ;isn't ready=ยังไม่พร้อม;remedy=ทางแก้
อธิบายเหตุฉุกเฉินต่อเจ้าหน้าที่|We need help because [event]; our immediate concern is [risk].|We need help because my friend is missing; our immediate concern is her safety.|ต้องการความช่วยเหลือเพราะเพื่อนหาย สิ่งกังวลเร่งด่วนคือความปลอดภัย|missing=หายตัว;immediate concern=เรื่องกังวลเร่งด่วน;safety=ความปลอดภัย
"""),
    ("B1", 7, "ตัดสินใจเรื่องสุขภาพ", """
อธิบายประวัติอาการ|I've been experiencing [symptom] for [duration], especially when [condition].|I've been experiencing headaches for two weeks, especially after screens.|ปวดศีรษะมาสองสัปดาห์ โดยเฉพาะหลังใช้หน้าจอ|experiencing=มีอาการ;headache=ปวดศีรษะ;especially=โดยเฉพาะ
บอกสิ่งที่ลองแล้ว|I've already tried [action], which [result].|I've already tried resting, which helped only briefly.|ลองพักแล้ว ช่วยได้เพียงชั่วครู่|already tried=ลองแล้ว;briefly=ชั่วครู่;resting=การพัก
ถามข้อดีข้อเสียการรักษา|What are the likely benefits and side effects of [treatment]?|What are the likely benefits and side effects of this medicine?|ยานี้มีประโยชน์และผลข้างเคียงที่น่าจะเกิดอะไรบ้าง|likely=ที่น่าจะ;side effect=ผลข้างเคียง;treatment=การรักษา
ทวนคำแนะนำ|Just to confirm, I should [action] unless [condition].|Just to confirm, I should exercise gently unless the pain gets worse.|ขอยืนยันว่าควรออกกำลังเบาๆ เว้นแต่อาการปวดแย่ลง|unless=เว้นแต่;gently=เบาๆ;gets worse=แย่ลง
ขอความช่วยเหลือด้านงาน|My doctor has advised [restriction], so could we [adjustment]?|My doctor has advised less screen time, so could we adjust my duties?|แพทย์แนะนำให้ลดเวลาหน้าจอ จึงขอปรับหน้าที่ได้ไหม|advised=แนะนำ;adjust duties=ปรับหน้าที่;restriction=ข้อจำกัด
"""),
    ("B1", 8, "ชุมชนและอาสาสมัคร", """
ชวนร่วมแก้ปัญหา|We're trying to [community goal], and we need people who can [contribution].|We're trying to clean the canal, and we need people who can organize supplies.|เรากำลังทำความสะอาดคลองและต้องการคนจัดอุปกรณ์|canal=คลอง;supplies=อุปกรณ์;organize=จัดการ
อธิบายผลกระทบท้องถิ่น|This issue affects [group] by [effect].|This issue affects older residents by making travel harder.|ปัญหานี้ทำให้ผู้สูงอายุเดินทางลำบากขึ้น|resident=ผู้อยู่อาศัย;affects=ส่งผล;travel=เดินทาง
เสนอแผนกิจกรรม|We could [action], provided that [condition].|We could hold a weekend market, provided that vendors sort their waste.|จัดตลาดสุดสัปดาห์ได้หากผู้ขายแยกขยะ|provided that=โดยมีเงื่อนไขว่า;vendor=ผู้ขาย;sort waste=แยกขยะ
กระจายบทบาท|Who could take responsibility for [task], while [person/team] handles [task]?|Who could take responsibility for registration, while I handle publicity?|ใครรับผิดชอบลงทะเบียนได้ ระหว่างที่ฉันทำประชาสัมพันธ์|responsibility=ความรับผิดชอบ;registration=ลงทะเบียน;publicity=ประชาสัมพันธ์
รายงานผลกิจกรรม|We managed to [result], although [difficulty].|We managed to collect 300 books, although we had little storage.|เรารวบรวมหนังสือได้ 300 เล่มแม้มีที่เก็บน้อย|managed to=ทำสำเร็จ;storage=ที่เก็บ;although=แม้ว่า
"""),
    ("B1", 9, "ข่าวและสื่อ", """
สรุปข่าวโดยอ้างแหล่ง|According to [source], [event], although [uncertainty].|According to the city website, the road will reopen, although no date is confirmed.|ตามเว็บไซต์เมือง ถนนจะเปิดอีกครั้งแต่ยังไม่ยืนยันวัน|according to=ตามที่;reopen=เปิดอีกครั้ง;confirmed=ยืนยันแล้ว
แยกข้อเท็จจริงจากความเห็น|The article states [fact], while the claim that [opinion] is an interpretation.|The article states that sales fell, while the claim that the business is failing is an interpretation.|บทความระบุยอดขายลด ส่วนคำกล่าวว่าธุรกิจล้มเหลวเป็นการตีความ|states=ระบุ;claim=คำกล่าวอ้าง;interpretation=การตีความ
ถามความน่าเชื่อถือ|Who produced this information, and what evidence do they provide?|Who produced this survey, and what evidence do they provide?|ใครจัดทำแบบสำรวจและให้หลักฐานอะไร|produced=จัดทำ;evidence=หลักฐาน;survey=แบบสำรวจ
เปรียบเทียบการรายงาน|Both reports mention [fact], but they differ on [point].|Both reports mention flooding, but they differ on its cause.|ทั้งสองรายงานกล่าวถึงน้ำท่วมแต่ต่างกันเรื่องสาเหตุ|mention=กล่าวถึง;differ=แตกต่าง;cause=สาเหตุ
แก้ข่าวผิดอย่างระวัง|I haven't found evidence for [claim]; the verified information is [fact].|I haven't found evidence for that rumor; the verified information is that the office remains open.|ยังไม่พบหลักฐานของข่าวลือ ข้อมูลยืนยันคือสำนักงานยังเปิด|rumor=ข่าวลือ;verified=ตรวจสอบแล้ว;remains open=ยังเปิด
"""),
    ("B1", 10, "ส่งมอบโครงการ", """
อัปเดตเทียบแผน|We've completed [work], while [work] is running [status] the plan.|We've completed the design, while development is running two days behind the plan.|ออกแบบเสร็จแล้ว ส่วนพัฒนาช้ากว่าแผนสองวัน|completed=เสร็จแล้ว;behind plan=ช้ากว่าแผน;development=การพัฒนา
อธิบายสาเหตุล่าช้า|The delay was caused by [cause], which meant [effect].|The delay was caused by missing data, which meant we couldn't test payments.|ความล่าช้าเกิดจากข้อมูลขาด ทำให้ทดสอบการชำระเงินไม่ได้|caused by=เกิดจาก;missing data=ข้อมูลขาด;meant=ส่งผลให้
เสนอแผนกู้เวลา|We can recover [time] by [action], without [risk].|We can recover one day by testing in parallel, without reducing coverage.|กู้เวลาได้หนึ่งวันด้วยการทดสอบคู่ขนานโดยไม่ลดความครอบคลุม|recover=กู้คืน;in parallel=พร้อมกัน;coverage=ความครอบคลุม
แจ้งความเสี่ยงพร้อมตัวกระตุ้น|There's a risk of [event] if [trigger], so we're [mitigation].|There's a risk of another delay if approval slips, so we're booking an early review.|เสี่ยงล่าช้าอีกหากอนุมัติช้า จึงนัดตรวจเร็วขึ้น|risk=ความเสี่ยง;approval=การอนุมัติ;mitigation=การลดความเสี่ยง
ขอการตัดสินใจโครงการ|To stay on schedule, we need a decision on [issue] by [deadline].|To stay on schedule, we need a decision on scope by noon.|เพื่อให้ทันแผน ต้องตัดสินใจขอบเขตภายในเที่ยง|stay on schedule=ทันแผน;scope=ขอบเขต;deadline=กำหนด
"""),
    ("B1", 11, "ประชุมเพื่อการตัดสินใจ", """
กำหนดผลลัพธ์ประชุม|The outcome we need today is [decision].|The outcome we need today is a choice between the two suppliers.|ผลที่ต้องได้วันนี้คือเลือกผู้ขายหนึ่งในสองราย|outcome=ผลลัพธ์;supplier=ผู้ขาย;choice=การเลือก
ดึงประเด็นกลับ|Before we move on, could we resolve [open issue]?|Before we move on, could we resolve who owns the final check?|ก่อนเปลี่ยนเรื่อง ขอปิดประเด็นว่าใครตรวจขั้นสุดท้าย|move on=ไปเรื่องต่อไป;resolve=หาข้อสรุป;open issue=ประเด็นค้าง
ท้าทายสมมติฐาน|What evidence supports the assumption that [claim]?|What evidence supports the assumption that users prefer email?|มีหลักฐานอะไรสนับสนุนสมมติฐานว่าผู้ใช้ชอบอีเมล|supports=สนับสนุน;assumption=สมมติฐาน;prefer=ชอบมากกว่า
เชิญมุมมองที่ขาด|We haven't heard from [group]. How would this affect your work?|We haven't heard from operations. How would this affect your work?|ยังไม่ได้ฟังฝ่ายปฏิบัติการ เรื่องนี้กระทบงานอย่างไร|heard from=ได้รับฟัง;operations=ฝ่ายปฏิบัติการ;affect=กระทบ
บันทึกมติและเงื่อนไข|We've decided to [action], subject to [condition].|We've decided to launch Monday, subject to a successful security check.|ตัดสินใจเปิดวันจันทร์โดยมีเงื่อนไขว่าตรวจความปลอดภัยผ่าน|subject to=ขึ้นอยู่กับ;launch=เปิดใช้งาน;security check=ตรวจความปลอดภัย
"""),
    ("B1", 12, "เขียนงานให้ชัด", """
สรุปผู้บริหารสั้น|The purpose of this note is to [purpose]; the key finding is [finding].|The purpose of this note is to review complaints; the key finding is that delays doubled.|บันทึกนี้ทบทวนข้อร้องเรียน โดยพบว่าความล่าช้าเพิ่มสองเท่า|purpose=จุดประสงค์;key finding=ข้อค้นพบหลัก;complaint=ข้อร้องเรียน
จัดลำดับเหตุผล|First, [fact]. As a result, [effect]. Therefore, [recommendation].|First, demand rose. As a result, queues grew. Therefore, we recommend more staff.|ความต้องการเพิ่มจึงคิวยาว ดังนั้นแนะนำเพิ่มพนักงาน|as a result=เป็นผลให้;therefore=ดังนั้น;demand=ความต้องการ
แยกการกระทำจากข้อมูล|Please [action] by [time]. For context, [background].|Please approve the budget by Friday. For context, the supplier holds the price until Monday.|กรุณาอนุมัติงบภายในศุกร์ ผู้ขายคงราคาถึงจันทร์|for context=ข้อมูลประกอบ;approve=อนุมัติ;holds the price=คงราคา
ปรับน้ำเสียงให้สุภาพ|Could you clarify [issue] so that we can [goal]?|Could you clarify which figures are final so that we can publish accurately?|ช่วยยืนยันว่าตัวเลขใดเป็นตัวสุดท้ายเพื่อเผยแพร่ได้ถูกต้อง|clarify=ชี้แจง;publish=เผยแพร่;accurately=อย่างถูกต้อง
แก้ประโยคกำกวม|To be specific, [actor] will [action] by [time].|To be specific, the finance team will verify the total by Thursday.|ระบุให้ชัดว่าทีมการเงินตรวจยอดภายในพฤหัส|to be specific=กล่าวให้ชัด;verify=ตรวจยืนยัน;finance team=ทีมการเงิน
"""),
    ("B1", 13, "กู้สถานการณ์บริการ", """
รับรู้ผลกระทบ|I understand that [failure] has caused [impact].|I understand that the late delivery has caused you to miss an event.|เข้าใจว่าการส่งช้าทำให้คุณพลาดงาน|caused=ทำให้เกิด;miss an event=พลาดงาน;impact=ผลกระทบ
ขอโทษพร้อมรับผิดชอบ|We should have [expected action]. I'm sorry we didn't.|We should have updated you earlier. I'm sorry we didn't.|เราควรแจ้งคุณเร็วกว่านี้ ขอโทษที่ไม่ได้ทำ|should have=ควรจะได้;updated=แจ้งความคืบหน้า;earlier=เร็วกว่านี้
ตรวจข้อเท็จจริงก่อนแก้|Before I propose a solution, may I confirm [detail]?|Before I propose a solution, may I confirm the delivery address?|ก่อนเสนอทางแก้ ขอสอบทานที่อยู่จัดส่ง|propose=เสนอ;solution=ทางแก้;confirm=ยืนยัน
เสนอการเยียวยาพร้อมเวลา|We can [remedy] by [time], or [alternative].|We can replace the order by tomorrow, or refund it today.|เปลี่ยนสินค้าให้พรุ่งนี้หรือคืนเงินวันนี้ได้|replace=เปลี่ยนสินค้า;refund=คืนเงิน;remedy=การเยียวยา
ปิดวงติดตาม|I'll personally check [item] and update you [time].|I'll personally check the courier status and update you this afternoon.|ฉันจะตรวจสถานะขนส่งและอัปเดตบ่ายนี้ด้วยตนเอง|personally=ด้วยตนเอง;courier=บริษัทขนส่ง;update=แจ้งความคืบหน้า
"""),
    ("B1", 14, "รับมือเหตุขัดข้อง", """
แจ้งเหตุและขอบเขต|We're seeing [issue] affecting [users/service].|We're seeing login failures affecting mobile users.|พบการเข้าสู่ระบบล้มเหลว กระทบผู้ใช้มือถือ|failure=ความล้มเหลว;affecting=ที่กระทบ;mobile user=ผู้ใช้มือถือ
แยกสิ่งที่รู้กับสมมติฐาน|We know [fact]; we're checking whether [hypothesis].|We know requests time out; we're checking whether the database is overloaded.|รู้ว่าคำขอหมดเวลา กำลังตรวจว่าฐานข้อมูลทำงานหนักเกินไปหรือไม่|time out=หมดเวลา;database=ฐานข้อมูล;overloaded=ทำงานหนักเกินไป
อธิบายมาตรการชั่วคราว|As a temporary measure, we've [action] to [goal].|As a temporary measure, we've disabled uploads to stabilize the service.|ปิดการอัปโหลดชั่วคราวเพื่อทำให้บริการเสถียร|temporary measure=มาตรการชั่วคราว;disabled=ปิดใช้;stabilize=ทำให้เสถียร
ให้เวลารายงานครั้งถัดไป|We'll provide another update by [time], even if [condition].|We'll provide another update by three, even if the cause is still unknown.|จะอัปเดตภายในบ่ายสามแม้ยังไม่ทราบสาเหตุ|provide an update=แจ้งความคืบหน้า;cause=สาเหตุ;unknown=ไม่ทราบ
ประกาศการฟื้นตัวอย่างระวัง|The service has recovered, and we're monitoring [signal] before closing the incident.|The service has recovered, and we're monitoring error rates before closing the incident.|บริการฟื้นแล้ว กำลังเฝ้าดูอัตราข้อผิดพลาดก่อนปิดเหตุ|recovered=ฟื้นตัว;monitoring=เฝ้าดู;error rate=อัตราข้อผิดพลาด
"""),
    ("B1", 15, "วิเคราะห์ข้อมูลและข้อสรุป", """
บรรยายความสัมพันธ์|As [measure] increased, [measure] tended to [change].|As response time increased, completion rates tended to fall.|เมื่อเวลาตอบสนองเพิ่ม อัตราทำสำเร็จมีแนวโน้มลด|tended to=มีแนวโน้ม;completion rate=อัตราทำสำเร็จ;fall=ลดลง
หลีกเลี่ยงสรุปเหตุผลเกินข้อมูล|The data shows [relationship], but it doesn't prove [cause].|The data shows a link with price, but it doesn't prove price caused the drop.|ข้อมูลแสดงความสัมพันธ์กับราคาแต่ยังพิสูจน์ไม่ได้ว่าเป็นสาเหตุ|link=ความสัมพันธ์;prove=พิสูจน์;caused=เป็นสาเหตุ
อธิบายค่าผิดปกติ|[Data point] is an outlier because [context].|Friday is an outlier because a promotion doubled traffic.|วันศุกร์เป็นค่าผิดปกติเพราะโปรโมชันทำให้ทราฟฟิกเพิ่มสองเท่า|outlier=ค่าผิดปกติ;promotion=โปรโมชัน;doubled=เพิ่มสองเท่า
แบ่งกลุ่มเพื่อเห็นภาพ|When we break the results down by [segment], [finding].|When we break the results down by device, mobile performs worse.|เมื่อแยกตามอุปกรณ์ พบว่ามือถือทำผลงานแย่กว่า|break down=แยกย่อย;segment=กลุ่มย่อย;performs worse=ทำผลงานแย่กว่า
เสนอการตรวจเพิ่ม|To test this explanation, we should compare [A] with [B].|To test this explanation, we should compare new users with returning users.|เพื่อทดสอบคำอธิบาย ควรเปรียบเทียบผู้ใช้ใหม่กับผู้ใช้เดิม|returning user=ผู้ใช้เดิม;compare=เปรียบเทียบ;explanation=คำอธิบาย
"""),
    ("B1", 16, "ปรับปรุงกระบวนการ", """
ระบุคอขวด|The main bottleneck occurs when [event], because [cause].|The main bottleneck occurs when orders need manual approval, because only one person can approve them.|คอขวดเกิดเมื่อต้องอนุมัติด้วยคนเพราะมีผู้อนุมัติคนเดียว|bottleneck=คอขวด;manual approval=อนุมัติด้วยคน;approve=อนุมัติ
วัดเวลาปัจจุบัน|It currently takes [duration] from [start] to [finish].|It currently takes three days from request to approval.|ปัจจุบันใช้สามวันตั้งแต่ขอจนอนุมัติ|currently=ปัจจุบัน;duration=ระยะเวลา;request=คำขอ
เสนอการทดลองเล็ก|We could pilot [change] with [group] and measure [metric].|We could pilot automatic reminders with one team and measure late approvals.|ทดลองแจ้งเตือนอัตโนมัติกับหนึ่งทีมและวัดการอนุมัติล่าช้าได้|pilot=ทดลองใช้;automatic reminder=การเตือนอัตโนมัติ;measure=วัด
พิจารณาผลข้างเคียง|This change may reduce [problem], but it could also [risk].|This change may reduce waiting, but it could also create duplicate alerts.|การเปลี่ยนนี้อาจลดการรอแต่สร้างการแจ้งเตือนซ้ำ|reduce=ลด;duplicate alert=การแจ้งเตือนซ้ำ;risk=ความเสี่ยง
กำหนดเกณฑ์ทดลอง|We'll keep the change if [metric] improves without [harm].|We'll keep the change if approval time improves without more errors.|จะใช้การเปลี่ยนต่อหากเวลาอนุมัติดีขึ้นโดยข้อผิดพลาดไม่เพิ่ม|keep the change=ใช้ต่อ;improves=ดีขึ้น;error=ข้อผิดพลาด
"""),
    ("B1", 17, "จัดการความต้องการ", """
ค้นหาเหตุผลเบื้องหลัง|What problem are we trying to solve by [requested feature]?|What problem are we trying to solve by adding an export button?|เราพยายามแก้ปัญหาอะไรด้วยปุ่มส่งออก|trying to solve=พยายามแก้;export=ส่งออก;feature=ฟีเจอร์
ระบุกลุ่มและความถี่|Which users need this, and how often would they use it?|Which users need bulk editing, and how often would they use it?|ผู้ใช้กลุ่มใดต้องแก้ไขหลายรายการและใช้บ่อยเพียงใด|bulk editing=แก้ไขหลายรายการ;how often=บ่อยเพียงใด;user=ผู้ใช้
เปิดกรณีขอบ|What should happen if [edge case]?|What should happen if the file contains duplicate accounts?|ควรเกิดอะไรหากไฟล์มีบัญชีซ้ำ|edge case=กรณีขอบ;contains=มีอยู่;duplicate=ซ้ำ
จัดลำดับตามคุณค่า|If we can only deliver one, which option creates more value and why?|If we can only deliver one, which report creates more value and why?|หากส่งมอบได้หนึ่งอย่าง รายงานใดสร้างคุณค่ามากกว่าและเพราะอะไร|deliver=ส่งมอบ;creates value=สร้างคุณค่า;option=ตัวเลือก
เขียนเกณฑ์ยอมรับ|We'll consider this complete when [observable result].|We'll consider this complete when users can download a valid CSV.|ถือว่าเสร็จเมื่อผู้ใช้ดาวน์โหลดซีเอสวีที่ใช้ได้|consider complete=ถือว่าเสร็จ;download=ดาวน์โหลด;valid=ใช้ได้
"""),
    ("B1", 18, "นำทีมในงานประจำ", """
มอบหมายพร้อมบริบท|Could you own [task] because [reason], and share [output] by [time]?|Could you own the demo because you know the flow, and share a draft by Thursday?|ช่วยรับผิดชอบเดโมเพราะรู้ขั้นตอน และส่งร่างภายในพฤหัสได้ไหม|own=รับผิดชอบ;demo=การสาธิต;draft=ฉบับร่าง
เช็กภาระงาน|What else are you committed to, and what would need to move?|What else are you committed to, and what would need to move if you take this?|ตอนนี้มีภาระอะไร และต้องเลื่อนอะไรหากรับงานนี้|committed to=มีภาระผูกพัน;need to move=ต้องเลื่อน;workload=ภาระงาน
โค้ชด้วยคำถาม|What have you tried, and where are you getting stuck?|What have you tried, and where are you getting stuck in the analysis?|ลองอะไรแล้ว และติดตรงไหนในการวิเคราะห์|getting stuck=ติดขัด;tried=ลองแล้ว;analysis=การวิเคราะห์
ให้คำชมเฉพาะเจาะจง|Your [action] made [positive result] because [reason].|Your clear handover made the release smoother because everyone knew their role.|การส่งต่องานที่ชัดทำให้ปล่อยงานราบรื่นเพราะทุกคนรู้บทบาท|handover=การส่งต่องาน;smoother=ราบรื่นขึ้น;role=บทบาท
จัดการงานที่พลาด|We missed [commitment]. What got in the way, and how will we recover?|We missed the review deadline. What got in the way, and how will we recover?|เราพลาดกำหนดตรวจ อะไรขัดขวางและจะกู้สถานการณ์อย่างไร|missed=พลาด;got in the way=เป็นอุปสรรค;recover=กู้สถานการณ์
"""),
    ("B1", 19, "สัมภาษณ์เชิงพฤติกรรม", """
เล่าสถานการณ์และงาน|The situation was [context], and my responsibility was [task].|The situation was a late launch, and my responsibility was coordinating the fix.|สถานการณ์คือเปิดตัวล่าช้า หน้าที่ฉันคือประสานงานแก้ไข|situation=สถานการณ์;responsibility=หน้าที่;coordinating=ประสานงาน
อธิบายการกระทำของตน|I decided to [action] because [reason].|I decided to narrow the scope because the deadline could not move.|ตัดสินใจลดขอบเขตเพราะเลื่อนกำหนดไม่ได้|decided to=ตัดสินใจ;scope=ขอบเขต;deadline=กำหนดส่ง
ระบุผลด้วยหลักฐาน|As a result, [outcome], measured by [evidence].|As a result, errors fell by 20 percent, measured by support tickets.|ผลคือข้อผิดพลาดลด 20 เปอร์เซ็นต์ วัดจากทิกเก็ตช่วยเหลือ|as a result=ผลคือ;measured by=วัดจาก;support ticket=ทิกเก็ตช่วยเหลือ
สะท้อนสิ่งที่เรียนรู้|Looking back, I would [change] because [lesson].|Looking back, I would involve operations earlier because they saw risks we missed.|เมื่อมองย้อน ฉันจะดึงฝ่ายปฏิบัติการมาเร็วขึ้นเพราะเขาเห็นความเสี่ยงที่เราพลาด|looking back=เมื่อมองย้อน;involve=ดึงเข้าร่วม;risk=ความเสี่ยง
ตอบเมื่อยังไม่มีประสบการณ์ตรง|I haven't handled [case] directly, but in a similar situation I [transferable action].|I haven't handled a merger directly, but I have aligned two teams with different processes.|ยังไม่เคยจัดการควบรวมโดยตรง แต่เคยประสานสองทีมที่มีกระบวนการต่างกัน|directly=โดยตรง;similar situation=สถานการณ์คล้ายกัน;aligned=ทำให้สอดคล้อง
"""),
    ("B1", 20, "อธิบายการเงิน", """
อธิบายงบเทียบจริง|We budgeted [amount], whereas actual spending was [amount] due to [cause].|We budgeted 80,000 baht, whereas actual spending was 92,000 due to repairs.|ตั้งงบแปดหมื่นแต่ใช้จริงเก้าหมื่นสองเพราะค่าซ่อม|budgeted=ตั้งงบ;actual spending=ค่าใช้จริง;repairs=ค่าซ่อม
แยกต้นทุนคงที่และผันแปร|[Cost] is fixed, while [cost] varies with [driver].|Rent is fixed, while delivery costs vary with order volume.|ค่าเช่าคงที่ ส่วนค่าจัดส่งเปลี่ยนตามจำนวนคำสั่งซื้อ|fixed cost=ต้นทุนคงที่;varies=เปลี่ยนแปลง;order volume=จำนวนคำสั่งซื้อ
อธิบายกระแสเงินสด|Although [business result], cash flow [state] because [timing].|Although sales grew, cash flow tightened because customers paid later.|แม้ยอดขายโต กระแสเงินสดตึงเพราะลูกค้าจ่ายช้าลง|cash flow=กระแสเงินสด;tightened=ตึงตัว;sales grew=ยอดขายโต
ถามสมมติฐานประมาณการ|What assumptions does this forecast make about [driver]?|What assumptions does this forecast make about exchange rates?|ประมาณการนี้ตั้งสมมติฐานเรื่องอัตราแลกเปลี่ยนอย่างไร|assumption=สมมติฐาน;forecast=ประมาณการ;exchange rate=อัตราแลกเปลี่ยน
เสนอการควบคุมค่าใช้จ่าย|We could reduce [cost] by [action], provided that [safeguard].|We could reduce travel costs by using video calls, provided that key visits continue.|ลดค่าเดินทางด้วยวิดีโอคอลได้ หากยังคงการเยี่ยมสำคัญ|reduce=ลด;travel cost=ค่าเดินทาง;provided that=โดยมีเงื่อนไขว่า
"""),
    ("B1", 21, "ธนาคารและการควบคุม", """
อธิบายสถานะรายการ|The payment was authorized at [time], but settlement is still pending.|The payment was authorized at ten, but settlement is still pending.|รายการได้รับอนุมัติสิบโมง แต่การชำระดุลยังรอ|authorized=ได้รับอนุมัติ;settlement=การชำระดุล;pending=ยังรอ
อธิบายการตรวจยอด|We reconcile [record A] against [record B] to identify [difference].|We reconcile ledger entries against bank statements to identify missing items.|เทียบรายการบัญชีกับใบแจ้งยอดเพื่อหารายการที่หาย|reconcile=กระทบยอด;ledger entry=รายการบัญชี;bank statement=ใบแจ้งยอด
รายงานรายการซ้ำ|The same [transaction] appears twice, so we need to verify whether [cause].|The same transfer appears twice, so we need to verify whether it was retried.|รายการโอนเดียวกันปรากฏสองครั้ง จึงต้องตรวจว่าเกิดจากลองใหม่หรือไม่|appears twice=ปรากฏสองครั้ง;verify=ตรวจยืนยัน;retried=ลองใหม่
อธิบายหลักฐานอนุมัติ|The audit trail shows who [action], when, and under which [authority].|The audit trail shows who approved the limit, when, and under which role.|ร่องรอยตรวจสอบแสดงว่าใครอนุมัติวงเงิน เมื่อไร และด้วยบทบาทใด|audit trail=ร่องรอยตรวจสอบ;approved=อนุมัติ;limit=วงเงิน
ยกระดับข้อยกเว้น|This exception exceeds [threshold], so it requires [approval].|This exception exceeds the daily limit, so it requires manager approval.|ข้อยกเว้นนี้เกินวงเงินรายวัน จึงต้องให้ผู้จัดการอนุมัติ|exception=ข้อยกเว้น;exceeds=เกิน;manager approval=การอนุมัติจากผู้จัดการ
"""),
    ("B1", 22, "บริหารความเสี่ยง", """
เขียนความเสี่ยงเหตุและผล|If [cause], there is a risk that [event], leading to [impact].|If the supplier is late, there is a risk that testing slips, leading to a delayed launch.|หากผู้ขายช้า การทดสอบอาจเลื่อนจนเปิดตัวล่าช้า|leading to=นำไปสู่;supplier=ผู้ขาย;delayed launch=เปิดตัวล่าช้า
ประเมินโอกาสและผลกระทบ|The likelihood is [rating] because [evidence], while the impact would be [effect].|The likelihood is low because we have two suppliers, while the impact would be severe.|โอกาสต่ำเพราะมีผู้ขายสองราย แต่ผลกระทบรุนแรง|likelihood=โอกาสเกิด;severe=รุนแรง;evidence=หลักฐาน
กำหนดมาตรการลด|We can reduce this risk by [control], owned by [person/team].|We can reduce this risk by testing backups, owned by operations.|ลดความเสี่ยงด้วยการทดสอบข้อมูลสำรอง โดยฝ่ายปฏิบัติการรับผิดชอบ|reduce risk=ลดความเสี่ยง;backup=ข้อมูลสำรอง;owned by=รับผิดชอบโดย
ตั้งสัญญาณเตือน|We'll trigger the contingency plan if [indicator] reaches [threshold].|We'll trigger the contingency plan if errors reach five percent.|จะใช้แผนสำรองหากข้อผิดพลาดถึงห้าเปอร์เซ็นต์|trigger=เริ่มใช้;contingency plan=แผนสำรอง;threshold=เกณฑ์
ยอมรับความเสี่ยงอย่างมีเหตุผล|We recommend accepting [risk] because [benefit] outweighs [exposure].|We recommend accepting the minor delay because safer testing outweighs the cost.|แนะนำยอมรับความล่าช้าเล็กน้อยเพราะการทดสอบที่ปลอดภัยคุ้มกว่าต้นทุน|accepting risk=ยอมรับความเสี่ยง;outweighs=มีน้ำหนักมากกว่า;exposure=ความเสียหายที่อาจเกิด
"""),
    ("B1", 23, "นำเสนอข้อเสนอ", """
เปิดด้วยปัญหาและคำขอ|We face [problem], and today I'm asking for [decision].|We face rising support costs, and today I'm asking for approval to automate triage.|เรามีต้นทุนช่วยเหลือเพิ่ม และวันนี้ขออนุมัติระบบคัดกรองอัตโนมัติ|rising=เพิ่มขึ้น;approval=การอนุมัติ;triage=การคัดกรอง
สร้างเรื่องจากหลักฐาน|The evidence shows [finding], which matters because [impact].|The evidence shows repeat calls are rising, which matters because customers wait longer.|หลักฐานแสดงว่าสายซ้ำเพิ่ม ทำให้ลูกค้ารอนานขึ้น|repeat call=สายโทรซ้ำ;matters=สำคัญ;wait longer=รอนานขึ้น
เปรียบเทียบตัวเลือก|Option A offers [benefit], whereas option B reduces [risk].|Option A offers speed, whereas option B reduces implementation risk.|ตัวเลือกเอเร็วกว่า ส่วนบีลดความเสี่ยงการนำไปใช้|whereas=ในขณะที่;implementation=การนำไปใช้;reduces risk=ลดความเสี่ยง
ตอบข้อกังวล|The concern about [risk] is valid; we plan to address it by [action].|The concern about job impact is valid; we plan to retrain affected staff.|ข้อกังวลผลกระทบงานมีเหตุผล เราวางแผนฝึกพนักงานที่ได้รับผลกระทบใหม่|valid=มีเหตุผล;address=จัดการ;retrain=ฝึกใหม่
ปิดด้วยขั้นตอนตัดสินใจ|If you approve [proposal], we'll [next step] by [date].|If you approve the pilot, we'll select the team by Friday.|หากอนุมัติการทดลอง เราจะเลือกทีมภายในศุกร์|approve=อนุมัติ;pilot=การทดลอง;select=เลือก
"""),
    ("B1", 24, "เจรจาขอบเขตและเวลา", """
เปิดด้วยผลประโยชน์ร่วม|We both want [shared goal], so let's clarify [constraint].|We both want a reliable launch, so let's clarify the deadline.|เราต่างต้องการเปิดระบบที่เชื่อถือได้ จึงมาชี้กำหนดเวลาให้ชัด|shared goal=เป้าหมายร่วม;reliable=เชื่อถือได้;clarify=ชี้ให้ชัด
ค้นหาความสำคัญเบื้องหลัง|Which matters more: [priority A] or [priority B], and why?|Which matters more: launching in June or including every report, and why?|อะไรสำคัญกว่า เปิดเดือนมิถุนายนหรือมีรายงานครบ และเพราะอะไร|matters more=สำคัญกว่า;including=รวม;priority=สิ่งสำคัญ
เสนอทางเลือกมีเงื่อนไข|We can meet [constraint] if we [trade-off].|We can meet the date if we move analytics to phase two.|ทันกำหนดได้หากย้ายงานวิเคราะห์ไปเฟสสอง|meet the date=ทันกำหนด;analytics=งานวิเคราะห์;phase two=เฟสสอง
ทดสอบข้อตกลงกลาง|Would [compromise] address your main concern?|Would a weekly report address your main concern about visibility?|รายงานรายสัปดาห์ช่วยคลายข้อกังวลหลักเรื่องการมองเห็นงานไหม|compromise=ข้อตกลงกลาง;address a concern=จัดการข้อกังวล;visibility=การมองเห็นสถานะ
บันทึกข้อตกลงและสิ่งค้าง|We've agreed to [decision]; [open issue] remains to be resolved by [time].|We've agreed to reduce scope; the support model remains to be resolved by Monday.|ตกลงลดขอบเขต ส่วนรูปแบบช่วยเหลือต้องสรุปภายในจันทร์|remains=ยังคง;resolved=หาข้อสรุป;support model=รูปแบบช่วยเหลือ
    """),
    # B2: nuanced judgment, leadership, and high-stakes professional contexts.
    ("B2", 5, "สื่อสารความสัมพันธ์อย่างละเอียดอ่อน", """
เปิดบทสนทนายาก|There's something I'd like us to discuss before it becomes [risk].|There's something I'd like us to discuss before it becomes a source of resentment.|มีเรื่องที่อยากคุยก่อนจะกลายเป็นความคับข้องใจ|resentment=ความคับข้องใจ;source of=ต้นเหตุ;discuss=หารือ
บรรยายรูปแบบโดยไม่เหมารวม|I've noticed that [pattern] on several occasions; how do you see it?|I've noticed that decisions change after our meetings on several occasions; how do you see it?|สังเกตหลายครั้งว่ามติเปลี่ยนหลังประชุม คุณมองอย่างไร|on several occasions=หลายครั้ง;decision=มติ;notice=สังเกต
สะท้อนมุมมองอีกฝ่าย|It sounds as though [interpretation]. Have I captured that fairly?|It sounds as though you felt pressured to agree. Have I captured that fairly?|ฟังดูเหมือนคุณรู้สึกถูกกดดันให้เห็นด้วย ฉันเข้าใจเป็นธรรมหรือไม่|pressured=ถูกกดดัน;captured fairly=ถ่ายทอดอย่างเป็นธรรม;agree=เห็นด้วย
ขอโทษโดยไม่แก้ตัว|I was wrong to [action]. The impact was [effect], regardless of my intention.|I was wrong to interrupt you. The impact was that your point was lost, regardless of my intention.|ฉันผิดที่ขัดจังหวะ ทำให้ประเด็นคุณหายไปไม่ว่าเจตนาฉันคืออะไร|interrupt=ขัดจังหวะ;regardless of=ไม่ว่า;intention=เจตนา
ตกลงพฤติกรรมใหม่|Going forward, could we both [behavior] whenever [trigger]?|Going forward, could we both flag concerns before decisions are announced?|ต่อไปเราทั้งคู่แจ้งข้อกังวลก่อนประกาศมติได้ไหม|going forward=ต่อไป;flag concerns=แจ้งข้อกังวล;announced=ประกาศ
"""),
    ("B2", 6, "เดินทางข้ามวัฒนธรรม", """
ตรวจบรรทัดฐานท้องถิ่น|What's considered appropriate when [situation] here?|What's considered appropriate when greeting a senior colleague here?|ที่นี่ถือว่าอะไรเหมาะสมเมื่อต้อนรับเพื่อนร่วมงานอาวุโส|considered appropriate=ถือว่าเหมาะสม;greeting=การทักทาย;senior colleague=เพื่อนร่วมงานอาวุโส
แก้ความเข้าใจผิดทางวัฒนธรรม|I may have misunderstood [custom]; I intended to [meaning].|I may have misunderstood the custom; I intended to show respect.|ฉันอาจเข้าใจธรรมเนียมผิด เจตนาคือแสดงความเคารพ|custom=ธรรมเนียม;intended=ตั้งใจ;respect=ความเคารพ
จัดการการเปลี่ยนแผนซับซ้อน|Given that [disruption], could you reroute us via [option] while preserving [need]?|Given that the border is closed, could you reroute us via Singapore while preserving our connection?|เมื่อชายแดนปิด ช่วยเปลี่ยนเส้นทางผ่านสิงคโปร์โดยยังต่อเที่ยวทันได้ไหม|reroute=เปลี่ยนเส้นทาง;preserve=รักษาไว้;connection=เที่ยวต่อ
เจรจาความรับผิดชอบผู้ให้บริการ|As [failure] was within your control, what compensation are you able to offer?|As the overbooking was within your control, what compensation are you able to offer?|เนื่องจากการจองเกินอยู่ในการควบคุมของคุณ เสนอชดเชยอะไรได้บ้าง|overbooking=การจองเกิน;compensation=ค่าชดเชย;within your control=อยู่ในการควบคุม
บรรยายประสบการณ์อย่างสมดุล|While [aspect] challenged my expectations, it helped me appreciate [insight].|While the indirect communication challenged my expectations, it helped me appreciate the value of context.|แม้การสื่อสารอ้อมท้าทายความคาดหวัง แต่ทำให้เห็นคุณค่าของบริบท|indirect=โดยอ้อม;appreciate=เห็นคุณค่า;context=บริบท
"""),
    ("B2", 7, "ตัดสินใจสุขภาพที่ซับซ้อน", """
สรุปข้อมูลหลายด้าน|My understanding is that [option] offers [benefit], but carries [risk].|My understanding is that surgery offers faster relief, but carries recovery risks.|เข้าใจว่าการผ่าตัดบรรเทาเร็วกว่าแต่มีความเสี่ยงช่วงฟื้นตัว|surgery=การผ่าตัด;relief=การบรรเทา;recovery=การฟื้นตัว
ถามคุณภาพหลักฐาน|How strong is the evidence that [treatment] improves [outcome] for people like me?|How strong is the evidence that this treatment improves mobility for people like me?|หลักฐานแน่นเพียงใดว่าการรักษานี้ช่วยการเคลื่อนไหวในคนแบบฉัน|mobility=การเคลื่อนไหว;evidence=หลักฐาน;outcome=ผลลัพธ์
ชั่งคุณภาพชีวิต|I'd prioritize [value], even if that means [trade-off].|I'd prioritize staying independent, even if that means a longer recovery.|ให้ความสำคัญกับการพึ่งตนเอง แม้ต้องฟื้นตัวนานขึ้น|prioritize=ให้ความสำคัญ;independent=พึ่งตนเอง;trade-off=สิ่งแลกเปลี่ยน
ขอความเห็นที่สอง|Before deciding, I'd like a second opinion on [question].|Before deciding, I'd like a second opinion on whether therapy could avoid surgery.|ก่อนตัดสินใจอยากได้ความเห็นที่สองว่ากายภาพบำบัดอาจเลี่ยงผ่าตัดได้ไหม|second opinion=ความเห็นที่สอง;therapy=การบำบัด;avoid=หลีกเลี่ยง
สื่อสารข้อจำกัดอย่างเป็นส่วนตัว|I can share [necessary fact], but I'd prefer to keep [detail] confidential.|I can share the work restrictions, but I'd prefer to keep the diagnosis confidential.|บอกข้อจำกัดการทำงานได้ แต่อยากเก็บคำวินิจฉัยเป็นความลับ|confidential=เป็นความลับ;diagnosis=คำวินิจฉัย;restriction=ข้อจำกัด
"""),
    ("B2", 8, "นโยบายชุมชน", """
กำหนดปัญหาเชิงระบบ|The issue is not merely [symptom]; it stems partly from [cause].|The issue is not merely litter; it stems partly from limited waste collection.|ปัญหาไม่ใช่แค่ขยะ แต่ส่วนหนึ่งมาจากการเก็บขยะที่จำกัด|merely=เพียง;stems from=มีต้นตอจาก;waste collection=การเก็บขยะ
นำเสนอมุมมองผู้มีส่วนได้เสีย|Residents value [benefit], whereas businesses are concerned about [cost].|Residents value quieter streets, whereas businesses are concerned about deliveries.|ชุมชนต้องการถนนเงียบ ส่วนธุรกิจกังวลการส่งของ|resident=ชาวบ้าน;whereas=ในขณะที่;delivery=การส่งของ
เสนอเกณฑ์นโยบาย|Any proposal should be judged against [criterion A], [criterion B], and [criterion C].|Any proposal should be judged against access, cost, and environmental impact.|ข้อเสนอควรประเมินจากการเข้าถึง ต้นทุน และผลกระทบสิ่งแวดล้อม|judged against=ประเมินเทียบกับ;access=การเข้าถึง;environmental=ด้านสิ่งแวดล้อม
ตอบข้อคัดค้าน|That concern is legitimate; however, [safeguard] could mitigate it.|That concern is legitimate; however, delivery windows could mitigate it.|ข้อกังวลมีเหตุผล แต่ช่วงเวลาส่งของช่วยลดได้|legitimate=มีเหตุผล;delivery window=ช่วงเวลาส่งของ;mitigate=บรรเทา
เสนอการทดลองนโยบาย|Rather than assume [outcome], we could trial [policy] and publish [measure].|Rather than assume traffic will improve, we could trial the closure and publish journey times.|แทนที่จะคาดว่าจราจรจะดีขึ้น เราทดลองปิดถนนและเผยแพร่เวลาเดินทางได้|trial=ทดลอง;closure=การปิด;journey time=เวลาเดินทาง
"""),
    ("B2", 9, "วิจารณ์สื่อและข้อโต้แย้ง", """
ระบุกรอบเรื่อง|The report frames [issue] primarily as [frame], leaving [aspect] underexplored.|The report frames unemployment primarily as an individual failure, leaving policy underexplored.|รายงานวางกรอบว่างงานเป็นความล้มเหลวส่วนบุคคล และสำรวจนโยบายน้อยไป|frames=วางกรอบ;underexplored=สำรวจไม่พอ;unemployment=การว่างงาน
ตรวจภาษาชี้นำ|The phrase [wording] implies [assumption] without establishing it.|The phrase "tax burden" implies harm without establishing it.|คำว่าภาระภาษีสื่อนัยว่าเกิดโทษโดยยังไม่ได้พิสูจน์|implies=สื่อนัย;establishing=พิสูจน์ให้ชัด;burden=ภาระ
ประเมินหลักฐานคัดเลือก|The evidence may be selective because [missing comparison].|The evidence may be selective because it excludes years when prices fell.|หลักฐานอาจเลือกมาเพราะตัดปีที่ราคาลดออก|selective=เลือกบางส่วน;excludes=ไม่รวม;comparison=ข้อมูลเปรียบเทียบ
สร้างข้อโต้แย้งที่แข็งแรงที่สุด|The strongest version of the opposing view is [argument].|The strongest version of the opposing view is that regulation protects smaller firms.|รูปแบบที่แข็งแรงที่สุดของมุมตรงข้ามคือกฎช่วยปกป้องธุรกิจเล็ก|opposing view=มุมตรงข้าม;regulation=กฎกำกับ;protects=ปกป้อง
สรุปแบบมีเงื่อนไข|On balance, [conclusion], provided we accept [limitation].|On balance, the policy appears effective, provided we accept the short follow-up period.|โดยรวมดูว่านโยบายได้ผล หากยอมรับข้อจำกัดว่าติดตามผลสั้น|on balance=เมื่อชั่งโดยรวม;provided=หาก;follow-up period=ช่วงติดตามผล
"""),
    ("B2", 10, "วางกลยุทธ์และลำดับความสำคัญ", """
เชื่อมเป้าหมายกับทางเลือก|If our strategic aim is [goal], then [option] deserves priority because [reason].|If our strategic aim is retention, then onboarding deserves priority because early drop-off is highest.|หากเป้ากลยุทธ์คือรักษาลูกค้า ควรให้ onboarding ก่อนเพราะลูกค้าหลุดช่วงต้นสูงสุด|strategic aim=เป้ากลยุทธ์;retention=การรักษาลูกค้า;drop-off=การหลุดออก
เปิดสมมติฐานกลยุทธ์|This direction assumes that [assumption]; how could we test that cheaply?|This direction assumes that customers value speed; how could we test that cheaply?|ทิศทางนี้สมมติว่าลูกค้าให้ค่าความเร็ว จะทดสอบราคาถูกได้อย่างไร|direction=ทิศทาง;assumes=ตั้งสมมติฐาน;cheaply=ด้วยต้นทุนต่ำ
จัดพอร์ตงาน|We should stop [work], sustain [work], and increase investment in [work].|We should stop the unused newsletter, sustain support, and increase investment in search.|ควรหยุดจดหมายข่าวที่ไม่มีคนใช้ คงฝ่ายช่วยเหลือ และลงทุนค้นหาเพิ่ม|sustain=คงไว้;investment=การลงทุน;unused=ไม่มีคนใช้
วางทางเลือกภายใต้ความไม่แน่นอน|If [scenario], we will [response]; if not, we retain the option to [alternative].|If demand doubles, we will add capacity; if not, we retain the option to delay hiring.|หากความต้องการเพิ่มสองเท่า จะเพิ่มกำลังรองรับ มิฉะนั้นยังเลือกชะลอจ้างได้|scenario=สถานการณ์;capacity=กำลังรองรับ;retain the option=ยังคงทางเลือก
กำหนดสัญญาณทบทวน|We should revisit this decision when [signal], rather than on a fixed date.|We should revisit this decision when churn exceeds five percent, rather than on a fixed date.|ควรทบทวนเมื่ออัตราเลิกใช้เกินห้าเปอร์เซ็นต์แทนวันที่ตายตัว|revisit=ทบทวน;churn=อัตราเลิกใช้;fixed date=วันที่ตายตัว
"""),
    ("B2", 11, "ธรรมาภิบาลและการตัดสินใจ", """
ชี้สิทธิ์ตัดสินใจ|[Role] is accountable for [decision], while [role] must be consulted.|The product owner is accountable for scope, while compliance must be consulted.|เจ้าของผลิตภัณฑ์รับผิดชอบขอบเขต โดยต้องปรึกษาฝ่ายกำกับ|accountable=รับผิดชอบผล;consulted=ได้รับการปรึกษา;compliance=ฝ่ายกำกับ
เปิดเผยผลประโยชน์ทับซ้อน|I should disclose that [relationship], although I have not [influence].|I should disclose that I advised the supplier, although I have not scored the bids.|ควรเปิดเผยว่าเคยแนะนำผู้ขาย แม้ไม่ได้ให้คะแนนข้อเสนอ|disclose=เปิดเผย;supplier=ผู้ขาย;bid=ข้อเสนอประมูล
บันทึกเหตุผลการตัดสินใจ|We chose [option] based on [criteria], despite [disadvantage].|We chose the local provider based on resilience, despite the higher price.|เลือกผู้ให้บริการในประเทศเพราะความทนทาน แม้ราคาสูงกว่า|based on=อิงจาก;resilience=ความทนทาน;despite=แม้ว่า
ยกระดับข้อยกเว้น|Because this falls outside [authority], it should be escalated to [body].|Because this falls outside our spending authority, it should be escalated to the committee.|เรื่องนี้เกินอำนาจใช้จ่ายจึงควรยกระดับให้คณะกรรมการ|falls outside=อยู่นอก;spending authority=อำนาจใช้จ่าย;committee=คณะกรรมการ
ทบทวนการตัดสินใจ|The decision remains reasonable given [known facts], but [new evidence] warrants review.|The decision remains reasonable given known costs, but the security finding warrants review.|มติยังสมเหตุผลจากต้นทุนที่รู้ แต่ข้อค้นพบความปลอดภัยทำให้ควรทบทวน|remains reasonable=ยังสมเหตุผล;warrants=ทำให้สมควร;finding=ข้อค้นพบ
"""),
    ("B2", 12, "งานเขียนระดับผู้บริหาร", """
นำด้วยข้อเสนอแนะ|We recommend [action] to achieve [outcome], at an estimated [cost].|We recommend a phased migration to reduce outage risk, at an estimated six-week cost.|แนะนำย้ายระบบเป็นระยะเพื่อลดความเสี่ยงหยุดชะงัก ใช้เวลาราวหกสัปดาห์|phased migration=การย้ายเป็นระยะ;outage=ระบบหยุด;estimated=โดยประมาณ
สรุปบริบทจำเป็น|This recommendation follows [change], which has [consequence].|This recommendation follows a regulatory change, which has shortened reporting deadlines.|ข้อเสนอนี้ตามการเปลี่ยนกฎซึ่งทำให้กำหนดรายงานสั้นลง|follows=เกิดตาม;regulatory=ด้านกฎกำกับ;shortened=ทำให้สั้นลง
แสดงทางเลือกที่ไม่เลือก|We considered [alternative] but rejected it because [criterion].|We considered outsourcing but rejected it because customer data would cross jurisdictions.|พิจารณาจ้างภายนอกแต่ไม่เลือกเพราะข้อมูลลูกค้าจะข้ามเขตอำนาจ|outsourcing=จ้างภายนอก;rejected=ไม่เลือก;jurisdiction=เขตอำนาจ
ระบุความไม่แน่นอน|The estimate is sensitive to [variable]; a [change] would [effect].|The estimate is sensitive to licensing costs; a ten-percent rise would erase the savings.|ประมาณการไวต่อต้นทุนใบอนุญาต การเพิ่มสิบเปอร์เซ็นต์จะลบเงินประหยัด|sensitive to=ไวต่อ;licensing cost=ค่าใบอนุญาต;erase=ลบล้าง
ขอการตัดสินใจชัด|Approval is requested by [date] so that [consequence].|Approval is requested by 15 June so that procurement can secure the quoted rate.|ขออนุมัติภายใน 15 มิถุนายนเพื่อให้จัดซื้อรักษาราคาที่เสนอได้|procurement=ฝ่ายจัดซื้อ;secure=รักษาไว้;quoted rate=ราคาที่เสนอ
"""),
    ("B2", 13, "จัดการข้อร้องเรียนรุนแรง", """
รับรู้ความเสียหายเฉพาะ|We recognize that [failure] resulted in [material impact], and we take that seriously.|We recognize that the duplicate charge resulted in missed payments, and we take that seriously.|เราตระหนักว่าการเรียกเก็บซ้ำทำให้พลาดชำระรายการอื่นและถือเป็นเรื่องจริงจัง|duplicate charge=เรียกเก็บซ้ำ;resulted in=ส่งผลให้;take seriously=ถือเป็นเรื่องจริงจัง
อธิบายข้อเท็จจริงโดยไม่แก้ตัว|Our review found [cause]; this explains the failure but does not excuse it.|Our review found a routing error; this explains the failure but does not excuse it.|ตรวจพบข้อผิดพลาดการส่งต่อ ซึ่งอธิบายเหตุแต่ไม่ใช่ข้อแก้ตัว|routing error=ข้อผิดพลาดการส่งต่อ;excuse=ข้อแก้ตัว;review=การตรวจสอบ
เสนอเยียวยาตามผลกระทบ|In light of [impact], we propose [remedy] in addition to [correction].|In light of the fees incurred, we propose reimbursement in addition to correcting the balance.|จากค่าธรรมเนียมที่เกิด เสนอชดใช้เพิ่มจากการแก้ยอด|in light of=เมื่อพิจารณา;reimbursement=การชดใช้;incurred=ที่เกิดขึ้น
กำหนดสิ่งที่จะตรวจต่อ|We have confirmed [fact]; [question] remains under investigation.|We have confirmed no data was lost; the delay remains under investigation.|ยืนยันว่าไม่มีข้อมูลสูญหาย ส่วนความล่าช้ายังอยู่ระหว่างตรวจ|confirmed=ยืนยันแล้ว;under investigation=อยู่ระหว่างตรวจ;data loss=ข้อมูลสูญหาย
ปิดพร้อมกลไกยกระดับ|If this resolution does not address [concern], I can refer it to [independent channel].|If this resolution does not address your concern, I can refer it to the independent review team.|หากทางแก้ยังไม่ตอบข้อกังวล จะส่งให้ทีมตรวจอิสระได้|resolution=ทางยุติ;refer=ส่งต่อ;independent=อิสระ
"""),
    ("B2", 14, "วิเคราะห์สาเหตุราก", """
แยกตัวกระตุ้นกับสาเหตุ|[Event] triggered the incident, but the underlying cause was [cause].|A traffic spike triggered the incident, but the underlying cause was unbounded retries.|ทราฟฟิกพุ่งเป็นตัวกระตุ้น แต่สาเหตุรากคือการลองซ้ำไร้ขีดจำกัด|triggered=กระตุ้น;underlying cause=สาเหตุราก;unbounded=ไร้ขีดจำกัด
อธิบายปัจจัยร่วม|The failure required both [condition A] and [condition B]; neither alone was sufficient.|The failure required stale cache data and a missing validation; neither alone was sufficient.|ความล้มเหลวต้องมีทั้งแคชเก่าและการตรวจที่หาย อย่างใดอย่างหนึ่งไม่พอ|stale=เก่าไม่ทันสมัย;validation=การตรวจสอบ;sufficient=เพียงพอ
ทดสอบสมมติฐานย้อนหลัง|If [hypothesis] were correct, we would expect [evidence], yet [observation].|If capacity were the cause, we would expect CPU saturation, yet usage stayed moderate.|หากกำลังรองรับเป็นสาเหตุ ซีพียูควรเต็ม แต่การใช้ยังปานกลาง|saturation=การใช้เต็ม;moderate=ปานกลาง;hypothesis=สมมติฐาน
แยกการแก้เฉพาะหน้ากับป้องกัน|[Action] restored service; preventing recurrence requires [systemic change].|Restarting restored service; preventing recurrence requires limiting retries.|รีสตาร์ตทำให้บริการกลับมา แต่ป้องกันซ้ำต้องจำกัดการลองใหม่|restored=ทำให้กลับมา;recurrence=การเกิดซ้ำ;systemic=เชิงระบบ
บันทึกบทเรียนไร้การกล่าวโทษ|The process allowed [failure mode]; we will change [control] rather than rely on [individual behavior].|The process allowed unreviewed changes; we will require peer approval rather than rely on memory.|กระบวนการเปิดให้แก้โดยไม่ตรวจ จึงจะบังคับอนุมัติโดยเพื่อนแทนพึ่งความจำ|unreviewed=ไม่ได้ตรวจ;peer approval=เพื่อนร่วมงานอนุมัติ;rely on=พึ่งพา
"""),
    ("B2", 15, "เล่าเรื่องด้วยข้อมูล", """
เลือกฐานเปรียบเทียบ|Compared with [baseline], [measure] changed by [amount], which is [interpretation].|Compared with the seasonal average, demand rose 12 percent, which is unusual.|เทียบค่าเฉลี่ยตามฤดูกาล ความต้องการเพิ่ม 12 เปอร์เซ็นต์ ซึ่งผิดปกติ|baseline=ฐานเปรียบเทียบ;seasonal average=ค่าเฉลี่ยตามฤดูกาล;unusual=ผิดปกติ
อธิบายองค์ประกอบการเปลี่ยน|Most of the increase came from [segment], while [segment] remained broadly stable.|Most of the increase came from renewals, while new sales remained broadly stable.|การเพิ่มส่วนใหญ่มาจากต่ออายุ ส่วนยอดขายใหม่ค่อนข้างคงที่|renewal=การต่ออายุ;broadly stable=ค่อนข้างคงที่;segment=ส่วนย่อย
แสดงช่วงความไม่แน่นอน|We estimate [value] within a range of [range], depending on [variable].|We estimate savings of 8-12 percent, depending on adoption.|ประเมินว่าประหยัด 8-12 เปอร์เซ็นต์ ขึ้นกับการนำไปใช้|estimate=ประเมิน;range=ช่วง;adoption=การนำไปใช้
ป้องกันการเปรียบเทียบผิด|The groups are not directly comparable because [difference].|The groups are not directly comparable because one received the offer earlier.|กลุ่มเปรียบเทียบตรงๆ ไม่ได้เพราะกลุ่มหนึ่งได้รับข้อเสนอก่อน|directly comparable=เปรียบเทียบตรงๆ ได้;received=ได้รับ;earlier=ก่อน
แปลงข้อค้นพบเป็นการตัดสินใจ|Given [finding], the decision is whether to [choice], not whether [settled point].|Given stable demand, the decision is whether to expand now, not whether the service has value.|จากความต้องการคงที่ สิ่งต้องตัดสินใจคือขยายตอนนี้ไหม ไม่ใช่บริการมีคุณค่าหรือไม่|given=เมื่อพิจารณา;expand=ขยาย;settled point=ประเด็นที่ชัดแล้ว
"""),
    ("B2", 16, "ออกแบบกระบวนการ", """
กำหนดหลักออกแบบ|The process should optimize for [goal] while safeguarding [constraint].|The process should optimize for speed while safeguarding independent approval.|กระบวนการควรเน้นความเร็วพร้อมรักษาการอนุมัติอิสระ|optimize for=ปรับให้เหมาะกับ;safeguarding=คุ้มครอง;independent approval=อนุมัติอิสระ
แยกเส้นทางตามความเสี่ยง|Cases that meet [criteria] can follow [fast path]; exceptions require [review].|Cases under the threshold can follow automatic approval; exceptions require manual review.|กรณีต่ำกว่าเกณฑ์อนุมัติอัตโนมัติได้ ข้อยกเว้นต้องตรวจด้วยคน|threshold=เกณฑ์;automatic approval=อนุมัติอัตโนมัติ;manual review=ตรวจด้วยคน
วางจุดควบคุม|At [stage], the owner must verify [evidence] before [next action].|At submission, the owner must verify consent before data processing.|ตอนส่งข้อมูล ผู้รับผิดชอบต้องตรวจความยินยอมก่อนประมวลผล|submission=การส่งข้อมูล;verify=ตรวจยืนยัน;consent=ความยินยอม
ออกแบบการกู้คืน|If [step] fails, the process should [recovery] without [harm].|If notification fails, the process should retry without duplicating the payment.|หากแจ้งเตือนล้มเหลว กระบวนการควรลองใหม่โดยไม่ทำรายการชำระซ้ำ|retry=ลองใหม่;duplicating=ทำซ้ำ;recovery=การกู้คืน
ประเมินหลังนำใช้|After [period], we'll review [metrics] and retire any step that [criterion].|After one month, we'll review cycle time and retire any step that adds no control value.|หลังหนึ่งเดือนจะทบทวนรอบเวลาและเลิกขั้นตอนที่ไม่เพิ่มคุณค่าควบคุม|cycle time=รอบเวลา;retire=เลิกใช้;control value=คุณค่าด้านควบคุม
"""),
    ("B2", 17, "อธิบายสถาปัตยกรรม", """
เปิดด้วยแรงขับการออกแบบ|The design is driven by [requirement], particularly [constraint].|The design is driven by resilience, particularly recovery across regions.|แบบระบบขับเคลื่อนด้วยความทนทาน โดยเฉพาะการกู้ข้ามภูมิภาค|driven by=ขับเคลื่อนด้วย;resilience=ความทนทาน;region=ภูมิภาค
อธิบายขอบเขตส่วนประกอบ|[Component] is responsible for [role], but it does not [excluded role].|The gateway authenticates requests, but it does not decide account access.|เกตเวย์ยืนยันตัวตนคำขอ แต่ไม่ตัดสินสิทธิ์บัญชี|gateway=เกตเวย์;authenticates=ยืนยันตัวตน;account access=สิทธิ์บัญชี
ป้องกัน trade-off|We accepted [cost] in exchange for [benefit], because [priority].|We accepted eventual consistency in exchange for availability, because payments must remain visible.|ยอมรับข้อมูลตรงกันภายหลังเพื่อแลกความพร้อมใช้ เพราะรายการชำระต้องยังเห็นได้|eventual consistency=ข้อมูลตรงกันภายหลัง;availability=ความพร้อมใช้;in exchange for=เพื่อแลกกับ
อธิบายโหมดล้มเหลว|If [dependency] becomes unavailable, [fallback] keeps [capability] operating.|If the risk service becomes unavailable, cached rules keep low-value payments operating.|หากบริการความเสี่ยงใช้ไม่ได้ กฎในแคชช่วยให้รายการมูลค่าต่ำยังทำงาน|dependency=ระบบที่พึ่งพา;fallback=ทางสำรอง;cached rule=กฎในแคช
ตอบคำถามขยายระบบ|The current design supports [scale]; beyond that, [bottleneck] would need [change].|The current design supports twice today's load; beyond that, storage would need partitioning.|แบบปัจจุบันรองรับสองเท่าของวันนี้ เกินนั้นต้องแบ่งพื้นที่จัดเก็บ|supports=รองรับ;load=ภาระงาน;partitioning=การแบ่งส่วน
"""),
    ("B2", 18, "บริหารผู้มีส่วนได้เสีย", """
ทำแผนที่อิทธิพลและผลกระทบ|[Stakeholder] has high influence but limited direct impact, so we should [engagement].|The regulator has high influence but limited direct impact, so we should consult early.|หน่วยกำกับมีอิทธิพลสูงแต่ผลกระทบตรงจำกัด จึงควรปรึกษาแต่เนิ่นๆ|stakeholder=ผู้มีส่วนได้เสีย;influence=อิทธิพล;consult=ปรึกษา
หาความกังวลเบื้องหลังจุดยืน|You prefer [position]. What risk are you most concerned it will prevent?|You prefer a delayed launch. What risk are you most concerned it will prevent?|คุณต้องการเลื่อนเปิด ความเสี่ยงใดที่กังวลและอยากป้องกันที่สุด|position=จุดยืน;delayed launch=เลื่อนเปิด;prevent=ป้องกัน
ปรับสารตามผู้ฟัง|For [audience], I'd emphasize [concern], while giving [audience] more detail on [topic].|For executives, I'd emphasize exposure, while giving engineers more detail on failure modes.|กับผู้บริหารจะเน้นความเสียหาย ส่วนวิศวกรจะลงรายละเอียดโหมดล้มเหลว|emphasize=เน้น;exposure=ความเสียหายที่อาจเกิด;failure mode=โหมดล้มเหลว
บริหารความคาดหวัง|We can commit to [certain outcome]; [uncertain outcome] depends on [factor].|We can commit to the pilot date; full rollout depends on its results.|ยืนยันวันทดลองได้ ส่วนเปิดเต็มขึ้นกับผลทดลอง|commit to=ให้คำมั่น;full rollout=เปิดเต็ม;depends on=ขึ้นอยู่กับ
สร้างแนวร่วม|If we align on [principle], we can leave [detail] open until [evidence].|If we align on customer safety, we can leave the vendor open until bids arrive.|หากเห็นตรงกันเรื่องความปลอดภัยลูกค้า เราค้างเรื่องผู้ขายไว้จนข้อเสนอมาถึงได้|align on=เห็นตรงกัน;leave open=ยังไม่สรุป;bid=ข้อเสนอประมูล
"""),
    ("B2", 19, "นำทีมผ่านความเปลี่ยนแปลง", """
อธิบายเหตุผลการเปลี่ยน|We are changing [practice] because [evidence], while preserving [value].|We are changing on-call rotations because burnout is rising, while preserving coverage.|เราปรับเวรเพราะภาวะหมดไฟเพิ่ม แต่ยังรักษาความครอบคลุม|on-call rotation=ตารางเวร;burnout=ภาวะหมดไฟ;coverage=ความครอบคลุม
รับรู้การสูญเสีย|This change creates [loss] for [group], even though it offers [benefit].|This change creates less autonomy for teams, even though it offers consistency.|การเปลี่ยนนี้ลดอิสระทีม แม้เพิ่มความสม่ำเสมอ|autonomy=อิสระ;consistency=ความสม่ำเสมอ;even though=แม้ว่า
เปิดพื้นที่คัดค้าน|What would have to be true for this change to fail?|What would have to be true for centralized support to fail?|เงื่อนไขใดจะทำให้การช่วยเหลือแบบรวมศูนย์ล้มเหลว|centralized=รวมศูนย์;fail=ล้มเหลว;have to be true=ต้องเป็นจริง
มอบอำนาจในกรอบ|Teams may adapt [element], provided they meet [non-negotiable].|Teams may adapt the workflow, provided they meet the audit standard.|ทีมปรับขั้นตอนได้หากผ่านมาตรฐานตรวจสอบที่ต่อรองไม่ได้|adapt=ปรับใช้;provided=หาก;audit standard=มาตรฐานตรวจสอบ
วัดการยอมรับจริง|We'll track [behavioral signal], rather than relying solely on [opinion signal].|We'll track actual tool use, rather than relying solely on survey enthusiasm.|จะติดตามการใช้เครื่องมือจริงแทนพึ่งเพียงความกระตือรือร้นในแบบสำรวจ|track=ติดตาม;rely solely on=พึ่งเพียง;enthusiasm=ความกระตือรือร้น
"""),
    ("B2", 20, "สัมภาษณ์ระดับอาวุโส", """
เล่าอิทธิพลโดยไม่มีอำนาจ|I gained alignment by [method], despite not owning [authority].|I gained alignment by making trade-offs visible, despite not owning the budget.|ทำให้เห็นพ้องด้วยการเปิด trade-off แม้ไม่ได้ถืออำนาจงบ|gained alignment=ทำให้เห็นพ้อง;visible=มองเห็นได้;authority=อำนาจ
อธิบายการตัดสินใจคลุมเครือ|With incomplete information, I chose [action] because [principle], while keeping [option] reversible.|With incomplete information, I chose a small pilot because learning mattered, while keeping the rollout reversible.|เมื่อข้อมูลไม่ครบ เลือกทดลองเล็กเพราะต้องเรียนรู้และยังย้อนการเปิดได้|incomplete=ไม่ครบ;reversible=ย้อนกลับได้;principle=หลักคิด
เล่าความล้มเหลวเชิงระบบ|My decision contributed to [failure]; I changed [system] so that [prevention].|My decision contributed to the outage; I changed the review gate so that risky releases need two approvals.|มติของฉันมีส่วนทำให้ระบบล่ม จึงเปลี่ยนด่านตรวจให้งานเสี่ยงต้องอนุมัติสองคน|contributed to=มีส่วนทำให้;review gate=ด่านตรวจ;approval=การอนุมัติ
แสดงการพัฒนาคน|I helped [person/team] move from [state] to [state] by [coaching action].|I helped a new lead move from task assignment to outcome ownership by asking planning questions.|ช่วยหัวหน้าใหม่เปลี่ยนจากแจกงานเป็นรับผิดชอบผลด้วยคำถามวางแผน|outcome ownership=รับผิดชอบผล;task assignment=การแจกงาน;coaching=การโค้ช
ถามเชิงกลยุทธ์กับนายจ้าง|How does this role shape [outcome], and where has progress been hardest?|How does this role shape platform strategy, and where has progress been hardest?|บทบาทนี้กำหนดกลยุทธ์แพลตฟอร์มอย่างไร และส่วนไหนก้าวหน้ายากที่สุด|shape=กำหนด;platform strategy=กลยุทธ์แพลตฟอร์ม;progress=ความก้าวหน้า
"""),
    ("B2", 21, "ควบคุมการเงินและธนาคาร", """
อธิบายหลักแบ่งหน้าที่|No single role should be able to [combined actions], which reduces [risk].|No single role should be able to create and approve a payment, which reduces fraud risk.|ไม่ควรมีบทบาทเดียวสร้างและอนุมัติการชำระได้ เพื่อลดทุจริต|segregation of duties=การแบ่งหน้าที่;fraud=ทุจริต;approve=อนุมัติ
วิเคราะห์ผลต่างกระทบยอด|The break arises because [system A] records [timing], whereas [system B] records [timing].|The break arises because the ledger records authorization, whereas settlement records cleared funds.|ผลต่างเกิดเพราะบัญชีบันทึกตอนอนุมัติ ส่วนระบบชำระดุลบันทึกเงินที่เคลียร์|reconciliation break=ผลต่างกระทบยอด;ledger=บัญชีแยกประเภท;cleared funds=เงินที่ชำระแล้ว
อธิบายความเป็นรายการเดียว|The idempotency key ensures that repeated [request] produces [single outcome].|The idempotency key ensures that repeated transfer requests produce one debit.|คีย์ป้องกันซ้ำทำให้คำขอโอนซ้ำเกิดการหักเงินครั้งเดียว|idempotency key=คีย์ป้องกันซ้ำ;repeated=ซ้ำ;debit=การหักเงิน
ประเมินข้อยกเว้นควบคุม|The override was authorized, but the missing [evidence] weakens [control].|The override was authorized, but the missing rationale weakens the audit trail.|การข้ามกฎได้รับอนุมัติ แต่เหตุผลที่ขาดทำให้ร่องรอยตรวจสอบอ่อนลง|override=การข้ามกฎ;rationale=เหตุผล;audit trail=ร่องรอยตรวจสอบ
เสนอการควบคุมชดเชย|Until [primary control] is restored, we propose [compensating control].|Until automated matching is restored, we propose daily independent reconciliation.|จนกว่าการจับคู่อัตโนมัติกลับมา เสนอกระทบยอดอิสระทุกวัน|automated matching=จับคู่อัตโนมัติ;compensating control=การควบคุมชดเชย;independent=อิสระ
"""),
    ("B2", 22, "ความเสี่ยงและสถานการณ์จำลอง", """
กำหนดสถานการณ์รุนแรงแต่เป็นไปได้|A severe but plausible scenario is [event], triggered by [conditions].|A severe but plausible scenario is regional payment failure, triggered by network loss during peak traffic.|สถานการณ์รุนแรงแต่เป็นไปได้คือชำระเงินระดับภูมิภาคล้มเหลวจากเครือข่ายขาดช่วงพีค|severe=รุนแรง;plausible=เป็นไปได้;peak traffic=ช่วงใช้งานสูงสุด
ติดตามเส้นทางผลกระทบ|The initial failure would affect [area], which could then [second-order effect].|The initial failure would affect merchants, which could then create cash-flow pressure.|ความล้มเหลวแรกกระทบร้านค้าและอาจสร้างแรงกดดันกระแสเงินสด|initial=เริ่มแรก;merchant=ร้านค้า;second-order=ผลกระทบต่อเนื่อง
ทดสอบความทนของการควบคุม|This control works under [normal condition], but may fail if [stress].|This control works under normal volume, but may fail if queues persist overnight.|การควบคุมใช้ได้ในปริมาณปกติ แต่อาจล้มเหลวหากคิวค้างข้ามคืน|persist=คงอยู่;overnight=ข้ามคืน;control=การควบคุม
กำหนดความเสี่ยงที่ยอมรับได้|We can tolerate [exposure] for [duration], but not [boundary].|We can tolerate delayed reports for one day, but not inaccurate balances.|ยอมให้รายงานช้าหนึ่งวันได้ แต่ยอมให้ยอดผิดไม่ได้|tolerate=ยอมรับได้;inaccurate=ไม่ถูกต้อง;boundary=ขอบเขต
ตัดสินใจลงทุนลดความเสี่ยง|The mitigation is justified if [loss avoided] exceeds [cost] over [horizon].|The backup site is justified if avoided downtime exceeds its three-year cost.|ศูนย์สำรองคุ้มค่าหากเวลาหยุดที่เลี่ยงได้มากกว่าต้นทุนสามปี|justified=คุ้มเหตุผล;avoided downtime=เวลาหยุดที่เลี่ยงได้;horizon=กรอบเวลา
"""),
    ("B2", 23, "นำเสนอในภาวะวิกฤต", """
เปิดด้วยสถานการณ์ปัจจุบัน|As of [time], [status]; the immediate priority is [priority].|As of noon, transfers remain delayed; the immediate priority is preventing duplicate debits.|ณ เที่ยง รายการโอนยังล่าช้า สิ่งเร่งด่วนคือป้องกันหักเงินซ้ำ|as of=ณ เวลา;immediate priority=สิ่งเร่งด่วน;duplicate debit=หักเงินซ้ำ
เรียงข้อเท็จจริงกับสิ่งไม่ทราบ|We have confirmed [facts]. We have not yet established [unknown].|We have confirmed the affected region. We have not yet established the trigger.|ยืนยันภูมิภาคที่ได้รับผลแล้ว แต่ยังไม่ทราบตัวกระตุ้น|established=พิสูจน์ชัด;affected=ได้รับผล;trigger=ตัวกระตุ้น
อธิบายการตัดสินใจภายใต้แรงกดดัน|We chose to [action] because [risk], despite [cost].|We chose to pause transfers because of duplicate risk, despite customer delays.|เลือกพักการโอนเพราะเสี่ยงซ้ำ แม้ลูกค้าล่าช้า|pause=พัก;despite=แม้ว่า;customer delay=ความล่าช้าลูกค้า
ตอบคำถามที่ยังตอบไม่ได้|I can't confirm [claim] yet; the evidence we need is [evidence].|I can't confirm a cyberattack yet; the evidence we need is the forensic log review.|ยังยืนยันการโจมตีไซเบอร์ไม่ได้ ต้องมีผลตรวจบันทึกเชิงนิติวิทยาศาสตร์|cyberattack=การโจมตีไซเบอร์;forensic=เชิงนิติวิทยาศาสตร์;confirm=ยืนยัน
ปิดด้วยเจ้าของและรอบอัปเดต|[Owner] is leading [workstream], and the next verified update will be at [time].|Security is leading the investigation, and the next verified update will be at two.|ฝ่ายความปลอดภัยนำการตรวจ และจะอัปเดตที่ยืนยันแล้วตอนบ่ายสอง|workstream=สายงาน;verified update=ข้อมูลอัปเดตที่ยืนยันแล้ว;investigation=การตรวจสอบ
"""),
    ("B2", 24, "การเจรจาขั้นสูง", """
แยกจุดยืนจากผลประโยชน์|Your position is [request]; which underlying concern matters most?|Your position is a fixed fee; which underlying concern matters most, predictability or total cost?|จุดยืนคือค่าคงที่ แต่ความกังวลใดสำคัญสุด ระหว่างคาดการณ์ได้กับต้นทุนรวม|position=จุดยืน;underlying concern=ความกังวลเบื้องหลัง;predictability=การคาดการณ์ได้
แลกหลายประเด็น|We could concede [low-value item] if you can move on [high-value item].|We could concede payment timing if you can move on liability.|เรายอมเรื่องเวลาชำระได้หากคุณขยับเรื่องความรับผิด|concede=ยอมให้;liability=ความรับผิด;payment timing=เวลาชำระ
กำหนดเงื่อนไขตามผลงาน|If [outcome] reaches [threshold], then [term]; otherwise, [fallback].|If uptime reaches 99.9 percent, then the bonus applies; otherwise, the base fee remains.|หากเวลาพร้อมใช้ถึง 99.9 เปอร์เซ็นต์จึงมีโบนัส มิฉะนั้นใช้ค่าพื้นฐาน|uptime=เวลาพร้อมใช้;threshold=เกณฑ์;base fee=ค่าพื้นฐาน
หยุดทางตันอย่างสร้างสรรค์|We appear stuck on [issue]. Could we agree on [principle] and return with options?|We appear stuck on liability. Could we agree on shared risk and return with options?|ดูเหมือนติดเรื่องความรับผิด เราตกลงหลักแบ่งความเสี่ยงแล้วกลับมาพร้อมตัวเลือกได้ไหม|stuck=ติดทางตัน;shared risk=ความเสี่ยงร่วม;return with=กลับมาพร้อม
ปิดพร้อมตรวจความเข้าใจ|Before we close, let me test the agreement against [scenario].|Before we close, let me test the agreement against a two-day outage.|ก่อนจบ ขอทดสอบความเข้าใจข้อตกลงกับกรณีระบบล่มสองวัน|test the agreement=ทดสอบข้อตกลง;outage=ระบบล่ม;before we close=ก่อนจบ
"""),
]


def grammar_rows(block: str) -> list[tuple[str, str, str, str, str, list[int], str]]:
    parsed = []
    for line in block.strip().splitlines():
        parts = [part.strip() for part in line.split("|")]
        if len(parts) != 7:
            raise ValueError(f"expected seven grammar fields: {line}")
        title, focus, pattern, example, meaning, units, vocab = parts
        parsed.append((title, focus, pattern, example, meaning,
                       [int(x) for x in units.split(",") if x], vocab))
    if len(parsed) != 5:
        raise ValueError("each grammar unit must contain exactly five lessons")
    return parsed


GRAMMAR_UNITS: list[tuple[str, int, str, str]] = [
    ("Pre-A1", 5, "be และคำสรรพนาม", """
บอกว่าฉันเป็นใคร|be: I am|I am [name/role].|I am Dao. I am a student.|ฉันชื่อดาว ฉันเป็นนักเรียน||student=นักเรียน;name=ชื่อ;role=บทบาท
บอกว่าอีกคนเป็นใคร|be: you are; he/she is|[Person] is [name/role].|She is Mali. She is our guide.|เธอชื่อมะลิ เธอเป็นผู้นำทางของเรา||guide=ผู้นำทาง;she=เธอ;our=ของเรา
บอกว่าเราและเขาเป็นใคร|be: we/they are|[People] are [group/role].|We are new here. They are our teachers.|พวกเราเป็นคนใหม่ที่นี่ พวกเขาเป็นครูของเรา||new=ใหม่;teacher=ครู;here=ที่นี่
ใช้รูปย่อของ be|be contractions|I'm [detail]. You're [detail].|I'm ready. You're early.|ฉันพร้อม คุณมาเร็ว||ready=พร้อม;early=เร็ว;contraction=รูปย่อ
ชี้คนและสิ่งของ|this/that with be|This is [near thing]. That is [far thing].|This is my phone. That is the exit.|นี่คือโทรศัพท์ฉัน นั่นคือทางออก||phone=โทรศัพท์;exit=ทางออก;near=ใกล้
"""),
    ("Pre-A1", 6, "คำถามและปฏิเสธด้วย be", """
ถามข้อมูลตัวตน|be yes/no question|Are you [detail]?|Are you the new cashier?|คุณคือพนักงานเก็บเงินคนใหม่ใช่ไหม|49|cashier=พนักงานเก็บเงิน;new=ใหม่;question=คำถาม
ตอบสั้นด้วย be|short answers with be|Yes, [pronoun] am/is/are. No, [pronoun]'m not/isn't/aren't.|Yes, I am. No, she isn't.|ใช่ ฉันเป็น ไม่ใช่ เธอไม่ได้เป็น|49,51|yes=ใช่;no=ไม่;short answer=คำตอบสั้น
ถามด้วย wh-word|wh-question with be|Where/What/Who is [subject]?|Where is the meeting room?|ห้องประชุมอยู่ที่ไหน|49|meeting room=ห้องประชุม;where=ที่ไหน;who=ใคร
บอกว่าไม่ใช่|negative be|[Subject] am not/isn't/aren't [detail].|The café isn't open.|คาเฟ่ไม่ได้เปิด||open=เปิด;café=คาเฟ่;negative=รูปปฏิเสธ
ตรวจสอบข้อมูล|be question and correction|Is [statement]? No, [correction].|Is the class at nine? No, it's at ten.|ชั้นเรียนเก้าโมงไหม ไม่ใช่ สิบโมง|49,51|class=ชั้นเรียน;at nine=เก้าโมง;correction=การแก้ข้อมูล
"""),
    ("Pre-A1", 7, "Present simple พื้นฐาน", """
บอกสิ่งที่ทำประจำ|present simple: I/you|I/You [base verb] [detail].|I walk to work.|ฉันเดินไปทำงาน|2|walk=เดิน;work=ที่ทำงาน;routine=กิจวัตร
พูดถึง he และ she|present simple third person|He/She [verb-s] [detail].|He works at a hotel.|เขาทำงานที่โรงแรม|2|hotel=โรงแรม;works=ทำงาน;third person=บุรุษที่สาม
บอกว่าไม่ทำ|present simple negative|I/You don't [verb]. He/She doesn't [verb].|I don't drive. She doesn't smoke.|ฉันไม่ขับรถ เธอไม่สูบบุหรี่|2|drive=ขับรถ;smoke=สูบบุหรี่;doesn't=ไม่ทำ
ถามกิจวัตร|present simple question|Do/Does [subject] [verb]?|Do you cook at home?|คุณทำอาหารที่บ้านไหม|49|cook=ทำอาหาร;at home=ที่บ้าน;do=คำช่วยคำถาม
บอกความชอบ|like/love with noun or -ing|I like [noun/verb-ing].|I like music. I love swimming.|ฉันชอบดนตรีและชอบว่ายน้ำมาก|53,58|music=ดนตรี;swimming=ว่ายน้ำ;love=ชอบมาก
"""),
    ("Pre-A1", 8, "คำนาม article และตำแหน่ง", """
เลือก a หรือ an|indefinite article a/an|It's a/an [singular thing].|It's an umbrella.|มันคือร่มหนึ่งคัน|71,72|umbrella=ร่ม;singular=เอกพจน์;article=คำนำหน้านาม
ใช้ the เมื่อรู้ว่าสิ่งใด|definite article the|The [known thing] is [place].|The key is on the table.|กุญแจดอกที่รู้กันอยู่บนโต๊ะ|72,73|key=กุญแจ;table=โต๊ะ;known=ที่รู้กัน
พูดหนึ่งชิ้นและหลายชิ้น|singular and plural nouns|There is one [noun]. There are [number] [plural].|There is one cup. There are two plates.|มีถ้วยหนึ่งใบและจานสองใบ|79,84|cup=ถ้วย;plate=จาน;plural=พหูพจน์
ใช้ in on under|basic place prepositions|The [thing] is in/on/under [place].|The bag is under the chair.|กระเป๋าอยู่ใต้เก้าอี้|123|bag=กระเป๋า;under=ใต้;chair=เก้าอี้
ถามและบอกสิ่งที่มี|there is/are|Is/Are there [thing]? There is/are [thing].|Is there a bank nearby? There is one opposite the park.|มีธนาคารใกล้ๆ ไหม มีหนึ่งแห่งตรงข้ามสวน|84|bank=ธนาคาร;opposite=ตรงข้าม;nearby=ใกล้ๆ
"""),
    ("Pre-A1", 9, "can have และคำสั่งง่าย", """
บอกความสามารถ|can for ability|I/She can [verb].|I can swim. She can drive.|ฉันว่ายน้ำได้ เธอขับรถได้|26|swim=ว่ายน้ำ;drive=ขับรถ;ability=ความสามารถ
บอกว่าทำไม่ได้|cannot/can't|[Subject] can't [verb].|I can't hear you.|ฉันได้ยินคุณไม่ชัด|26|hear=ได้ยิน;can't=ไม่สามารถ;you=คุณ
ขออนุญาตง่าย|can for permission|Can I [verb], please?|Can I sit here, please?|ขอนั่งตรงนี้ได้ไหม|26,37|sit=นั่ง;permission=การอนุญาต;please=กรุณา
บอกสิ่งที่มี|have/have got|I have / I've got [thing].|I've got a ticket.|ฉันมีตั๋วหนึ่งใบ|17|ticket=ตั๋ว;have got=มี;thing=สิ่งของ
ทำตามคำสั่ง|imperative|[Base verb] [object], please.|Open the window, please.|กรุณาเปิดหน้าต่าง||open=เปิด;window=หน้าต่าง;instruction=คำสั่ง
"""),
    ("A1", 5, "Present simple เพื่อชีวิตประจำวัน", """
บอกกิจวัตรพร้อมเวลา|present simple with time phrase|I [verb] [time phrase].|I start work at half past eight.|ฉันเริ่มงานแปดโมงครึ่ง|2,121|start work=เริ่มงาน;half past=ครึ่งชั่วโมง;time phrase=วลีเวลา
ใช้คำบอกความถี่|adverbs of frequency|[Subject] always/usually/often/sometimes/never [verb].|I usually take the train.|ปกติฉันขึ้นรถไฟ|2,110|usually=โดยปกติ;take the train=ขึ้นรถไฟ;frequency=ความถี่
ถามรายละเอียดกิจวัตร|present simple wh-question|When/Where/Why do/does [subject] [verb]?|Where does your sister work?|พี่สาวหรือน้องสาวคุณทำงานที่ไหน|49|sister=พี่สาวหรือน้องสาว;where=ที่ไหน;work=ทำงาน
ปฏิเสธและแก้ข้อมูล|present simple negative contrast|[Subject] don't/doesn't [verb]; [subject] [verb].|He doesn't drive; he takes the bus.|เขาไม่ขับรถ เขาขึ้นรถเมล์|2|takes the bus=ขึ้นรถเมล์;drive=ขับรถ;contrast=ความต่าง
ตอบสั้นและต่อข้อมูล|present simple short answer|Yes, [subject] do/does. [Extra detail].|Yes, she does. She cooks every evening.|ใช่ เธอทำ เธอทำอาหารทุกเย็น|49,51|every evening=ทุกเย็น;cooks=ทำอาหาร;extra detail=ข้อมูลเพิ่ม
"""),
    ("A1", 6, "Present continuous ตอนนี้", """
บอกสิ่งที่กำลังเกิด|present continuous affirmative|[Subject] am/is/are [verb-ing] now.|I'm waiting for the bus now.|ตอนนี้ฉันกำลังรอรถเมล์|1|waiting=กำลังรอ;bus=รถเมล์;now=ตอนนี้
สะกดกริยา ing|verb-ing spelling|[Verb] becomes [verb-ing].|She is making dinner.|เธอกำลังทำอาหารเย็น|1|making=กำลังทำ;dinner=อาหารเย็น;spelling=การสะกด
ถามว่ากำลังทำอะไร|present continuous question|What is/are [subject] [verb-ing]?|What are they talking about?|พวกเขากำลังคุยเรื่องอะไร|1,49|talking=กำลังคุย;about=เกี่ยวกับ;they=พวกเขา
บอกสิ่งที่ไม่ได้เกิด|present continuous negative|[Subject] am not/isn't/aren't [verb-ing].|The printer isn't working.|เครื่องพิมพ์ไม่ได้กำลังทำงาน|1|printer=เครื่องพิมพ์;working=ทำงาน;negative=รูปปฏิเสธ
แยกตอนนี้กับเป็นประจำ|present continuous vs present simple|I'm [verb-ing] now, but I usually [verb].|I'm taking a taxi today, but I usually walk.|วันนี้ฉันนั่งแท็กซี่ แต่ปกติเดิน|3,4|taxi=แท็กซี่;today=วันนี้;usually=โดยปกติ
"""),
    ("A1", 7, "Past simple เหตุการณ์จบแล้ว", """
บอกอดีตด้วย be|past simple be|[Subject] was/were [detail].|We were busy yesterday.|เมื่อวานเรายุ่ง|5|busy=ยุ่ง;yesterday=เมื่อวาน;were=เคยอยู่
ใช้กริยาปกติ|past simple regular verb|[Subject] [verb-ed] [past time].|I visited my aunt last weekend.|ฉันไปเยี่ยมป้าเมื่อสุดสัปดาห์ก่อน|5|visited=ไปเยี่ยม;aunt=ป้า;last weekend=สุดสัปดาห์ก่อน
ใช้กริยาไม่ปกติ|past simple irregular verb|[Subject] [past verb] [detail].|She went home early.|เธอกลับบ้านเร็ว|5|went=ไปแล้ว;early=เร็ว;irregular=ไม่ปกติ
บอกว่าไม่ได้ทำ|past simple negative|[Subject] didn't [base verb].|I didn't see the message.|ฉันไม่เห็นข้อความ|5|didn't=ไม่ได้;message=ข้อความ;see=เห็น
ถามเรื่องอดีต|past simple question|Did [subject] [base verb]?|Did you call the clinic?|คุณโทรหาคลินิกหรือยัง|5,49|call=โทร;clinic=คลินิก;did=คำช่วยอดีต
"""),
    ("A1", 8, "อนาคตและแผน", """
บอกแผนด้วย going to|be going to for intention|[Subject] am/is/are going to [verb].|I'm going to apply for the course.|ฉันตั้งใจจะสมัครหลักสูตร|20|apply=สมัคร;course=หลักสูตร;intention=ความตั้งใจ
คาดการณ์จากหลักฐาน|going to prediction|It's going to [event]; [present evidence].|It's going to rain; look at those clouds.|ฝนกำลังจะตก ดูเมฆเหล่านั้น|20|rain=ฝนตก;cloud=เมฆ;evidence=หลักฐาน
ตัดสินใจทันทีด้วย will|will for instant decision|I'll [verb] [detail].|I'll answer the phone.|ฉันจะรับโทรศัพท์เอง|21|answer=รับสาย;phone=โทรศัพท์;instant decision=การตัดสินใจทันที
เสนอและสัญญา|will for offer/promise|I'll [help/action] for you.|I'll carry that bag for you.|ฉันจะถือกระเป๋าใบนั้นให้|21,22|carry=ถือ;offer=ข้อเสนอช่วย;promise=คำสัญญา
ใช้ present continuous กับนัด|present continuous for arrangement|I'm [verb-ing] [future time].|I'm meeting the dentist on Thursday.|ฉันมีนัดหมอฟันวันพฤหัส|19|dentist=หมอฟัน;meeting=มีนัด;Thursday=วันพฤหัส
"""),
    ("A1", 9, "ปริมาณ เปรียบเทียบ และการเชื่อม", """
แยกนามนับได้|countable and uncountable nouns|a/an [countable]; some [uncountable/plural]|a banana; some rice; some bananas|กล้วยหนึ่งลูก ข้าวจำนวนหนึ่ง กล้วยหลายลูก|69,70,71|banana=กล้วย;rice=ข้าว;countable=นับได้
ถามปริมาณ|how much/how many|How much [uncountable]? How many [plural]?|How much water? How many bottles?|น้ำเท่าไร ขวดกี่ใบ|69,70,87|water=น้ำ;bottle=ขวด;quantity=ปริมาณ
เปรียบเทียบขั้นกว่า|comparative adjective|[A] is [comparative] than [B].|The train is faster than the bus.|รถไฟเร็วกว่ารถเมล์|105,106,107|faster=เร็วกว่า;train=รถไฟ;than=กว่า
บอกตำแหน่งและเวลา|in/on/at for place and time|[Event/thing] is in/on/at [time/place].|The meeting is at ten in Room 4.|ประชุมสิบโมงที่ห้องสี่|121,123,124|meeting=ประชุม;room=ห้อง;at ten=สิบโมง
เชื่อมเหตุผลง่าย|and/but/because/so|[Clause] because [reason], so [result].|I was tired because I worked late, so I went home.|ฉันเหนื่อยเพราะทำงานดึก จึงกลับบ้าน|113|tired=เหนื่อย;worked late=ทำงานดึก;result=ผลลัพธ์
"""),
    ("A2", 25, "Present และ Past ในเรื่องเล่า", """
แยกกิจวัตรกับเหตุชั่วคราว|present simple vs continuous|I usually [routine], but this week I'm [temporary action].|I usually drive, but this week I'm taking the train.|ปกติขับรถ แต่สัปดาห์นี้กำลังขึ้นรถไฟ|3,4|usually=ปกติ;this week=สัปดาห์นี้;temporary=ชั่วคราว
เล่าเหตุการณ์ต่อเนื่องในอดีต|past continuous|At [time], [subject] was/were [verb-ing].|At eight, I was waiting outside.|ตอนสองทุ่มฉันกำลังรออยู่ข้างนอก|6|waiting=กำลังรอ;outside=ข้างนอก;at eight=ตอนแปดโมง
แทรกเหตุสั้นในเหตุยาว|past continuous vs past simple|While [ongoing action], [short event].|While I was cooking, the alarm rang.|ระหว่างกำลังทำอาหาร สัญญาณเตือนดัง|5,6|while=ระหว่าง;alarm=สัญญาณเตือน;rang=ดังขึ้น
ถามและปฏิเสธในอดีต|past questions and negatives|Why did [subject] [verb]? [Subject] didn't [verb].|Why did the train stop? It didn't move for an hour.|ทำไมรถไฟหยุด มันไม่ขยับหนึ่งชั่วโมง|5,49|stop=หยุด;move=เคลื่อน;for an hour=หนึ่งชั่วโมง
พูดถึงนิสัยเก่า|used to|[Subject] used to [verb], but now [present].|I used to work nights, but now I work mornings.|เคยทำงานกลางคืน แต่ตอนนี้ทำช่วงเช้า|18|used to=เคยทำ;nights=ช่วงกลางคืน;mornings=ช่วงเช้า
"""),
    ("A2", 26, "Present perfect เชื่อมอดีตกับปัจจุบัน", """
เล่าประสบการณ์|present perfect experience|[Subject] have/has [past participle] [experience].|I've visited Chiang Mai three times.|ฉันเคยไปเชียงใหม่สามครั้ง|7,8|visited=เคยไป;three times=สามครั้ง;experience=ประสบการณ์
ถามว่าเคยหรือยัง|ever/never with present perfect|Have you ever [past participle]? I've never [past participle].|Have you ever flown alone? I've never flown alone.|เคยบินคนเดียวไหม ฉันไม่เคยบินคนเดียว|7,49|ever=เคยไหม;never=ไม่เคย;flown=บินแล้ว
บอกผลที่เห็นตอนนี้|present perfect result|[Subject] has [past participle], so [present result].|The bus has left, so we'll need a taxi.|รถเมล์ออกไปแล้ว จึงต้องใช้แท็กซี่|7,8|left=ออกไปแล้ว;need=ต้องการ;result=ผล
ใช้ just already yet|present perfect with just/already/yet|I've just/already [participle]. I haven't [participle] yet.|I've already sent it, but I haven't received a reply yet.|ส่งไปแล้ว แต่ยังไม่ได้รับคำตอบ|7,111|already=แล้ว;yet=ยัง;reply=คำตอบ
บอกช่วงเวลาด้วย for และ since|present perfect with for/since|[Subject] have/has [participle] for/since [time].|We've lived here since 2022.|เราอยู่ที่นี่มาตั้งแต่ปี 2022|11,12|since=ตั้งแต่;for=เป็นเวลา;lived=อาศัย
"""),
    ("A2", 27, "Future และ Modal ในแผนจริง", """
แยกแผนกับการตัดสินใจทันที|going to vs will|I'm going to [plan]. I'll [instant response].|I'm going to cook. I'll answer the door first.|ตั้งใจจะทำอาหาร แต่จะไปเปิดประตูก่อน|20,21,23|plan=แผน;answer the door=เปิดประตู;first=ก่อน
ใช้ตารางเวลาในอนาคต|present simple for timetable|[Service/event] leaves/starts at [time].|The last train leaves at 11.30.|รถไฟเที่ยวสุดท้ายออกห้าทุ่มครึ่ง|19|last train=รถไฟเที่ยวสุดท้าย;leaves=ออก;timetable=ตารางเวลา
ใช้ when และ if กับอนาคต|future time/condition clause|When/If [present], [subject] will [verb].|When I arrive, I'll call you.|เมื่อถึงแล้วจะโทรหา|25|arrive=มาถึง;when=เมื่อ;call=โทร
บอกความจำเป็นและข้อห้าม|have to/must/mustn't|We have to [duty]. You mustn't [prohibition].|We have to sign in. You mustn't share the code.|เราต้องลงชื่อเข้า ห้ามแชร์รหัส|31,32|sign in=ลงชื่อเข้า;share=แชร์;code=รหัส
ให้คำแนะนำและบอกความเป็นไปได้|should/may/might|You should [advice]. It may/might [possibility].|You should take a jacket. It might get cold.|ควรเอาเสื้อคลุมไป อากาศอาจหนาว|29,30,33|jacket=เสื้อคลุม;might=อาจ;advice=คำแนะนำ
"""),
    ("A2", 28, "Article ปริมาณ และการเปรียบเทียบ", """
เลือก a the หรือไม่ใช้ article|articles in context|I saw a [thing]. The [thing] was [detail]. [Plural/general] are [general fact].|I saw a dog. The dog was wet. Dogs need exercise.|ฉันเห็นสุนัขหนึ่งตัว สุนัขตัวนั้นเปียก โดยทั่วไปสุนัขต้องออกกำลัง|71,72,73,75|wet=เปียก;exercise=การออกกำลัง;general=โดยทั่วไป
ใช้ some any no|some/any/no|There is some [noun]. Is there any [noun]? There is no [noun].|There is some milk. Is there any coffee? There is no tea.|มีนม มีกาแฟไหม ไม่มีชา|85,86|milk=นม;coffee=กาแฟ;tea=ชา
บอกปริมาณมากน้อย|much/many/a few/a little|We have a few [plural] and a little [uncountable].|We have a few chairs and a little time.|เรามีเก้าอี้ไม่กี่ตัวและเวลานิดหน่อย|87|a few=เล็กน้อยที่นับได้;a little=เล็กน้อยที่นับไม่ได้;chair=เก้าอี้
ใช้ขั้นกว่าและขั้นสูงสุด|comparative and superlative|[A] is [comparative]; [B] is the [superlative].|This route is shorter, but the river road is the safest.|เส้นนี้สั้นกว่า แต่ถนนริมน้ำปลอดภัยที่สุด|105,106,107,108|route=เส้นทาง;shorter=สั้นกว่า;safest=ปลอดภัยที่สุด
ใช้ preposition เวลาและการเคลื่อนที่|time/place/movement prepositions|[Subject] goes from [A] to/into [B] at [time].|The bus goes from the airport to town at six.|รถเมล์ไปจากสนามบินเข้าเมืองตอนหกโมง|121,123,126|airport=สนามบิน;town=ตัวเมือง;from=จาก
"""),
    ("A2", 29, "ประโยคที่ทำให้พูดได้หลากหลาย", """
พูดถึงกิจกรรมที่ชอบและแผน|gerund vs infinitive|I enjoy [verb-ing], and I plan to [verb].|I enjoy hiking, and I plan to climb Doi Inthanon.|ฉันชอบเดินป่าและวางแผนจะขึ้นดอยอินทนนท์|53,54|hiking=เดินป่า;plan to=วางแผนจะ;climb=ปีน
บอกจุดประสงค์|infinitive of purpose|[Subject] [action] to [purpose].|I called to confirm the booking.|ฉันโทรเพื่อยืนยันการจอง|64|confirm=ยืนยัน;booking=การจอง;purpose=จุดประสงค์
ขยายคำนามด้วย relative clause|defining relative clause|[Noun] who/that/which [clause].|I need a charger that works with this phone.|ฉันต้องการที่ชาร์จที่ใช้กับโทรศัพท์นี้ได้|92,93|charger=ที่ชาร์จ;works with=ใช้กับ;relative clause=อนุประโยคขยาย
ใช้ passive เมื่อเน้นสิ่งที่เกิด|basic passive|[Thing] is/was [past participle].|The package was delivered yesterday.|พัสดุถูกส่งเมื่อวาน|42,43|package=พัสดุ;delivered=ถูกส่ง;passive=รูปถูกกระทำ
วางเงื่อนไขจริง|zero and first conditional|If [present], [present/will + verb].|If the card fails, try again. If it fails again, I'll call the bank.|หากบัตรใช้ไม่ได้ให้ลองใหม่ หากยังไม่ได้ฉันจะโทรหาธนาคาร|38|fails=ใช้ไม่ได้;try again=ลองใหม่;condition=เงื่อนไข
    """),
    ("B1", 25, "Perfect และ Past ที่สัมพันธ์กัน", """
เน้นกิจกรรมต่อเนื่องถึงปัจจุบัน|present perfect continuous|[Subject] have/has been [verb-ing] for/since [time].|I've been reviewing applications since nine.|ฉันตรวจใบสมัครต่อเนื่องมาตั้งแต่เก้าโมง|9,10,11|reviewing=กำลังตรวจ;application=ใบสมัคร;since=ตั้งแต่
แยกผลกับกิจกรรม|present perfect simple vs continuous|I've [participle] [result]; I've been [verb-ing] [activity].|I've answered ten emails; I've been working since dawn.|ตอบอีเมลแล้วสิบฉบับ และทำงานต่อเนื่องตั้งแต่รุ่งเช้า|9,10|answered=ตอบแล้ว;since dawn=ตั้งแต่รุ่งเช้า;result=ผลลัพธ์
เลือก present perfect หรือ past simple|present perfect vs past simple|I've [participle] [unfinished time]. I [past verb] [finished time].|I've met her twice this year. I met her in May.|ปีนี้พบเธอสองครั้ง โดยครั้งหนึ่งพบเดือนพฤษภาคม|13,14|twice=สองครั้ง;this year=ปีนี้;in May=ในเดือนพฤษภาคม
เรียงอดีตก่อนหลัง|past perfect|By the time [past event], [subject] had [participle].|By the time I arrived, the meeting had ended.|เมื่อฉันมาถึง ประชุมจบไปแล้ว|15|by the time=เมื่อถึงเวลา;arrived=มาถึง;had ended=จบไปแล้ว
อธิบายสาเหตุจากกิจกรรมก่อนหน้า|past perfect continuous|[Subject] had been [verb-ing], so [past result].|They had been driving all night, so they were exhausted.|พวกเขาขับรถมาตลอดคืนจึงหมดแรง|16|all night=ตลอดคืน;exhausted=หมดแรง;driving=ขับรถ
"""),
    ("B1", 26, "Future forms และกำหนดเวลา", """
เลือกแผน การคาดการณ์ และตาราง|future form choice|I'm going to [plan]. I think [subject] will [prediction]. [Event] starts at [time].|I'm going to apply. I think they'll reply soon. The course starts Monday.|จะสมัคร คิดว่าเขาจะตอบเร็ว หลักสูตรเริ่มจันทร์|19,20,21,23|apply=สมัคร;reply=ตอบ;course=หลักสูตร
พูดถึงสิ่งที่จะกำลังเกิด|future continuous|At [future time], [subject] will be [verb-ing].|At this time tomorrow, we'll be presenting.|เวลานี้พรุ่งนี้เราจะกำลังนำเสนอ|24|this time tomorrow=เวลานี้พรุ่งนี้;presenting=นำเสนอ;future=อนาคต
พูดถึงสิ่งที่จะเสร็จ|future perfect|By [future time], [subject] will have [participle].|By Friday, we'll have completed the migration.|ภายในศุกร์เราจะย้ายระบบเสร็จแล้ว|24|by Friday=ภายในศุกร์;completed=เสร็จสิ้น;migration=การย้ายระบบ
ใช้อนาคตหลัง when และ if|future clauses|When/If [present or present perfect], [future clause].|When you've checked the figures, we'll publish them.|เมื่อคุณตรวจตัวเลขเสร็จ เราจะเผยแพร่|25|checked=ตรวจเสร็จ;figure=ตัวเลข;publish=เผยแพร่
คาดการณ์อย่างมีระดับความมั่นใจ|will/may/might prediction|[Outcome] will/may/might [verb], depending on [factor].|Demand might rise, depending on the final price.|ความต้องการอาจเพิ่ม ขึ้นกับราคาสุดท้าย|21,29,30|demand=ความต้องการ;depending on=ขึ้นอยู่กับ;final price=ราคาสุดท้าย
"""),
    ("B1", 27, "Modal เพื่อความแน่นอนและหน้าที่", """
อนุมานปัจจุบัน|must/can't for deduction|[Subject] must/can't be [conclusion] because [evidence].|She must be nearby because her laptop is here.|เธอต้องอยู่ใกล้เพราะแล็ปท็อปอยู่ที่นี่|28|nearby=ใกล้ๆ;evidence=หลักฐาน;deduction=การอนุมาน
อนุมานอดีต|must/might/can't have|[Subject] must/might/can't have [participle].|They might have missed the last train.|พวกเขาอาจพลาดรถไฟเที่ยวสุดท้าย|27,28,29|might have=อาจได้;missed=พลาด;last train=รถเที่ยวสุดท้าย
แยกหน้าที่กับคำแนะนำ|must/have to/should|You must/have to [duty]; you should [advice].|You have to wear a badge; you should arrive early.|คุณต้องติดบัตร และควรมาถึงเร็ว|31,33|badge=บัตร;arrive early=มาถึงเร็ว;duty=หน้าที่
พูดถึงความจำเป็นที่ไม่มี|don't have to/needn't|You don't have to/needn't [verb].|You don't have to print the form.|คุณไม่จำเป็นต้องพิมพ์แบบฟอร์ม|32|print=พิมพ์;form=แบบฟอร์ม;needn't=ไม่จำเป็น
ขอและเสนออย่างสุภาพ|could/would requests and offers|Could/Would you [request]? Would you like [offer]?|Could you review this? Would you like more time?|ช่วยตรวจสิ่งนี้ได้ไหม ต้องการเวลาเพิ่มไหม|37|review=ตรวจทาน;more time=เวลาเพิ่ม;polite=สุภาพ
"""),
    ("B1", 28, "Conditionals Passive และ Reported speech", """
สมมติสถานการณ์ปัจจุบัน|second conditional|If [past simple], [subject] would [verb].|If we had more time, we would test every case.|หากมีเวลามากกว่านี้ เราจะทดสอบทุกกรณี|38|had more time=มีเวลามากขึ้น;would test=จะทดสอบ;case=กรณี
เสียดายอดีต|third conditional|If [subject] had [participle], [subject] would have [participle].|If we'd backed up the file, we wouldn't have lost it.|หากสำรองไฟล์ไว้ เราคงไม่ทำหาย|40|backed up=สำรองแล้ว;lost=ทำหาย;regret=ความเสียดาย
เลือก passive ตาม tense|passive across tenses|[Thing] is/was/has been/will be [participle].|The request has been approved and will be processed today.|คำขอได้รับอนุมัติแล้วและจะประมวลผลวันนี้|42,43,44|approved=ได้รับอนุมัติ;processed=ประมวลผล;request=คำขอ
รายงานคำพูด|reported statements|[Person] said that [backshifted clause].|Mina said that the supplier was late.|มีนาบอกว่าผู้ขายมาสาย|47,48|said that=บอกว่า;supplier=ผู้ขาย;late=สาย
รายงานคำถามทางอ้อม|reported/indirect questions|[Person] asked [question word] [subject] [verb].|He asked when the report would be ready.|เขาถามว่ารายงานจะพร้อมเมื่อไร|50|asked=ถาม;would be ready=จะพร้อม;indirect=ทางอ้อม
"""),
    ("B1", 29, "Relative Gerund Infinitive และ Linking", """
ให้ข้อมูลจำเป็น|defining relative clause|[Noun] who/that/which [defining detail].|Choose the option that costs less to maintain.|เลือกตัวเลือกที่มีค่าดูแลน้อยกว่า|92,93|maintain=ดูแลรักษา;option=ตัวเลือก;defining=ที่จำเป็น
เพิ่มข้อมูลเสริม|non-defining relative clause|[Noun], who/which [extra detail], [main clause].|The clinic, which opened last year, now serves 500 families.|คลินิกซึ่งเปิดปีก่อน ตอนนี้บริการ 500 ครอบครัว|94,95,96|clinic=คลินิก;serves=ให้บริการ;extra detail=ข้อมูลเสริม
เลือก gerund หรือ infinitive ตามกริยา|verb + -ing/to|[Verb] [gerund/to-infinitive].|We avoided delaying the launch and decided to reduce scope.|เราเลี่ยงการเลื่อนเปิดและตัดสินใจลดขอบเขต|53,54,55|avoided=หลีกเลี่ยง;decided=ตัดสินใจ;scope=ขอบเขต
แยกความหมายของ ing และ to|meaning change with -ing/to|remember/stop/try [verb-ing or to-verb]|Remember to lock the door; I remember locking it.|จำไว้ว่าต้องล็อกประตู และฉันจำได้ว่าได้ล็อกแล้ว|56,57|remember=จำ;lock=ล็อก;meaning change=ความหมายเปลี่ยน
เชื่อมข้อโต้แย้ง|although/despite/therefore|Although [clause], [clause]. Despite [noun], [clause]. Therefore, [result].|Although costs rose, demand held. Despite the increase, we grew. Therefore, we will continue.|แม้ต้นทุนเพิ่ม ความต้องการคงอยู่ และเรายังโต ดังนั้นจะทำต่อ|113|although=แม้ว่า;despite=แม้;therefore=ดังนั้น
"""),
    ("B2", 25, "Tense synthesis ครบ 12 รูป", """
เลือก present simple หรือ continuous|present simple and present continuous|[Routine/state] versus [temporary/current activity].|I manage the team, but this month I'm covering support.|ฉันบริหารทีมเป็นปกติ แต่เดือนนี้กำลังช่วยฝ่ายสนับสนุน|1,2,3,4|manage=บริหาร;covering=ทำหน้าที่แทน;temporary=ชั่วคราว
เล่าอดีตด้วย simple และ continuous|past simple and past continuous|While [background], [completed event].|While we were deploying, the network failed.|ระหว่างกำลังติดตั้ง เครือข่ายล้มเหลว|5,6|deploying=กำลังติดตั้ง;network=เครือข่าย;failed=ล้มเหลว
เปรียบ present perfect สองรูป|present perfect simple and continuous|[Completed/result count] versus [duration/activity].|We've fixed six defects; we've been testing since dawn.|แก้ข้อบกพร่องแล้วหกรายการ และทดสอบต่อเนื่องตั้งแต่รุ่งเช้า|7,8,9,10|defect=ข้อบกพร่อง;testing=ทดสอบ;since dawn=ตั้งแต่รุ่งเช้า
เรียงอดีตด้วย past perfect สองรูป|past perfect simple and continuous|[Earlier completion] and [earlier ongoing cause] before [past event].|The backup had finished, but the system had been slowing before it failed.|ข้อมูลสำรองเสร็จแล้ว แต่ระบบช้าต่อเนื่องก่อนล้ม|15,16|backup=ข้อมูลสำรอง;slowing=ช้าลง;before=ก่อน
รู้จักอนาคตสี่รูป|future simple/continuous/perfect/perfect continuous|will [verb]; will be [verb-ing]; will have [participle]; will have been [verb-ing]|By noon I'll decide; at two I'll be presenting; by five I'll have finished. In June, I'll have been leading the team for a year.|เที่ยงจะตัดสินใจ บ่ายสองจะกำลังนำเสนอ ห้าโมงจะเสร็จ และมิถุนายนจะนำทีมครบปี|21,24|by noon=ภายในเที่ยง;presenting=นำเสนอ;for a year=เป็นเวลาหนึ่งปี
"""),
    ("B2", 26, "Modality และเงื่อนไขเชิงนัย", """
ประเมินอดีตด้วย modal perfect|modal perfect|[Subject] should/could/might have [participle].|We should have consulted users; we could have avoided rework.|เราควรปรึกษาผู้ใช้และอาจเลี่ยงงานแก้ซ้ำได้|27,29,33,34|consulted=ได้ปรึกษา;avoided=ได้เลี่ยง;rework=งานแก้ซ้ำ
ลดความหนักแน่นอย่างเหมาะสม|hedged modality|This may/might/could [claim], although [limit].|This may indicate fraud, although an error could explain it.|สิ่งนี้อาจชี้ทุจริต แม้ข้อผิดพลาดก็อธิบายได้|29,30|indicate=ชี้;fraud=ทุจริต;although=แม้ว่า
ผสมเงื่อนไขอดีตกับปัจจุบัน|mixed conditional|If [past perfect], [present conditional result].|If we had documented the decision, we wouldn't be debating it now.|หากบันทึกมติไว้ ตอนนี้คงไม่ต้องถกกัน|39,40|documented=บันทึก;debating=ถกเถียง;mixed=แบบผสม
ใช้ wish กับปัจจุบันและอดีต|wish/regret|I wish [past form]. I wish [past perfect].|I wish the process were simpler. I wish we'd tested it earlier.|อยากให้กระบวนการง่ายกว่านี้ และเสียดายที่ไม่ได้ทดสอบเร็วกว่านี้|39,40,41|wish=อยากให้;earlier=เร็วกว่านี้;regret=เสียดาย
กำหนดเงื่อนไขแบบเป็นทางการ|unless/as long as/provided|[Outcome] unless/as long as/provided [condition].|We can proceed provided the controls are independently tested.|ดำเนินต่อได้หากการควบคุมผ่านการทดสอบอิสระ|115|provided=โดยมีเงื่อนไข;proceed=ดำเนินต่อ;independently=อย่างอิสระ
"""),
    ("B2", 27, "Passive Causative และ Reporting", """
เลือก passive เพื่อเน้นกระบวนการ|advanced passive|[Object] is being/has been/had been [participle].|The accounts had been frozen before the review began.|บัญชีถูกระงับก่อนการตรวจเริ่ม|43,44|frozen=ถูกระงับ;account=บัญชี;review=การตรวจ
รายงานความเชื่อทั่วไป|reporting passive|It is said that [clause]. [Subject] is believed to [verb].|It is believed that the change will reduce errors.|เชื่อกันว่าการเปลี่ยนนี้จะลดข้อผิดพลาด|45|is believed=เชื่อกันว่า;reduce=ลด;error=ข้อผิดพลาด
ใช้ causative กับบริการ|have/get something done|[Subject] have/get [object] [participle].|We had the figures independently verified.|เราให้บุคคลอิสระตรวจยืนยันตัวเลข|46|verified=ตรวจยืนยัน;figure=ตัวเลข;independently=โดยอิสระ
รายงานสารโดยรักษาระดับความแน่นอน|reported speech with stance|[Person] claimed/confirmed/suggested that [clause].|The vendor confirmed that no records had been lost.|ผู้ขายยืนยันว่าไม่มีระเบียนสูญหาย|47,48|confirmed=ยืนยัน;record=ระเบียน;lost=สูญหาย
ฝังคำถามอย่างสุภาพ|embedded question|Could you clarify whether/why/how [clause]?|Could you clarify why the threshold was changed?|ช่วยชี้แจงว่าเหตุใดเกณฑ์จึงถูกเปลี่ยน|50|clarify=ชี้แจง;threshold=เกณฑ์;embedded=ที่ฝังในประโยค
"""),
    ("B2", 28, "Clause reduction และ Verb patterns", """
ลด relative clause ด้วย ing|present participle clause|[Noun] [verb-ing phrase] [main verb].|Customers using the old app must update it.|ลูกค้าที่ใช้แอปเก่าต้องอัปเดต|97|using=ที่กำลังใช้;update=อัปเดต;participle=รูปกริยาขยาย
ลด relative clause ด้วย ed|past participle clause|[Noun] [past-participle phrase] [main verb].|Payments flagged as unusual require review.|รายการชำระที่ถูกระบุว่าผิดปกติต้องตรวจ|97|flagged=ถูกระบุ;unusual=ผิดปกติ;require=จำเป็นต้อง
ใช้ ing clause บอกเหตุประกอบ|supplementary -ing clause|[Main clause], [verb-ing] [result/context].|The supplier withdrew, leaving us without a backup.|ผู้ขายถอนตัว ทำให้เราไม่มีตัวสำรอง|68|withdrew=ถอนตัว;leaving=ทำให้เหลือ;backup=ตัวสำรอง
เลือก gerund infinitive ตามเจตนา|advanced gerund/infinitive contrast|regret/remember/try/need [verb-ing or to-verb]|We regret to announce the closure; I regret approving it.|เสียใจที่ต้องประกาศปิด และฉันเสียดายที่เคยอนุมัติ|56,57,58|regret=เสียใจหรือเสียดาย;closure=การปิด;approving=การอนุมัติ
ใช้ preposition กับ ing|preposition + gerund|[Phrase/preposition] [verb-ing].|We succeeded in reducing errors without increasing delays.|เราลดข้อผิดพลาดสำเร็จโดยไม่เพิ่มความล่าช้า|60,61,62,63|succeeded in=ทำสำเร็จ;reducing=ลด;delay=ความล่าช้า
"""),
    ("B2", 29, "Article Preposition Comparison และ Discourse", """
เลือก article ในบริบทนามธรรม|advanced articles|a/the/zero article with institutions and abstractions|The team went to court after the dispute, but the court building was closed.|ทีมขึ้นศาลหลังข้อพิพาท แต่อาคารศาลปิด|72,73,74,75,76,77,78|court=ศาล;dispute=ข้อพิพาท;building=อาคาร
ควบคุมปริมาณอย่างแม่น|advanced quantifiers|few/a few; little/a little; most/most of|Few requests failed, but a few required manual repair.|แทบไม่มีคำขอล้มเหลว แต่มีบางรายการต้องแก้ด้วยคน|87,88,89,90,91|few=แทบไม่มี;a few=มีบ้าง;manual=ด้วยคน
ขยายการเปรียบเทียบ|qualified comparison|far/slightly/no [comparative]; not nearly as ... as|The revised plan is far safer but slightly more expensive.|แผนใหม่ปลอดภัยกว่ามากแต่แพงขึ้นเล็กน้อย|105,106,107,108|far safer=ปลอดภัยกว่ามาก;slightly=เล็กน้อย;revised=ฉบับปรับ
เลือก preposition ตามความสัมพันธ์|advanced preposition choice|noun/adjective/verb + preposition|The increase in costs resulted from changes to the contract.|การเพิ่มของต้นทุนเกิดจากการเปลี่ยนสัญญา|121,122,123,124,125,126,127,128,129,130,131,132,133,134,135,136|increase in=การเพิ่มของ;resulted from=เกิดจาก;contract=สัญญา
เชื่อมข้อโต้แย้งซับซ้อน|discourse linking|although/despite/whereas/unless/by the time/in case|Although demand rose, profit fell, whereas cash flow improved; we'll retain reserves in case conditions worsen.|แม้ความต้องการเพิ่ม กำไรลด แต่กระแสเงินสดดีขึ้น เราจะเก็บเงินสำรองเผื่อสถานการณ์แย่ลง|113,114,115,116,117,118,119,120|whereas=ในขณะที่;reserve=เงินสำรอง;worsen=แย่ลง
"""),
]


LEVEL_SLUG = {"Pre-A1": "prea1", "A1": "a1", "A2": "a2", "B1": "b1", "B2": "b2"}
ORIGINAL_FOUNDATION_IDS = {
    "grammar-prea1-u05-l01", "grammar-prea1-u05-l02", "grammar-prea1-u05-l03",
    "grammar-prea1-u05-l04", "grammar-prea1-u05-l05", "grammar-prea1-u06-l04",
    "grammar-prea1-u09-l05",
}
COACHING = {
    "Pre-A1": "ฟังทีละวลีสั้น พูดตาม แล้วเปลี่ยนข้อมูลหนึ่งจุด เน้นให้ผู้ฟังเข้าใจ",
    "A1": "พูดประโยคเต็มช้าๆ แล้วลองใหม่โดยไม่อ่าน เลือกคำที่ตรงกับเรื่องจริงของคุณ",
    "A2": "เตรียมใจความหนึ่งประโยค พูดให้ครบ แล้วตอบคำถามต่อยอดด้วยรายละเอียดใหม่",
    "B1": "จัดคำตอบเป็นใจความ เหตุผล และรายละเอียด ตรวจว่ารูปภาษาสอดคล้องกับเวลาและเจตนา",
    "B2": "เลือกโครงสร้างตามน้ำหนักความหมาย ระดับความแน่นอน และผู้ฟัง แล้วป้องกันข้อสรุปด้วยหลักฐาน",
}


def parse_vocab(spec: str, example: str) -> list[dict[str, str]]:
    words = []
    for item in spec.split(";"):
        term, meaning = item.split("=", 1)
        words.append({"term": term, "meaning": meaning, "example": example})
    if not 2 <= len(words) <= 4:
        raise ValueError(f"vocabulary must have 2-4 items: {spec}")
    return words


def slots_for(pattern: str) -> list[dict[str, str]]:
    names = []
    for raw in re.findall(r"\[([^]]+)\]", pattern):
        for name in raw.split("/"):
            name = name.strip()
            if name and name not in names:
                names.append(name)
    return [
        {"name": name, "explanation": f"แทน {name} ด้วยข้อมูลจริงหรือข้อมูลสมมติที่เหมาะกับสถานการณ์"}
        for name in names
    ]


def shared_fields(level: str, title: str, pattern: str, example: str,
                  meaning: str, vocab: str, unit_title: str) -> dict:
    objective = f"ใช้ภาษาเพื่อ{title}ในสถานการณ์ใหม่ และตอบคำถามต่อยอดได้"
    rubric = f"คำตอบสื่อว่า “{title}” ได้เหมาะกับบริบท ใช้รูปภาษาเป้าหมายได้ และเปลี่ยนรายละเอียดจากตัวอย่าง"
    return {
        "title": title,
        "objective": objective,
        "pattern": pattern,
        "example": example,
        "meaning": meaning,
        "explanation": (
            f"บทนี้ฝึก{title} ใช้โครง “{pattern}” เป็นเครื่องมือสื่อสาร ไม่ใช่ประโยคที่ต้องท่องตรงตัว "
            "เลือกคำให้ตรงเวลา ผู้ฟัง และผลที่ต้องการ แล้วฟังว่าคู่สนทนาต้องการข้อมูลใดเพิ่ม"
        ),
        "vocabulary": parse_vocab(vocab, example),
        "drills": [
            {
                "kind": "noticing",
                "prompt": f"อ่านสถานการณ์เรื่อง “{unit_title}” แล้วชี้ว่าส่วนใดของตัวอย่างทำหน้าที่{title}",
                "target": f"อธิบายหน้าที่ของรูปภาษาเป้าหมายได้; ไม่ต้องใช้ถ้อยคำตรงเฉลย",
            },
            {
                "kind": "guided_response",
                "prompt": f"สมมติว่ารายละเอียดในตัวอย่างเปลี่ยนไปหนึ่งอย่าง พูดประโยคใหม่เพื่อ{title}",
                "target": rubric,
            },
            {
                "kind": "situational_response",
                "prompt": f"คู่สนทนาในสถานการณ์ “{unit_title}” ต้องการข้อมูลเพิ่มหนึ่งข้อ ตอบให้ครบและเป็นธรรมชาติ",
                "target": rubric + "; มีรายละเอียดสนับสนุนอย่างน้อยหนึ่งอย่าง",
            },
            {
                "kind": "repair_and_transfer",
                "prompt": f"ถ้าคู่สนทนาเข้าใจผิด ให้แก้ความเข้าใจ แล้วใช้เป้าหมาย “{title}” ในสถานการณ์งานหรือชีวิตประจำวันที่ต่างจากตัวอย่าง",
                "target": rubric + "; แก้ความเข้าใจอย่างสุภาพและโอนไปใช้กับบริบทใหม่",
                "time_goal_seconds": 45 if level in {"B1", "B2"} else 30,
            },
        ],
        "conversation_prompt": (
            f"สร้างบทสนทนาใหม่เกี่ยวกับ {unit_title} ให้ผู้เรียนฝึก{title}ด้วยข้อมูลของตนเอง "
            f"ปรับความยากระดับ {level} ถามต่อให้ผู้เรียนอธิบายหรือยืนยันหนึ่งครั้ง "
            "จากนั้นเปลี่ยนเป็นอีกบริบทหนึ่ง ห้ามถือว่าการพูดซ้ำตัวอย่างคือทำได้สำเร็จ"
        ),
        "acceptance": [
            f"สื่อเป้าหมายเรื่อง{title}ได้โดยไม่อ่านตัวอย่าง",
            "ใช้โครงสร้างและคำศัพท์ได้เหมาะกับความหมาย แม้ถ้อยคำไม่ตรงตัวอย่าง",
            "ตอบสถานการณ์ใหม่สองครั้ง พร้อมรายละเอียดหรือเหตุผลที่คู่สนทนาเข้าใจได้",
        ],
        "version": VERSION,
        "assessment": False,
        "coaching_notes": COACHING[level],
        "slots": slots_for(pattern),
    }


def build() -> list[dict]:
    lessons = []
    ordinal = 101
    for level, unit, unit_title, block in PRACTICAL_UNITS:
        for lesson_no, (title, pattern, example, meaning, vocab) in enumerate(rows(block), 1):
            lesson = {
                "id": f"{LEVEL_SLUG[level]}-practical-u{unit:02}-l{lesson_no:02}",
                "ordinal": ordinal,
                "level": level,
                "unit": unit,
                "unit_title": unit_title,
                "grammar_focus": "",
                "source_units": [],
            }
            lesson.update(shared_fields(level, title, pattern, example, meaning, vocab, unit_title))
            lessons.append(lesson)
            ordinal += 1

    for level, unit, unit_title, block in GRAMMAR_UNITS:
        for lesson_no, (title, focus, pattern, example, meaning, sources, vocab) in enumerate(grammar_rows(block), 1):
            lesson = {
                "id": f"grammar-{LEVEL_SLUG[level]}-u{unit:02}-l{lesson_no:02}",
                "ordinal": ordinal,
                "level": level,
                "unit": unit,
                "unit_title": unit_title,
                "grammar_focus": focus,
                "source_units": sources,
            }
            lesson.update(shared_fields(level, title, pattern, example, meaning, vocab, unit_title))
            lesson["explanation"] = (
                f"เป้าหมายคือ {focus}: {meaning} สังเกตรูปใน “{example}” แล้วเลือกใช้ตามความหมายจริง "
                "ภาษาไทยอาจไม่แสดงเวลา article หรือรูปกริยาแบบเดียวกับอังกฤษ จึงให้ตัดสินจากบริบทก่อนผันรูป"
            )
            if "perfect continuous" in focus and "future" in focus:
                lesson["explanation"] += (
                    " ในสี่รูปอนาคตนี้ให้ใช้ future simple, continuous และ perfect เป็นหลัก; "
                    "future perfect continuous เป็นรูปที่พบน้อยกว่า ให้รู้จักและใช้เมื่อจำเป็นต้องเน้นระยะเวลาจนถึงจุดหนึ่งในอนาคต"
                )
            lessons.append(lesson)
            ordinal += 1
    return lessons


def validate(lessons: list[dict]) -> None:
    required = {
        "id", "ordinal", "level", "unit", "unit_title", "title", "objective", "pattern",
        "example", "meaning", "explanation", "vocabulary", "drills", "conversation_prompt",
        "acceptance", "version", "assessment", "coaching_notes", "slots", "grammar_focus",
        "source_units",
    }
    assert len(lessons) == 425
    assert len({x["id"] for x in lessons}) == 425
    assert [x["ordinal"] for x in lessons] == list(range(101, 526))
    counts = {(level, kind): 0 for level in LEVEL_SLUG for kind in ("practical", "grammar")}
    for lesson in lessons:
        missing = required - lesson.keys()
        assert not missing, (lesson["id"], missing)
        assert all(lesson[key] not in (None, "") for key in required - {"grammar_focus", "slots", "source_units"})
        assert len(lesson["vocabulary"]) in (2, 3, 4)
        assert len(lesson["drills"]) == 4
        assert len(lesson["acceptance"]) >= 3
        kind = "grammar" if lesson["id"].startswith("grammar-") else "practical"
        counts[(lesson["level"], kind)] += 1
        if kind == "grammar":
            assert lesson["grammar_focus"]
            assert lesson["source_units"] or lesson["id"] in ORIGINAL_FOUNDATION_IDS
            assert all(isinstance(x, int) and 1 <= x <= 145 for x in lesson["source_units"])
        else:
            assert lesson["level"] in {"A2", "B1", "B2"}
    for level in ("A2", "B1", "B2"):
        assert counts[(level, "practical")] == 100
    for level in LEVEL_SLUG:
        assert counts[(level, "grammar")] == 25
    assert sum(counts[(level, "practical")] for level in LEVEL_SLUG) == 300


def main() -> None:
    lessons = build()
    validate(lessons)
    OUT.write_text(json.dumps(lessons, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"wrote {len(lessons)} lessons to {OUT.relative_to(ROOT)}")


if __name__ == "__main__":
    main()
