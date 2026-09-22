#!/usr/bin/env python3
"""Generate the versioned Learn Ebook course from manifest unit titles only.

The generator reads only .ebook/manifest.json metadata (unit titles) and
never reads extracted lesson, exercise, answer, or page-image content.
"""

import json, re
from collections import Counter
from pathlib import Path

root = Path(__file__).resolve().parents[1]
manifest = json.loads((root/'.ebook/manifest.json').read_text())
titles = [u['title'] for u in manifest['units']]
assert len(titles) == 145

topics = [
    dict(person='Mina', obj='the client dashboard', obj_th='แดชบอร์ดของลูกค้า', obj2='the handoff note', obj2_th='บันทึกส่งต่องาน', verb='review', ing='reviewing', past='reviewed', pp='reviewed', verb_th='ทบทวน', place='the meeting room', place_th='ห้องประชุม', time='before lunch', time_th='ก่อนอาหารกลางวัน', scenario='a client handoff', scenario_th='การส่งต่องานให้ลูกค้า'),
    dict(person='Arun', obj='the release calendar', obj_th='ปฏิทินการปล่อยงาน', obj2='the sprint plan', obj2_th='แผนสปรินต์', verb='update', ing='updating', past='updated', pp='updated', verb_th='อัปเดต', place='the project room', place_th='ห้องโครงการ', time='every morning', time_th='ทุกเช้า', scenario='a software release', scenario_th='การปล่อยซอฟต์แวร์'),
    dict(person='Nora', obj='the supplier invoice', obj_th='ใบแจ้งหนี้ของผู้ขาย', obj2='the purchase order', obj2_th='ใบสั่งซื้อ', verb='check', ing='checking', past='checked', pp='checked', verb_th='ตรวจสอบ', place='the finance desk', place_th='โต๊ะการเงิน', time='after the call', time_th='หลังการโทร', scenario='a supplier payment', scenario_th='การจ่ายเงินให้ผู้ขาย'),
    dict(person='Leo', obj='the support queue', obj_th='คิวฝ่ายช่วยเหลือ', obj2='the urgent ticket', obj2_th='ทิกเก็ตเร่งด่วน', verb='handle', ing='handling', past='handled', pp='handled', verb_th='จัดการ', place='the support desk', place_th='โต๊ะช่วยเหลือ', time='this afternoon', time_th='บ่ายนี้', scenario='a customer support shift', scenario_th='กะงานช่วยเหลือลูกค้า'),
    dict(person='Pim', obj='the presentation slides', obj_th='สไลด์นำเสนอ', obj2='the speaker notes', obj2_th='โน้ตผู้บรรยาย', verb='prepare', ing='preparing', past='prepared', pp='prepared', verb_th='เตรียม', place='the training room', place_th='ห้องอบรม', time='for tomorrow', time_th='สำหรับพรุ่งนี้', scenario='a project presentation', scenario_th='การนำเสนอโครงการ'),
    dict(person='Kai', obj='the service contract', obj_th='สัญญาบริการ', obj2='the renewal terms', obj2_th='เงื่อนไขต่อสัญญา', verb='negotiate', ing='negotiating', past='negotiated', pp='negotiated', verb_th='เจรจา', place='the client office', place_th='สำนักงานลูกค้า', time='on Friday', time_th='วันศุกร์', scenario='a contract renewal', scenario_th='การต่อสัญญา'),
    dict(person='Fah', obj='the backup file', obj_th='ไฟล์สำรอง', obj2='the test result', obj2_th='ผลทดสอบ', verb='restore', ing='restoring', past='restored', pp='restored', verb_th='กู้คืน', place='the lab', place_th='ห้องทดลอง', time='before the demo', time_th='ก่อนการสาธิต', scenario='a system recovery', scenario_th='การกู้คืนระบบ'),
    dict(person='Ben', obj='the travel booking', obj_th='การจองเดินทาง', obj2='the arrival time', obj2_th='เวลามาถึง', verb='confirm', ing='confirming', past='confirmed', pp='confirmed', verb_th='ยืนยัน', place='the station', place_th='สถานี', time='before departure', time_th='ก่อนออกเดินทาง', scenario='a work trip', scenario_th='การเดินทางไปทำงาน'),
    dict(person='Suda', obj='the interview plan', obj_th='แผนสัมภาษณ์', obj2='the candidate profile', obj2_th='ประวัติผู้สมัคร', verb='arrange', ing='arranging', past='arranged', pp='arranged', verb_th='จัดเตรียม', place='the hiring panel', place_th='คณะสัมภาษณ์', time='next week', time_th='สัปดาห์หน้า', scenario='a job interview', scenario_th='การสัมภาษณ์งาน'),
    dict(person='Tom', obj='the delivery route', obj_th='เส้นทางส่งของ', obj2='the warehouse list', obj2_th='รายการคลังสินค้า', verb='track', ing='tracking', past='tracked', pp='tracked', verb_th='ติดตาม', place='the loading bay', place_th='ลานขนส่ง', time='at six o’clock', time_th='หกโมง', scenario='a delivery update', scenario_th='การอัปเดตการส่งของ'),
    dict(person='May', obj='the weekly budget', obj_th='งบประมาณรายสัปดาห์', obj2='the expense report', obj2_th='รายงานค่าใช้จ่าย', verb='compare', ing='comparing', past='compared', pp='compared', verb_th='เปรียบเทียบ', place='the finance meeting', place_th='การประชุมการเงิน', time='at month-end', time_th='สิ้นเดือน', scenario='a budget review', scenario_th='การทบทวนงบประมาณ'),
    dict(person='Oat', obj='the recipe card', obj_th='การ์ดสูตรอาหาร', obj2='the shopping list', obj2_th='รายการซื้อของ', verb='write', ing='writing', past='wrote', pp='written', verb_th='เขียน', place='the kitchen', place_th='ห้องครัว', time='after work', time_th='หลังเลิกงาน', scenario='a shared meal', scenario_th='มื้ออาหารร่วมกัน'),
    dict(person='Yui', obj='the clinic appointment', obj_th='นัดคลินิก', obj2='the medicine label', obj2_th='ฉลากยา', verb='book', ing='booking', past='booked', pp='booked', verb_th='จอง', place='the reception desk', place_th='เคาน์เตอร์ต้อนรับ', time='today', time_th='วันนี้', scenario='a health appointment', scenario_th='นัดหมายด้านสุขภาพ'),
    dict(person='Dan', obj='the apartment key', obj_th='กุญแจอพาร์ตเมนต์', obj2='the repair request', obj2_th='คำขอซ่อม', verb='collect', ing='collecting', past='collected', pp='collected', verb_th='รับ', place='the building entrance', place_th='ทางเข้าอาคาร', time='after the inspection', time_th='หลังการตรวจ', scenario='a home repair', scenario_th='การซ่อมบ้าน'),
    dict(person='Lina', obj='the training exercise', obj_th='แบบฝึกอบรม', obj2='the lesson note', obj2_th='โน้ตบทเรียน', verb='practice', ing='practicing', past='practiced', pp='practiced', verb_th='ฝึก', place='the classroom', place_th='ห้องเรียน', time='in the evening', time_th='ตอนเย็น', scenario='a professional course', scenario_th='หลักสูตรพัฒนางาน'),
    dict(person='Joe', obj='the campaign message', obj_th='ข้อความแคมเปญ', obj2='the community poster', obj2_th='โปสเตอร์ชุมชน', verb='share', ing='sharing', past='shared', pp='shared', verb_th='แบ่งปัน', place='the community center', place_th='ศูนย์ชุมชน', time='this weekend', time_th='สุดสัปดาห์นี้', scenario='a community campaign', scenario_th='แคมเปญชุมชน'),
    dict(person='Rin', obj='the project milestone', obj_th='หมุดหมายโครงการ', obj2='the risk register', obj2_th='ทะเบียนความเสี่ยง', verb='report', ing='reporting', past='reported', pp='reported', verb_th='รายงาน', place='the stand-up meeting', place_th='การประชุมสั้น', time='at nine', time_th='เก้าโมง', scenario='a project status update', scenario_th='การอัปเดตสถานะโครงการ'),
    dict(person='Vee', obj='the access request', obj_th='คำขอสิทธิ์เข้าใช้', obj2='the security notice', obj2_th='ประกาศความปลอดภัย', verb='approve', ing='approving', past='approved', pp='approved', verb_th='อนุมัติ', place='the admin portal', place_th='พอร์ทัลผู้ดูแล', time='before the deadline', time_th='ก่อนกำหนดส่ง', scenario='an access request', scenario_th='คำขอสิทธิ์เข้าใช้'),
    dict(person='Gai', obj='the feedback form', obj_th='แบบฟอร์มความคิดเห็น', obj2='the revised draft', obj2_th='ร่างที่แก้ไข', verb='complete', ing='completing', past='completed', pp='completed', verb_th='กรอกให้เสร็จ', place='the review call', place_th='การคุยทบทวน', time='by Friday', time_th='ภายในวันศุกร์', scenario='a feedback cycle', scenario_th='รอบรับความคิดเห็น'),
    dict(person='Eve', obj='the airport map', obj_th='แผนที่สนามบิน', obj2='the boarding pass', obj2_th='บัตรขึ้นเครื่อง', verb='find', ing='finding', past='found', pp='found', verb_th='หา', place='the departure hall', place_th='โถงขาออก', time='before boarding', time_th='ก่อนขึ้นเครื่อง', scenario='an airport transfer', scenario_th='การเปลี่ยนเที่ยวบินที่สนามบิน'),
]

def topic(i):
    return topics[(i-1) % len(topics)]

def bare(s):
    return re.sub(r'^(?:a|an|the)\s+', '', s, flags=re.I)

def cap(s):
    return s[:1].upper() + s[1:]

def plural_phrase(s):
    words = s.split()
    if not words:
        return s
    last = words[-1]
    if last.endswith(('s', 'x', 'z', 'ch', 'sh')):
        words[-1] = last + 'es'
    elif last.endswith('y') and len(last) > 1 and last[-2] not in 'aeiou':
        words[-1] = last[:-1] + 'ies'
    else:
        words[-1] = last + 's'
    return ' '.join(words)

def E(en, th):
    for name in ('Mina','Arun','Nora','Leo','Pim','Kai','Fah','Ben','Suda','Tom','May','Oat','Yui','Dan','Lina','Joe','Rin','Vee','Gai','Eve'):
        th = re.sub(rf'{name}(?=[\u0e00-\u0e7f])', name + ' ', th)
    return {'en': en, 'th': th}

EXAMPLE_TEMPLATES = [
    ("I am {ing} {o} before lunch.", "{p} is {ing} {o2} while I take notes.", "We are {ing} {o} and {o2} together."),
    ("I {v} {o} every morning.", "{p} {third} {o2} on Mondays.", "Our team {third} {o} every afternoon."),
    ("I am {ing} {o} now, but I {v} it every Friday.", "{p} is {ing} {o2} today, but they {third} the log daily.", "We are {ing} {o} this week, and we {v} {o2} every quarter."),
    ("I am {ing} {o} this week, but I usually {v} it on Fridays.", "{p} is {ing} {o2} temporarily, but they {third} the process permanently.", "We are {ing} {o} for this project, but we {v} {o2} as a regular task."),
    ("I {past} {o} yesterday.", "{p} {past} {o2} after the call.", "We {past} both {o} and {o2} before lunch."),
    ("I was {ing} {o} when the phone rang.", "{p} was {ing} {o2} when the client called.", "We were {ing} {o} when the meeting started."),
    ("I have {pp} {o}, so it is ready now.", "{p} has {pp} {o2}, and the team can continue.", "We have {pp} both files, so the handoff is ready."),
    ("I have {pp} {o} twice this week.", "{p} has {pp} {o2} before.", "We have {pp} this process several times this month."),
    ("I have been {ing} {o} since nine.", "{p} has been {ing} {o2} for two hours.", "We have been {ing} the handoff plan all morning."),
    ("I have been {ing} {o}, so I have {pp} the first section.", "{p} has been {ing} {o2}, and {p} has {pp} one issue.", "We have been {ing} {o}, and we have {pp} {o2}."),
    ("How long have you been {ing} {o}?", "How long has {p} been {ing} {o2}?", "How long have they been {ing} the handoff plan?"),
    ("I have worked with {o} for three years.", "{p} has maintained {o2} since January.", "We have used this process since the last audit."),
    ("I have {pp} {o}; I {past} it yesterday.", "{p} has {pp} {o2}; {p} {past} it last week.", "We have finished {o}, but we sent {o2} on Monday."),
    ("I have {pp} {o}, so I sent the summary at noon.", "{p} has {pp} {o2}; {p} sent it at three.", "We have confirmed the handoff, and we notified the client at four."),
    ("I had {pp} {o} before the meeting started.", "{p} had {pp} {o2} before the client called.", "We had checked the release plan before the demo began."),
    ("I had been {ing} {o} for an hour before lunch.", "{p} had been {ing} {o2} before the client called.", "We had been preparing the handoff for two hours before the meeting began."),
    ("I have {o} on my laptop.", "{p} has got {o2}.", "We have the files, and the team has got the access."),
    ("I used to {v} {o} on Fridays.", "{p} used to {v} {o2} by hand.", "We did not use to track the handoff online."),
    ("I am {ing} {o} with the client tomorrow.", "{p} {third} {o2} with Arun on Friday.", "We are meeting about the handoff next Monday."),
    ("I am going to {v} {o} after the call.", "{p} is going to {v} {o2} before Friday.", "We are going to change the plan because the data is late."),
    ("I will {v} {o} tonight; shall we discuss {o2} tomorrow?", "We will {v} {o2}; shall we send it to the client?", "I will check {o}; shall we call Arun?"),
    ("I will {v} {o} for you.", "{p} will send {o2} by noon.", "We will help with the release plan, I promise."),
    ("I will {v} {o} now; I am going to send {o2} later.", "{p} will call the client; they are going to update the release calendar first.", "We will change the schedule; we are going to explain the reason."),
    ("At ten tomorrow, I will be {ing} {o}; by noon, I will have finished {o2}.", "{p} will be presenting the release plan at three; by Friday, {p} will have completed the report.", "We will be preparing the handoff this afternoon; by evening, we will have sent the summary."),
    ("When I {v} {o}, I will call the client.", "If {p} {third} {o2}, we will send the release email.", "When we finish {o}, we will share {o2}."),
    ("I can {v} {o} today.", "Could {p} {v} {o2}?", "We will be able to finish the handoff by Friday."),
    ("I could {v} {o} when I was an intern.", "{p} could have {pp} {o2}, but {p} missed the deadline.", "We could fix the issue yesterday, but we could have asked for help earlier."),
    ("We must {v} {o} before the call; {o2} can't be final yet.", "{p} must check {o2}; it can't be complete without the client signature.", "The client must be online; the handoff can't be sent yet."),
    ("I may {v} {o} after lunch.", "{p} might {v} {o2} today.", "We may need to delay the handoff if the client is late."),
    ("I may have {pp} {o} too quickly.", "{p} might have missed {o2}.", "We may have sent the wrong release file."),
    ("I have to {v} {o} before lunch.", "{p} must {v} {o2} today.", "We have to confirm the client handoff by Friday."),
    ("You mustn't share {o} outside the team; you needn't print it.", "{p} mustn't delete {o2}; {p} needn't copy it again.", "We mustn't miss the deadline; we needn't stay late if the file is ready."),
    ("You should {v} {o} before the call.", "{p} should {v} {o2} today.", "We should confirm the handoff with the client."),
    ("The client should receive {o} by noon, but it is still missing.", "{p} should check {o2} before sending it.", "We should know the handoff status by now."),
    ("You'd better {v} {o}; it's time we sent {o2}.", "{p} had better update {o2}; it's time the client knew.", "We'd better confirm the release; it's time we called the client."),
    ("I would review {o} every Friday when I worked there.", "{p} would update {o2} after each meeting.", "I would appreciate it if you would confirm the handoff."),
    ("Could you {v} {o}? Would you like {o2}?", "Would {p} update {o2}? Could {p} send it to the client?", "Could we confirm the handoff? Would you like to join the call?"),
    ("If I {v} {o}, I will send {o2}.", "If {p} updated {o2}, {p} would help the client.", "If we have time, we will check the release; if we had more time, we would test it again."),
    ("If I knew the {bo} password, I would review it; I wish I knew it.", "If {p} had {o2}, they would update it; they wish they had it.", "If we knew the release date, we would plan the handoff; we wish we knew it."),
    ("If I had {pp} {o}, I would have found the issue; I wish I had {pp} it.", "If {p} had {pp} {o2}, they would have helped the client; they wish they had {pp} it.", "If we had checked the release plan, we would have avoided the delay; we wish we had checked it."),
    ("I wish I knew the {bo} password.", "{p} wishes they had updated {o2}.", "We wish the client would reply today."),
    ("{O} is reviewed every morning.", "{O2} was updated after the call.", "The release report is checked before the demo."),
    ("{O} must be reviewed before lunch.", "{O2} has been updated.", "The release plan is being checked by the team."),
    ("It is reported that {o} is ready.", "It was announced that {o2} had changed.", "{p} is reported to have approved the release."),
    ("{O} are supposed to be ready by noon.", "{p} is supposed to update {o2} today.", "We are supposed to confirm the handoff before Friday."),
    ("I had {o} reviewed by a specialist.", "{p} had {o2} updated before the call.", "We will have the release report checked tomorrow."),
    ("The reviewer said that they had reviewed {o}.", "Arun said that he would update {o2}.", "The client said that the handoff was ready."),
    ("{p} told me to {v} {o}.", "The manager asked Arun to update {o2}.", "I told the team not to share the release plan."),
    ("What did you {v} for {o}?", "Has {p} updated {o2}?", "Will we confirm the handoff before Friday?"),
    ("{p} asked where {o} was.", "I wondered whether Arun had updated {o2}.", "The client asked when we would send the release plan."),
    ("I think {o} is ready, and I think so.", "I hope {o2} is ready; I hope so.", "I don't think the handoff is late; I don't think so."),
    ("{O} is ready, isn't it?", "{p} updated {o2}, didn't they?", "We can confirm the handoff, can't we?"),
    ("I enjoy {ing} {o}.", "{p} stopped updating {o2}.", "We avoid sharing the release plan outside the team."),
    ("I decided to {v} {o}.", "{p} remembered to update {o2}.", "We agreed to confirm the handoff."),
    ("I want you to {v} {o}.", "{p} asked Arun to update {o2}.", "We need the client to confirm the handoff."),
    ("I remember {ing} {o} yesterday.", "{p} remembered to update {o2}.", "We regret sending the old handoff."),
    ("I tried {ing} {o} in a new browser.", "{p} needs to update {o2}.", "We helped the client confirm the handoff."),
    ("I like {ing} {o}.", "{p} loves updating {o2}.", "We would like to confirm the handoff today."),
    ("I prefer {ing} {o} in the morning.", "{p} prefers to update {o2} after lunch.", "We would rather confirm the handoff by phone."),
    ("Before {ing} {o}, I check the agenda.", "{p} left after updating {o2}.", "We talked about confirming the handoff."),
    ("I am used to {o} now.", "{p} is getting used to updating {o2}.", "We are used to working with the handoff process."),
    ("We succeeded in reviewing {o} on time.", "{p} insisted on updating {o2} personally.", "They succeeded in confirming the handoff."),
    ("There is no point in reviewing {o} again.", "It is worth updating {o2} now.", "There is no point in delaying the handoff; it is worth confirming it."),
    ("I opened {o} to review the data.", "{p} used {o2} for the client.", "We updated the release plan so that the team could act."),
    ("It is easy to review {o}.", "It is ready to send {o2}.", "We are pleased to confirm the handoff."),
    ("I am responsible for reviewing {o}.", "{p} is worried about updating {o2}.", "We are interested in confirming the handoff."),
    ("I saw {p} review {o}.", "We heard Arun updating {o2}.", "The client saw us confirm the handoff."),
    ("Reviewing {o}, I found a missing figure.", "Updating {o2}, {p} called the client.", "Confirming the handoff, we closed the ticket."),
    ("We need some information about {o}.", "There are a few notes in {o2}.", "We need several updates before the release."),
    ("We need some information from {o}.", "How much feedback is in {o2}?", "Please send me a piece of information about the release."),
    ("I need a copy of {o}.", "{p} collected some notes for {o2}.", "We need some information before the release."),
    ("I opened a {bo} and reviewed the {bo} later.", "{p} wrote a {bo2}; the {bo2} is ready.", "We saw a release plan, and the plan looked useful."),
    ("{O} is on the desk.", "Please open {o2} we discussed.", "The sun came through the office window during the handoff."),
    ("{p} is at work reviewing {o}.", "After work, {p} went to the office to update {o2}.", "The office is near the station."),
    ("Clients use {o} at work.", "The clients in this handoff need {o2}.", "Reports help teams, but the reports from today need review."),
    ("The telephone is beside {o2}.", "The tiger is a useful example when discussing {o}.", "The rich should support the release team."),
    ("{p} reviewed {o} in Bangkok.", "Arun updated {o2} in Thailand.", "The Pacific route affected the release schedule."),
    ("The World Bank published a report about {o}.", "Google shared {o2} with the team.", "I read the Financial Times before the release call."),
    ("One {bo} has three tabs.", "The {bo2} lists two people responsible for the task.", "{p} updated the {bop} and the notes before lunch."),
    ("The {bo} shows our sales report.", "{p} saved the {bo2} in the project folder.", "We reviewed the release schedule after lunch."),
    ("The dashboard of the client is ready.", "The title of {o2} is clear.", "The schedule of the release changed today."),
    ("I reviewed {o} myself.", "I sent {o2} to myself.", "We prepared ourselves for the client handoff."),
    ("I reviewed {o} on my own.", "{p} updated {o2} by herself.", "We completed the client handoff on our own."),
    ("There is a {bo} on the screen; it is clear.", "There are two {bo2p} in the folder; it is time to review them.", "There is a release delay; it is frustrating."),
    ("Would you like some help with {o}?", "Do you have any questions about {o2}?", "We don't have any updates for the release."),
    ("There is no update on {o}.", "None of the options in {o2} are final.", "Nobody approved the release plan."),
    ("We have many reports and much information about {o}.", "There is little time but a lot of work on the handoff.", "We have a few {bo2p} and plenty of support for the release."),
    ("All of the {bo} data is current.", "Most of the {bo2p} are ready.", "None of the release files is complete."),
    ("Either {bo} is suitable for the demo.", "Either of the {bo2p} is acceptable.", "Either plan will work for the release."),
    ("All the {bo} data is ready.", "Every {bo2} has an owner.", "We reviewed the whole release plan."),
    ("Each {bo} has a separate owner.", "Every {bo2} needs a date.", "Each of the release files has a label."),
    ("The analyst who reviews {o} leads the call.", "The {bo2} that {p} updated is ready.", "The release file which we tested is safe."),
    ("The {bo} I reviewed is ready.", "The {bo2} {p} updated is clear.", "The release file we tested passed."),
    ("The analyst whose {bo} I reviewed called me.", "The client whom {p} called approved {o2}.", "The office where we signed the release is nearby."),
    ("{p}, who reviews {o}, leads the call.", "The {bo2}, which Arun updated, is ready.", "Bangkok, where our release team works, is busy."),
    ("The {bo}, which looks simple, contains complex data.", "{p}, who usually updates {o2}, is away today.", "The release plan, which Arun approved, still needs a date."),
    ("The {bo} reviewed yesterday is ready.", "The {bo2} updated by {p} is clear.", "The release file tested by the team passed."),
    ("The {bo} is confusing, so I am confused.", "The {bo2} is reassuring, and {p} is reassured.", "The delayed release is frustrating, and the client is frustrated."),
    ("The new {bo} is clear.", "The detailed {bo2} looks useful.", "The release plan is practical and ready."),
    ("The {bo} is clear, and {p} explains it clearly.", "The {bo2} is complete, and Arun checks it carefully.", "The release plan is useful, and we use it efficiently."),
    ("{p} reviewed {o} well.", "Arun updated {o2} quickly and worked hard.", "We arrived late, but the release was hardly delayed."),
    ("{O} is so clear that everyone can use it.", "It is such a useful document about {o2} that we shared it.", "The release was so successful that the client called."),
    ("{O} is clear enough to share.", "{O2} is too long to read quickly.", "The release is fast enough to meet the deadline."),
    ("{O} is quite clear.", "{O2} is pretty useful for the team.", "The release plan is rather complicated but fairly practical."),
    ("The latest version of {o} is clearer than the old one.", "The latest version of {o2} is more useful than the email.", "This release is faster than the previous release."),
    ("The latest version of {o} is much clearer than the old one.", "The latest version of {o2} is far more useful than the chat message.", "Is the release any faster today?"),
    ("{O} is as clear as the report.", "{O2} is not as long as the old version.", "{p} works as carefully as Arun on the release."),
    ("This is the clearest {bo} in the team.", "{O2} is the most useful document today.", "That was the fastest release this year."),
    ("We reviewed {o} in the meeting room before lunch.", "{p} updated {o2} at the office after the call.", "We sent the release report to the client on Friday."),
    ("{p} usually reviews {o} before lunch.", "Arun has carefully updated {o2}.", "We quickly confirmed the release."),
    ("I still need {o}.", "{p} doesn't need {o2} any more.", "Have we confirmed the release yet? We have already checked it."),
    ("Even the {bo} was reviewed by the client.", "{p} even updated {o2} on Sunday.", "We confirmed even the smallest part of {o}."),
    ("In spite of the delay, we reviewed {o}.", "Despite updating {o2} late, {p} called the client.", "Although the release was risky, we sent it."),
    ("Take a copy of {o} in case the network fails.", "{p} saved {o2} in case the client asks.", "We will carry a backup in case the release stops."),
    ("Unless {o} is ready, we will delay the call.", "As long as {o2} is clear, we can proceed.", "We will release the update provided the tests pass."),
    ("As I reviewed {o}, {p} called.", "As Arun updated {o2}, the client waited.", "As the release was late, we changed the schedule."),
    ("{O} looks like a report.", "{p} works as the owner of {o2}.", "As we discussed, {o} is ready."),
    ("{p} talks as if {p} knows {o}.", "{O2} looks like it is final.", "Arun acts as if the review of {o} has started."),
    ("During the client handoff, we reviewed {o}.", "{p} worked for two hours on {o2}.", "While we tested the release, the client waited."),
    ("I will review {o} by noon.", "{p} will wait until {o2} is ready.", "By the time we release the update, the client will have received the report."),
    ("We review {o} at nine.", "{p} updates {o2} on Monday.", "The release starts in September."),
    ("We finished {o} on time.", "{p} sent {o2} in time for the call.", "At the end of the release, we understood the issue; in the end, the client agreed."),
    ("The file is in the {bo} folder.", "{p} is at the {bo2} desk.", "The note is on the board beside {o2}."),
    ("We met in the {bo} area.", "{p} left {o2} at the {bo} desk.", "The release team works at 20 Market Street."),
    ("I checked {o} on the train.", "{p} reviewed {o2} at the station.", "We talked about the release in the taxi."),
    ("I went to the client office to review {o}.", "{p} arrived at the {bo2} desk.", "We moved into the release room."),
    ("I am at work reviewing {o}.", "{p} is at home with {o2}.", "Arun is on duty during the release."),
    ("I sent {o} by email.", "{O2} was prepared by {p}.", "We must finish the release by Friday."),
    ("The reason for reviewing {o} is clear.", "{p} explained the cause of the delay in {o2}.", "We found a solution to the release problem."),
    ("I am responsible for {o}.", "{p} is interested in {o2}.", "We are ready for the release."),
    ("I am afraid of losing the data in {o}.", "{p} is good at updating {o2}.", "We are satisfied with the release result."),
    ("I listened to the client about the dinner plan while reviewing {o}.", "{p} spoke to the chef about {o2}.", "We waved at the vendor when the delivery arrived."),
    ("We talked about {o}.", "{p} asked for {o2}.", "Arun looked after the release."),
    ("We talked about {o}.", "{p} thought about {o2}.", "Arun reminded us of the release date."),
    ("The client complained of an error in {o}.", "{p} paid for the service in {o2}.", "We depended on the release calendar."),
    ("We believe in the data from {o}.", "{p} put {o2} into the folder.", "Arun agreed with the release team."),
    ("Please look up {o} before the call.", "{p} carried out a review of {o2}.", "We called back after the release."),
    ("Please log in to the account before opening {o}.", "{p} checked out of the hotel after reading {o2}.", "The client signed in, and Arun signed out after the handoff."),
    ("Please find out why {o} is late.", "{p} pointed out an error in {o2}.", "We worked out the release schedule."),
    ("Turn on the device before checking {o}.", "{p} switched off the phone alert after reading {o2}.", "We powered on the release server before the test."),
    ("Please carry on with the meeting after reviewing {o}.", "{p} took off the old label from {o2}.", "We went on after the release delay."),
    ("Please speed up the {bo} refresh.", "{p} wrote down the {bo2} details.", "We cut down the release delay."),
    ("We need to step up checks on {o}.", "{p} used up the budget for {o2}.", "We wrapped up the release meeting."),
    ("Please walk up to the {bo} screen.", "{p} picked up the {bo2}.", "We picked up the release documents."),
    ("Please send {o} back by Friday.", "{p} moved away from the old handoff process.", "I called the client back after the release and reviewed {o2}."),
]

def examples(i):
    if i < 1 or i > len(EXAMPLE_TEMPLATES):
        raise AssertionError(i)
    t = topic(i)
    values = {
        'p': t['person'], 'o': t['obj'], 'o2': t['obj2'],
        'O': cap(t['obj']), 'O2': cap(t['obj2']),
        'bo': bare(t['obj']), 'bo2': bare(t['obj2']),
        'bop': plural_phrase(bare(t['obj'])), 'bo2p': plural_phrase(bare(t['obj2'])),
        'v': t['verb'], 'third': f"{t['verb']}s",
        'ing': t['ing'], 'past': t['past'], 'pp': t['pp'],
        'pl': t['place'], 'tm': t['time'], 'tmt': t['time_th'],
    }
    rendered = [template.format(**values) for template in EXAMPLE_TEMPLATES[i - 1]]
    thai = thai_examples(i, t)
    if len(thai) != 3:
        raise AssertionError(f'lesson {i} needs three Thai example translations')
    return [E(text, thai[index]) for index, text in enumerate(rendered)]

family_names = [
    (4, 'present_tenses', 'ปัจจุบันและความแตกต่างระหว่างชั่วคราวกับกิจวัตร'),
    (18, 'past_present_tenses', 'เวลาในอดีตและผลต่อปัจจุบัน'), (25, 'future', 'แผนและการคาดการณ์'),
    (37, 'modals', 'ความสามารถ หน้าที่ และคำแนะนำ'), (41, 'conditionals', 'เงื่อนไขและความปรารถนา'),
    (46, 'passive_voice', 'ผู้กระทำที่ถูกละไว้และโครงสร้าง passive'),
    (52, 'reported_questions', 'การรายงาน คำถาม และ question tags'), (68, 'verb_patterns', 'รูปกริยาหลังคำกริยาและบุพบท'),
    (79, 'countability_articles', 'คำนามนับได้และ articles'), (91, 'noun_phrases', 'คำนาม ปริมาณ และ pronouns'),
    (99, 'clauses_and_adjectives', 'อนุประโยคและคำคุณศัพท์'), (108, 'adverbs_and_comparison', 'คำวิเศษณ์และการเปรียบเทียบ'),
    (122, 'connectors_and_time', 'คำเชื่อมและเวลา'), (128, 'place_prepositions', 'บุพบทตำแหน่งและการเคลื่อนที่'),
    (136, 'prepositions_in_use', 'บุพบทกับคำนาม คุณศัพท์ และกริยา'), (145, 'phrasal_verbs', 'กริยาวลีในงานและชีวิตประจำวัน'),
]
def family(i):
    for end, key, th in family_names:
        if i <= end: return key, th
    raise AssertionError(i)

banks = [
 [('agenda','กำหนดการ'),('deadline','กำหนดส่ง'),('dashboard','แดชบอร์ด'),('handoff','การส่งต่องาน'),('update','การอัปเดต'),('brief','สรุปย่อ'),('priority','ลำดับความสำคัญ'),('progress','ความคืบหน้า'),('client','ลูกค้า'),('schedule','ตารางเวลา')],
 [('appointment','นัดหมาย'),('calendar','ปฏิทิน'),('reminder','การเตือน'),('route','เส้นทาง'),('booking','การจอง'),('receipt','ใบเสร็จ'),('address','ที่อยู่'),('arrival','การมาถึง'),('departure','การออกเดินทาง'),('confirmation','การยืนยัน')],
 [('budget','งบประมาณ'),('invoice','ใบแจ้งหนี้'),('refund','เงินคืน'),('fee','ค่าธรรมเนียม'),('salary','เงินเดือน'),('expense','ค่าใช้จ่าย'),('rate','อัตรา'),('contract','สัญญา'),('quote','ใบเสนอราคา'),('payment','การชำระเงิน')],
 [('bug','ข้อผิดพลาด'),('release','การปล่อยงาน'),('feature','ฟีเจอร์'),('backup','ไฟล์สำรอง'),('account','บัญชี'),('password','รหัสผ่าน'),('device','อุปกรณ์'),('network','เครือข่าย'),('server','เซิร์ฟเวอร์'),('access','สิทธิ์เข้าใช้')],
 [('feedback','ความคิดเห็น'),('proposal','ข้อเสนอ'),('draft','ร่าง'),('revision','การแก้ไข'),('approval','การอนุมัติ'),('decision','การตัดสินใจ'),('reason','เหตุผล'),('option','ตัวเลือก'),('risk','ความเสี่ยง'),('result','ผลลัพธ์')],
 [('meeting','การประชุม'),('participant','ผู้เข้าร่วม'),('presentation','การนำเสนอ'),('question','คำถาม'),('answer','คำตอบ'),('summary','สรุป'),('note','โน้ต'),('action','การลงมือทำ'),('owner','ผู้รับผิดชอบ'),('follow-up','การติดตามผล')],
 [('shift','กะงาน'),('queue','คิว'),('supplier','ผู้ขาย'),('delivery','การส่งของ'),('stock','สต็อก'),('order','คำสั่งซื้อ'),('package','พัสดุ'),('warehouse','คลังสินค้า'),('support','การช่วยเหลือ'),('ticket','ทิกเก็ต')],
 [('policy','นโยบาย'),('permission','การอนุญาต'),('safety','ความปลอดภัย'),('notice','ประกาศ'),('rule','กฎ'),('requirement','ข้อกำหนด'),('training','การอบรม'),('badge','บัตรประจำตัว'),('facility','สถานที่'),('incident','เหตุการณ์')],
 [('interview','การสัมภาษณ์'),('candidate','ผู้สมัคร'),('role','บทบาท'),('experience','ประสบการณ์'),('strength','จุดแข็ง'),('goal','เป้าหมาย'),('teamwork','การทำงานเป็นทีม'),('skill','ทักษะ'),('reference','ผู้รับรอง'),('offer','ข้อเสนอ')],
 [('habit','นิสัย'),('routine','กิจวัตร'),('choice','ทางเลือก'),('challenge','ความท้าทาย'),('solution','วิธีแก้'),('effort','ความพยายาม'),('practice','การฝึก'),('mistake','ข้อผิดพลาด'),('improvement','การพัฒนา'),('confidence','ความมั่นใจ')],
 [('airport','สนามบิน'),('platform','ชานชาลา'),('luggage','สัมภาระ'),('passport','หนังสือเดินทาง'),('gate','ประตูขึ้นเครื่อง'),('delay','ความล่าช้า'),('ticket','ตั๋ว'),('station','สถานี'),('map','แผนที่'),('direction','ทิศทาง')],
 [('recipe','สูตรอาหาร'),('ingredient','ส่วนผสม'),('menu','เมนู'),('portion','ส่วนหนึ่ง'),('flavor','รสชาติ'),('order','คำสั่งอาหาร'),('table','โต๊ะ'),('kitchen','ห้องครัว'),('breakfast','อาหารเช้า'),('allergy','อาการแพ้')],
 [('neighborhood','ย่านที่พัก'),('building','อาคาร'),('entrance','ทางเข้า'),('floor','ชั้น'),('elevator','ลิฟต์'),('corner','หัวมุม'),('bridge','สะพาน'),('park','สวน'),('library','ห้องสมุด'),('clinic','คลินิก')],
 [('weather','สภาพอากาศ'),('forecast','พยากรณ์'),('temperature','อุณหภูมิ'),('umbrella','ร่ม'),('traffic','การจราจร'),('noise','เสียงรบกวน'),('crowd','ฝูงชน'),('event','งานอีเวนต์'),('ticket','ตั๋ว'),('weekend','สุดสัปดาห์')],
 [('phone','โทรศัพท์'),('message','ข้อความ'),('call','การโทร'),('email','อีเมล'),('file','ไฟล์'),('link','ลิงก์'),('photo','รูปภาพ'),('voice','เสียง'),('reply','การตอบกลับ'),('contact','ผู้ติดต่อ')],
 [('project','โครงการ'),('milestone','หมุดหมาย'),('launch','การเปิดตัว'),('timeline','เส้นเวลา'),('resource','ทรัพยากร'),('task','งาน'),('quality','คุณภาพ'),('scope','ขอบเขต'),('issue','ประเด็น'),('target','เป้าหมาย')],
 [('health','สุขภาพ'),('appointment','นัดหมาย'),('medicine','ยา'),('exercise','การออกกำลัง'),('sleep','การนอน'),('stress','ความเครียด'),('energy','พลังงาน'),('symptom','อาการ'),('advice','คำแนะนำ'),('recovery','การฟื้นตัว')],
 [('home','บ้าน'),('rent','ค่าเช่า'),('repair','การซ่อม'),('key','กุญแจ'),('neighbor','เพื่อนบ้าน'),('utility','สาธารณูปโภค'),('room','ห้อง'),('kitchen','ห้องครัว'),('furniture','เฟอร์นิเจอร์'),('move','การย้าย')],
 [('course','หลักสูตร'),('lesson','บทเรียน'),('exercise','แบบฝึก'),('teacher','ครู'),('classmate','เพื่อนร่วมชั้น'),('exam','การสอบ'),('skill','ทักษะ'),('note','โน้ต'),('level','ระดับ'),('certificate','ใบประกาศ')],
 [('environment','สิ่งแวดล้อม'),('recycle','รีไซเคิล'),('waste','ขยะ'),('energy','พลังงาน'),('water','น้ำ'),('plastic','พลาสติก'),('community','ชุมชน'),('garden','สวน'),('public','สาธารณะ'),('campaign','แคมเปญ')],
]

VOCAB_VERB_TH = {
    'agreed': 'เห็นด้วยกับ', 'answered': 'ตอบ', 'arranged': 'จัดเตรียม',
    'asked': 'ถาม', 'assessed': 'ประเมิน', 'assigned': 'มอบหมาย',
    'avoided': 'หลีกเลี่ยง', 'booked': 'จอง', 'bought': 'ซื้อ',
    'briefed': 'บรีฟ', 'cared': 'ดูแล', 'changed': 'เปลี่ยน',
    'checked': 'ตรวจ', 'chose': 'เลือก', 'cleaned': 'ทำความสะอาด',
    'cleared': 'เคลียร์', 'collected': 'รับ', 'compared': 'เปรียบเทียบ',
    'completed': 'ทำให้เสร็จ', 'confirmed': 'ยืนยัน', 'contacted': 'ติดต่อ',
    'covered': 'ดูแล', 'described': 'อธิบาย', 'discussed': 'พูดคุยเรื่อง',
    'edited': 'แก้ไข', 'explained': 'อธิบาย', 'fixed': 'แก้ไข',
    'followed': 'ทำตาม', 'helped': 'ช่วย', 'informed': 'แจ้ง',
    'inspected': 'ตรวจสอบ', 'joined': 'เข้าร่วม', 'labelled': 'ติดป้าย',
    'logged': 'บันทึก', 'met': 'พบกันที่', 'monitored': 'เฝ้าดู',
    'moved': 'ย้าย', 'noted': 'จด', 'opened': 'เปิด', 'paid': 'จ่าย',
    'planned': 'วางแผน', 'posted': 'โพสต์', 'practised': 'ฝึก',
    'prepared': 'เตรียม', 'protected': 'ปกป้อง', 'provided': 'ให้',
    'reached': 'บรรลุ', 'read': 'อ่าน', 'recorded': 'บันทึก',
    'recycle': 'รีไซเคิล', 'reduced': 'ลด', 'reported': 'รายงาน',
    'requested': 'ขอ', 'reviewed': 'ทบทวน', 'saved': 'บันทึก',
    'scheduled': 'นัดหมาย', 'secured': 'รักษาความปลอดภัยให้', 'set': 'ตั้ง',
    'shared': 'แบ่งปัน', 'showed': 'แสดง', 'signed': 'เซ็น',
    'talked': 'พูดคุยเรื่อง', 'tested': 'ทดสอบ', 'took': 'ใช้',
    'tracked': 'ติดตาม', 'visited': 'ไปที่', 'waited': 'รอที่',
    'walked': 'เดินไปที่', 'welcomed': 'ต้อนรับ', 'wrote': 'เขียน',
}

VOCAB_MULTIWORD_VERB_TH = (
    ('asked for', 'ขอ'),
    ('asked about', 'ถาม'),
    ('marked the date', 'ทำเครื่องหมายวันที่ในปฏิทิน'),
    ('made a choice', 'เลือก'),
    ('made a call', 'ทำการโทร'),
    ('took the medicine', 'รับประทาน'),
    ('practised with', 'ฝึกกับ'),
    ('bought each', 'ซื้อ'),
)

VOCAB_RELATION_TH = (
    (' before ', 'ก่อน'), (' after ', 'หลัง'), (' during ', 'ระหว่าง'),
    (' around ', 'รอบ'), (' through ', 'ผ่าน'), (' about ', 'เกี่ยวกับ'),
    (' for ', 'สำหรับ'), (' from ', 'จาก'), (' with ', 'กับ'),
    (' of ', 'ของ'), (' on ', 'ใน'), (' in ', 'ใน'), (' at ', 'ใน'),
)

def thai_vocab_example(term, meaning, english, t):
    """Translate each vocabulary usage sentence by its action and relation.

    The English recipes intentionally vary the action (check, review, request,
    etc.). Reusing one Thai phrase for every word hides that distinction and
    gives learners an inaccurate meaning, so infer the same action and timing
    relation from the concrete sentence.
    """
    body = english.rstrip('.').strip()
    lower = body.casefold()
    if term == 'sleep' and 'health check' in lower:
        return 'เราติดตามการนอนระหว่างการตรวจสุขภาพ'

    rest = body[3:].strip() if lower.startswith('we ') else body
    action = None
    for prefix, thai in VOCAB_MULTIWORD_VERB_TH:
        if rest.casefold().startswith(prefix):
            action = thai
            break
    if action is None:
        verb = rest.split(maxsplit=1)[0].casefold() if rest else ''
        action = VOCAB_VERB_TH.get(verb)
    if action is None:
        raise AssertionError(f'missing Thai vocabulary action for {term}: {english}')

    # The final connector before the scenario carries the practical time or
    # topic relationship. Some health examples have a fixed sentence instead.
    scenario = t['scenario']
    scenario_position = body.rfind(f' {scenario}')
    prefix = body[:scenario_position] + ' ' if scenario_position >= 0 else body
    relation = ''
    relation_pos = -1
    for english_relation, thai_relation in VOCAB_RELATION_TH:
        position = prefix.rfind(english_relation)
        if position > relation_pos:
            relation_pos = position
            relation = thai_relation
    if not relation:
        relation = 'ใน'
    return f"เรา{action}{meaning}{relation}{t['scenario_th']}"

def vocab_example(term, meaning, t):
    scenario=t['scenario']; scenario_th=t['scenario_th']
    usage={
      'agenda':f"We checked the agenda before {scenario}.", 'deadline':f"We noted the deadline for {scenario}.",
      'dashboard':f"We opened the dashboard during {scenario}.", 'handoff':f"We planned the handoff before {scenario}.",
      'brief':f"We read the brief before {scenario}.", 'priority':f"We agreed on the priority during {scenario}.",
      'progress':f"We reviewed progress during {scenario}.", 'client':f"We briefed the client before {scenario}.",
      'schedule':f"We checked the schedule before {scenario}.", 'update':f"We shared the update after {scenario}.",
      'salary':f"We discussed salary during {scenario}.", 'revision':f"We reviewed the revision before {scenario}.",
      'experience':f"We described our experience during {scenario}.", 'flavor':f"We compared the flavor during {scenario}.",
      'appointment':f"We confirmed the appointment before {scenario}.", 'calendar':f"We marked the date on the calendar for {scenario}.",
      'reminder':f"We set a reminder for {scenario}.", 'route':f"We checked the route before {scenario}.",
      'booking':f"We confirmed the booking for {scenario}.", 'receipt':f"We kept the receipt after {scenario}.",
      'address':f"We checked the address before {scenario}.", 'arrival':f"We tracked the arrival time for {scenario}.",
      'departure':f"We checked the departure time before {scenario}.", 'confirmation':f"We saved the confirmation for {scenario}.",
      'budget':f"We reviewed the budget during {scenario}.", 'invoice':f"We checked the invoice after {scenario}.",
      'refund':f"We requested a refund after {scenario}.", 'fee':f"We asked about the fee before {scenario}.",
      'salary':f"We discussed the salary during {scenario}.", 'expense':f"We recorded the expense for {scenario}.",
      'rate':f"We compared the rate before {scenario}.", 'contract':f"We signed the contract after {scenario}.",
      'quote':f"We requested a quote for {scenario}.", 'payment':f"We confirmed the payment after {scenario}.",
      'bug':f"We fixed the bug before {scenario}.", 'release':f"We planned the release after {scenario}.",
      'feature':f"We tested the feature before {scenario}.", 'backup':f"We checked the backup before {scenario}.",
      'account':f"We secured the account for {scenario}.", 'password':f"We changed the password after {scenario}.",
      'device':f"We tested the device before {scenario}.", 'network':f"We checked the network during {scenario}.",
      'server':f"We monitored the server during {scenario}.", 'access':f"We requested access before {scenario}.",
      'feedback':f"We requested feedback after {scenario}.", 'proposal':f"We reviewed the proposal before {scenario}.",
      'draft':f"We edited the draft for {scenario}.", 'revision':f"We discussed the revision during {scenario}.",
      'approval':f"We waited for approval before {scenario}.", 'decision':f"We recorded the decision after {scenario}.",
      'reason':f"We explained the reason during {scenario}.", 'option':f"We compared each option before {scenario}.",
      'risk':f"We assessed the risk before {scenario}.", 'result':f"We reviewed the result after {scenario}.",
      'meeting':f"We joined the meeting for {scenario}.", 'participant':f"We welcomed the participant at {scenario}.",
      'presentation':f"We prepared the presentation for {scenario}.", 'question':f"We answered the question during {scenario}.",
      'answer':f"We checked the answer after {scenario}.", 'summary':f"We read the summary after {scenario}.",
      'note':f"We wrote a note during {scenario}.", 'action':f"We assigned an action after {scenario}.",
      'owner':f"We contacted the owner about {scenario}.", 'follow-up':f"We scheduled a follow-up after {scenario}.",
      'shift':f"We covered the shift during {scenario}.", 'queue':f"We cleared the queue after {scenario}.",
      'supplier':f"We contacted the supplier about {scenario}.", 'delivery':f"We tracked the delivery for {scenario}.",
      'stock':f"We checked the stock before {scenario}.", 'order':f"We confirmed the order for {scenario}.",
      'package':f"We labelled the package for {scenario}.", 'warehouse':f"We visited the warehouse for {scenario}.",
      'support':f"We provided support during {scenario}.", 'ticket':f"We closed the ticket after {scenario}.",
      'policy':f"We read the policy before {scenario}.", 'permission':f"We requested permission for {scenario}.",
      'safety':f"We checked safety before {scenario}.", 'notice':f"We posted a notice about {scenario}.",
      'rule':f"We explained the rule before {scenario}.", 'requirement':f"We checked the requirement for {scenario}.",
      'training':f"We scheduled training for {scenario}.", 'badge':f"We checked the badge before {scenario}.",
      'facility':f"We inspected the facility for {scenario}.", 'incident':f"We reported the incident after {scenario}.",
      'interview':f"We arranged the interview for {scenario}.", 'candidate':f"We welcomed the candidate at {scenario}.",
      'role':f"We explained the role during {scenario}.", 'experience':f"We discussed the experience during {scenario}.",
      'strength':f"We described a strength during {scenario}.", 'goal':f"We set a goal for {scenario}.",
      'teamwork':f"We discussed teamwork during {scenario}.", 'skill':f"We practised a skill for {scenario}.",
      'reference':f"We checked a reference after {scenario}.", 'offer':f"We reviewed the offer after {scenario}.",
      'habit':f"We changed a habit during {scenario}.", 'routine':f"We planned a routine for {scenario}.",
      'choice':f"We made a choice during {scenario}.", 'challenge':f"We discussed the challenge in {scenario}.",
      'solution':f"We tested a solution for {scenario}.", 'effort':f"We recognised the effort during {scenario}.",
      'practice':f"We scheduled practice before {scenario}.", 'mistake':f"We corrected a mistake after {scenario}.",
      'improvement':f"We measured an improvement after {scenario}.", 'confidence':f"We built confidence through {scenario}.",
      'airport':f"We checked the airport map before {scenario}.", 'platform':f"We waited at the platform during {scenario}.",
      'luggage':f"We labelled our luggage before {scenario}.", 'passport':f"We showed our passport before {scenario}.",
      'gate':f"We walked to the gate before {scenario}.", 'delay':f"We reported the delay during {scenario}.",
      'ticket':f"We saved the ticket for {scenario}.", 'station':f"We met at the station before {scenario}.",
      'map':f"We checked the map before {scenario}.", 'direction':f"We asked for directions during {scenario}.",
      'recipe':f"We followed the recipe during {scenario}.", 'ingredient':f"We bought each ingredient for {scenario}.",
      'menu':f"We read the menu before {scenario}.", 'portion':f"We chose a small portion for {scenario}.",
      'flavor':f"We discussed the flavor during {scenario}.", 'table':f"We booked a table for {scenario}.",
      'kitchen':f"We cleaned the kitchen after {scenario}.", 'breakfast':f"We prepared breakfast before {scenario}.",
      'allergy':f"We reported an allergy before {scenario}.", 'health':f"We talked about health during {scenario}.",
      'medicine':f"We took the medicine after {scenario}.", 'exercise':f"We scheduled exercise around {scenario}.",
      'sleep':"We tracked sleep during the health check.", 'stress':f"We discussed stress during {scenario}.",
      'energy':f"We tracked energy during {scenario}.", 'symptom':f"We reported the symptom during {scenario}.",
      'advice':f"We followed the advice after {scenario}.", 'recovery':f"We discussed recovery after {scenario}.",
      'home':f"We cleaned the home before {scenario}.", 'rent':f"We paid the rent before {scenario}.",
      'repair':f"We requested a repair after {scenario}.", 'key':f"We collected the key before {scenario}.",
      'neighbor':f"We informed the neighbor about {scenario}.", 'utility':f"We checked the utility bill after {scenario}.",
      'room':f"We prepared the room for {scenario}.", 'furniture':f"We moved the furniture before {scenario}.",
      'move':f"We planned the move after {scenario}.", 'course':f"We joined the course for {scenario}.",
      'lesson':f"We reviewed the lesson before {scenario}.", 'teacher':f"We asked the teacher about {scenario}.",
      'classmate':f"We practised with a classmate during {scenario}.", 'exam':f"We prepared for the exam before {scenario}.",
      'certificate':f"We requested the certificate after {scenario}.", 'environment':f"We protected the environment during {scenario}.",
      'recycle':f"We recycle plastic after {scenario}.", 'waste':f"We reduced waste during {scenario}.",
      'energy':f"We tracked energy during {scenario}.", 'water':f"We saved water during {scenario}.",
      'plastic':f"We avoided plastic during {scenario}.", 'community':f"We helped the community with {scenario}.",
      'garden':f"We cared for the garden after {scenario}.", 'public':f"We chose a public space for {scenario}.",
      'campaign':f"We joined the campaign for {scenario}.", 'phone':f"We charged the phone before {scenario}.",
      'message':f"We sent a message after {scenario}.", 'call':f"We made a call about {scenario}.",
      'email':f"We answered the email after {scenario}.", 'file':f"We attached the file for {scenario}.",
      'link':f"We opened the link before {scenario}.", 'photo':f"We saved the photo from {scenario}.",
      'voice':f"We recorded a voice note for {scenario}.", 'reply':f"We sent a reply after {scenario}.",
      'contact':f"We updated the contact after {scenario}.", 'project':f"We reviewed the project during {scenario}.",
      'milestone':f"We reached a milestone in {scenario}.", 'launch':f"We planned the launch for {scenario}.",
      'timeline':f"We checked the timeline for {scenario}.", 'resource':f"We assigned a resource to {scenario}.",
      'task':f"We completed a task for {scenario}.", 'quality':f"We checked quality during {scenario}.",
      'scope':f"We agreed on the scope of {scenario}.", 'issue':f"We logged an issue during {scenario}.",
      'target':f"We set a target for {scenario}.",
    }
    # Keep overrides after the bank-specific entries so repeated terms stay natural.
    usage.update({
      'salary':f"We discussed salary during {scenario}.",
      'revision':f"We reviewed the revision before {scenario}.",
      'experience':f"We described our experience during {scenario}.",
      'flavor':f"We compared the flavor during {scenario}.",
    })
    if term not in usage:
        raise AssertionError(f'missing natural vocabulary recipe for {term}')
    english = usage[term]
    return english, thai_vocab_example(term, meaning, english, t)

def vocab(i):
    # Each scenario has a related bank; no generic word-number placeholders.
    topic_bank=[0,15,2,6,5,4,3,10,8,6,2,11,16,17,17,19,15,7,4,10]
    bank=banks[topic_bank[(i-1)%len(topic_bank)]]
    t=topic(i); out=[]
    for j,(term,meaning) in enumerate(bank,1):
        en,th=vocab_example(term,meaning,t)
        out.append({'id':f'v-{j:02d}','term':term,'meaning_th':meaning,'example_en':en,'example_th':th})
    return out

def make_quiz(i, ex, words, pattern):
    t=topic(i)
    # The first two items are meaning checks. Every option is a valid example
    # of this lesson's pattern; only one matches the requested meaning, so a
    # learner is never marked wrong for choosing another grammatical sentence.
    def rotate(items, shift):
        shift %= len(items)
        return items[shift:] + items[:shift]
    first_options = rotate([ex[0]['en'], ex[1]['en'], ex[2]['en']], i)
    second_options = rotate([ex[0]['en'], ex[1]['en'], ex[2]['en']], i + 1)
    vocabulary_options = rotate([words[0]['term'], words[1]['term'], words[2]['term']], i + 2)
    return [
      {'id':'quiz-01','kind':'choice','prompt_en':'Which sentence matches the first Thai meaning?','prompt_th':f"ประโยคใดตรงกับความหมายนี้: {ex[0]['th']}",'options':first_options,'answers':[ex[0]['en']], 'explanation_th':f"ประโยคที่ตรงกับความหมายใช้โครงสร้าง {pattern}"},
      {'id':'quiz-02','kind':'choice','prompt_en':'Which sentence matches the second Thai meaning?','prompt_th':f"ประโยคใดตรงกับความหมายนี้: {ex[1]['th']}",'options':second_options,'answers':[ex[1]['en']], 'explanation_th':f"ประโยคที่ตรงกับความหมายใช้โครงสร้าง {pattern}"},
      {'id':'quiz-03','kind':'write','prompt_en':f"Write one sentence using this formula: {pattern}",'prompt_th':f"เขียนหนึ่งประโยคตามสูตรนี้: {pattern}",'options':[],'answers':[ex[2]['en']], 'explanation_th':'ตรวจรูปกริยา คำช่วย และลำดับคำให้ตรงกับสูตร'},
      {'id':'quiz-04','kind':'choice','prompt_en':f"Which English word means: {words[0]['meaning_th']}?",'prompt_th':f"คำภาษาอังกฤษคำใดแปลว่า “{words[0]['meaning_th']}” ในสถานการณ์{topic(i)['scenario_th']}",'options':vocabulary_options,'answers':[words[0]['term']], 'explanation_th':f"คำตอบคือ {words[0]['term']} ซึ่งหมายถึง {words[0]['meaning_th']}"},
      {'id':'quiz-05','kind':'write','prompt_en':f"Write a useful sentence for {topic(i)['scenario']} using this lesson pattern.",'prompt_th':f"เขียนประโยคที่ใช้ได้จริงใน{topic(i)['scenario_th']}โดยคงโครงสร้างของบท",'options':[],'answers':[ex[1]['en']], 'explanation_th':'คำตอบควรสื่อสารได้จริงและคงโครงสร้างของบทเรียน'},
    ]

PATTERNS = ['Subject + am/is/are + verb-ing + object/time.', 'Subject + base verb(s) + frequency phrase.', 'Subject + am/is/are + verb-ing now; subject + base verb(s) routinely.', 'Subject + am/is/are + verb-ing temporarily; subject + base verb(s) permanently.', 'Subject + past verb + completed event/time.', 'Subject + was/were + verb-ing when + past event.', 'Subject + have/has + past participle + current result.', 'Subject + have/has + past participle + unfinished time/experience.', 'Subject + have/has been + verb-ing + for/since + duration.', 'Subject + have/has been + verb-ing; subject + have/has + past participle.', 'How long + have/has + subject + been + verb-ing?', 'Subject + have/has + past participle + for/since + time.', 'Subject + have/has + past participle; subject + past verb + finished time.', 'Subject + have/has + past participle for current result; subject + past verb at + time.', 'Subject + had + past participle + before + past event.', 'Subject + had been + verb-ing + before + past event.', 'Subject + have/has + noun; subject + have/has got + noun.', 'Subject + used to + base verb + past habit.', 'Subject + am/is/are + verb-ing + future appointment/time.', 'Subject + am/is/are going to + base verb + plan/evidence.', 'Subject + will + base verb; Shall we + base verb?', 'Subject + will + base verb + promise/offer.', 'Subject + will + base verb + decision; subject + am/is/are going to + plan.', 'Subject + will be + verb-ing; subject + will have + past participle + deadline.', 'When/if + present clause, subject + will + base verb.', 'Subject + can/could + base verb; subject + be able to + base verb.', 'Subject + could + base verb (past ability); subject + could have + past participle (missed chance).', "Subject + must + base verb; subject + can't + base verb (logical conclusion).", 'Subject + may/might + base verb + present possibility.', 'Subject + may/might have + past participle + past possibility.', 'Subject + have to/must + base verb + obligation.', "Subject + mustn't + base verb; subject + needn't + base verb.", 'Subject + should + base verb + advice.', 'Subject + should + base verb + expectation/criticism.', "Subject + had better + base verb; It's time + subject + past verb.", 'Subject + would + base verb + polite request/past habit.', 'Could/Would + subject + base verb?; Would you like + noun/to + verb?', 'If + present, subject + will; if + past, subject + would.', 'If + subject + past, subject + would; subject + wish + past.', 'If + subject + had + past participle, subject + would have + past participle; wish + past perfect.', 'Subject + wish + past/past perfect/would + base.', 'Subject + am/is/are/was/were + past participle (passive).', 'Subject + be/been/being + past participle in a modal/perfect/gerund.', 'It is/was + past participle + that-clause; subject + be + reported + to.', 'Subject + be + supposed to + base verb.', 'Subject + have + object + past participle.', 'Subject + said (that) + clause with backshift.', 'Subject + told/asked + object + to/not to + base verb.', 'Wh/Yes-No question + auxiliary + subject + base verb?', 'Subject + asked/wondered + wh/if + subject + verb.', "I think/hope/believe + so/not; I don't think + so.", 'Statement + question tag auxiliary + pronoun?', 'Verb + gerund; stop/avoid/enjoy + verb-ing.', 'Verb + to + base verb; decide/remember + infinitive.', 'Verb + object + to + base verb.', 'Remember/regret + verb-ing versus + to + base verb.', 'Try/need/help + gerund/infinitive depending meaning.', 'Like/love/hate + gerund; would like + to + base.', 'Prefer + gerund/to + base; would rather + base.', 'Preposition + verb-ing.', 'Be/get used to + noun/verb-ing.', 'Succeed/insist + preposition + verb-ing.', 'There is no point in + verb-ing; It is worth + verb-ing.', 'To + base verb (purpose); for + noun; so that + clause.', 'Adjective + to + base verb.', 'Adjective + preposition + verb-ing.', 'See/hear + object + base verb versus verb-ing.', 'Verb-ing clause + main clause with shared subject.', 'Countable plural versus uncountable noun in quantity phrases.', 'Uncountable noun + some/much/a piece of + noun.', 'A/an + singular count; some + plural/uncountable.', 'First mention a/an; known item the.', 'The + shared/unique noun.', 'Zero article institution; the + building/place.', 'Plural noun in general; the + specific group.', 'The + species/instrument/adjective group.', 'Proper names without the; the + geographic names.', 'The + organizations/media/collections; zero article for names.', 'Regular/irregular plural noun forms.', 'Noun+noun compound.', 'Noun + of + noun.', 'Reflexive pronoun after subject/object.', "On one's own/by oneself + verb.", 'There is/are + noun; it is + adjective/time.', 'Some in offers/positive; any in questions/negative.', 'No + noun; none/nothing/nobody as pronoun.', 'Much/many/little/few/a lot of/plenty of + noun.', 'All/most/no/none + (of) + determiner/pronoun.', 'Either + singular noun; either of + plural pronoun.', 'All/every/whole + noun.', 'Each/every + singular noun; each of + plural.', 'Relative who/that/which + clause.', 'Object relative pronoun can be omitted.', 'Whose/whom/where + clause.', 'Non-defining relative clause with commas.', 'Non-defining relative clause with extra contrast.', 'Past participle phrase + noun (reduced relative).', '-ing adjective for cause; -ed adjective for feeling.', 'Adjective order + noun; adjective after linking verb.', 'Adjective + noun; verb + adverb.', 'Well/fast/late/hard/hardly in adverb position.', 'So + adjective/adverb; such + (a/an) + noun.', 'Adjective/adverb + enough; too + adjective + to.', 'Quite/pretty/rather/fairly + adjective/adverb.', 'Adjective/adverb comparative + than.', 'Much/far/a lot + comparative; any + comparative.', 'As + adjective/adverb + as; not as + ... + as.', 'The + superlative + noun.', 'Verb + object + place + time.', 'Frequency/manner adverb + auxiliary/main verb.', 'Still/any more/yet/already + correct position.', 'Even + focused word/phrase.', 'In spite of/despite + noun/gerund; although + clause.', 'In case + present clause.', 'Unless/as long as/provided + clause.', 'As + clause for time/reason.', 'Like + noun; as + clause/role.', 'Like/as if + clause.', 'During + noun; for + duration; while + clause.', 'By + deadline; until + endpoint; by the time + clause.', 'At + clock time; on + day/date; in + month/year.', 'On time/in time; at the end/in the end.', 'In/at/on + position phrase.', 'In/at/on + building/surface/address.', 'In/at/on + transport/place/location.', 'Go/come + to; arrive + at/in; move + into.', 'At work/home; on duty/in trouble etc.', 'By + transport/agent/deadline.', 'Noun + preposition collocation.', 'Adjective + preposition collocation A.', 'Adjective + preposition collocation B.', 'Verb + to/at + object.', 'Verb + about/for/of/after + object.', 'Verb + about/of + topic.', 'Verb + of/for/from/on + object.', 'Verb + in/into/with/to/on + object.', 'Phrasal verb = verb + particle; meaning not literal.', 'Phrasal verbs with in/out.', 'Phrasal verbs with out.', 'Phrasal verbs with on/off (start/stop).', 'Phrasal verbs with on/off (continue/remove).', 'Phrasal verbs with up/down.', 'Phrasal verbs with up (increase/finish).', 'Phrasal verbs with up (approach/collect).', 'Phrasal verbs with away/back.']

THAI_EXAMPLE_TEMPLATES = [
    ("ฉันกำลัง{vt}{ot}ก่อนเที่ยง", "{p}กำลัง{vt}{o2t}ขณะที่ฉันจดโน้ต", "เรากำลัง{vt}{ot}และ{o2t}ร่วมกัน"),
    ("ฉัน{vt}{ot}ทุกเช้า", "{p}{vt}{o2t}ทุกวันจันทร์", "ทีมของเรา{vt}{ot}ทุกบ่าย"),
    ("ฉันกำลัง{vt}{ot}ตอนนี้ แต่ฉัน{vt}มันทุกวันศุกร์", "{p}กำลัง{vt}{o2t}วันนี้ แต่{p}{vt}บันทึกทุกวัน", "เรากำลัง{vt}{ot}สัปดาห์นี้ และเรา{vt}{o2t}ทุกไตรมาส"),
    ("สัปดาห์นี้ฉันกำลัง{vt}{ot} แต่ปกติฉัน{vt}มันวันศุกร์", "{p}กำลัง{vt}{o2t}ชั่วคราว แต่{p}{vt}กระบวนการนี้เป็นงานประจำ", "เรากำลัง{vt}{ot}สำหรับโครงการนี้ แต่เรา{vt}{o2t}เป็นงานประจำ"),
    ("เมื่อวานฉัน{vt}{ot}", "{p}{vt}{o2t}หลังการโทร", "เราทำทั้ง{ot}และ{o2t}เสร็จก่อนอาหารกลางวัน"),
    ("ฉันกำลัง{vt}{ot}ตอนโทรศัพท์ดัง", "{p}กำลัง{vt}{o2t}ตอนลูกค้าโทรมา", "เรากำลัง{vt}{ot}เมื่อการประชุมเริ่ม"),
    ("ฉัน{vt}{ot}แล้ว จึงพร้อมใช้งาน", "{p}{vt}{o2t}แล้ว ทีมจึงทำงานต่อได้", "เราทำไฟล์ทั้งสองรายการเสร็จแล้ว การส่งต่องานจึงพร้อม"),
    ("ฉัน{vt}{ot}สองครั้งในสัปดาห์นี้", "{p}{vt}{o2t}มาก่อน", "เรา{vt}กระบวนการนี้หลายครั้งในเดือนนี้"),
    ("ฉันกำลัง{vt}{ot}มาตั้งแต่เก้าโมง", "{p}กำลัง{vt}{o2t}มาสองชั่วโมงแล้ว", "เรากำลัง{vt}แผนส่งต่องานมาตลอดเช้า"),
    ("ฉันกำลัง{vt}{ot}อยู่ จึงทำส่วนแรกเสร็จแล้ว", "{p}กำลัง{vt}{o2t} และ{p}ทำประเด็นหนึ่งเสร็จแล้ว", "เรากำลัง{vt}{ot} และเราเตรียม{o2t}เสร็จแล้ว"),
    ("คุณกำลัง{vt}{ot}มานานเท่าไรแล้ว", "{p}กำลัง{vt}{o2t}มานานเท่าไรแล้ว", "พวกเขากำลัง{vt}แผนส่งต่องานมานานเท่าไรแล้ว"),
    ("ฉันทำงานกับ{ot}มาสามปีแล้ว", "{p}ดูแล{o2t}มาตั้งแต่เดือนมกราคม", "เราใช้กระบวนการนี้มาตั้งแต่การตรวจครั้งก่อน"),
    ("ฉันทำ{ot}เสร็จแล้ว ฉันทำเมื่อวาน", "{p}ทำ{o2t}เสร็จแล้ว และทำเมื่อสัปดาห์ก่อน", "เราทำ{ot}เสร็จแล้ว แต่ส่ง{o2t}วันจันทร์"),
    ("ฉันทำ{ot}เสร็จแล้ว จึงส่งสรุปตอนเที่ยง", "{p}ทำ{o2t}เสร็จแล้ว และส่งตอนบ่ายสาม", "เรายืนยันการส่งต่องานแล้ว และแจ้งลูกค้าตอนสี่โมง"),
    ("ฉันทำ{ot}เสร็จก่อนการประชุมเริ่ม", "{p}ทำ{o2t}เสร็จก่อนลูกค้าโทรมา", "เราตรวจแผนปล่อยงานเสร็จก่อนเริ่มสาธิต"),
    ("ฉันกำลัง{vt}{ot}มาหนึ่งชั่วโมงก่อนอาหารกลางวัน", "{p}กำลัง{vt}{o2t}ก่อนลูกค้าโทรมา", "เรากำลังเตรียมการส่งต่องานมาสองชั่วโมงก่อนประชุม"),
    ("ฉันมี{ot}อยู่บนแล็ปท็อป", "{p}มี{o2t}อยู่", "เรามีไฟล์ และทีมมีสิทธิ์เข้าใช้"),
    ("ฉันเคย{vt}{ot}ทุกวันศุกร์", "{p}เคย{vt}{o2t}ด้วยมือ", "เราไม่เคยติดตามการส่งต่องานออนไลน์"),
    ("ฉันกำลัง{vt}{ot}กับลูกค้าพรุ่งนี้", "{p}{vt}{o2t}กับ Arun วันศุกร์", "เรานัดพบเรื่องการส่งต่องานวันจันทร์หน้า"),
    ("ฉันกำลังจะ{vt}{ot}หลังการโทร", "{p}กำลังจะ{vt}{o2t}ก่อนวันศุกร์", "เรากำลังจะเปลี่ยนแผนเพราะข้อมูลมาช้า"),
    ("ฉันจะ{vt}{ot}คืนนี้ เราคุยเรื่อง{o2t}พรุ่งนี้ไหม", "เราจะ{vt}{o2t} เราส่งให้ลูกค้าดีไหม", "ฉันจะตรวจ{ot} เราโทรหา Arun ดีไหม"),
    ("ฉันจะ{vt}{ot}ให้คุณ", "{p}จะส่ง{o2t}ตอนเที่ยง", "เราจะช่วยเรื่องแผนปล่อยงาน ฉันรับปาก"),
    ("ฉันจะ{vt}{ot}ตอนนี้ และกำลังจะส่ง{o2t}ทีหลัง", "{p}จะโทรหาลูกค้า และกำลังจะอัปเดตปฏิทินปล่อยงานก่อน", "เราจะเปลี่ยนตาราง และกำลังจะอธิบายเหตุผล"),
    ("พรุ่งนี้สิบโมงฉันจะกำลัง{vt}{ot} และเที่ยงจะทำ{o2t}เสร็จแล้ว", "{p}จะกำลังนำเสนอแผนปล่อยงานตอนบ่ายสาม และวันศุกร์จะทำรายงานเสร็จ", "บ่ายนี้เราจะกำลังเตรียมการส่งต่องาน และเย็นนี้จะส่งสรุปเสร็จ"),
    ("เมื่อฉัน{vt}{ot} ฉันจะโทรหาลูกค้า", "ถ้า{p}{vt}{o2t} เราจะส่งอีเมลปล่อยงาน", "เมื่อเราทำ{ot}เสร็จ เราจะแชร์{o2t}"),
    ("วันนี้ฉันสามารถ{vt}{ot}ได้", "{p}สามารถ{vt}{o2t}ได้ไหม", "เราจะสามารถทำการส่งต่องานเสร็จภายในวันศุกร์"),
    ("ตอนเป็นเด็กฝึกงานฉันสามารถ{vt}{ot}ได้", "{p}น่าจะ{vt}{o2t}ได้ แต่พลาดกำหนดส่ง", "เมื่อวานเราแก้ปัญหาได้ แต่เราน่าจะขอความช่วยเหลือเร็วกว่านี้"),
    ("เราต้อง{vt}{ot}ก่อนโทร และ{o2t}ยังไม่น่าจะเสร็จ", "{p}ต้องตรวจ{o2t} และมันยังเสร็จสมบูรณ์ไม่ได้หากไม่มีลายเซ็นลูกค้า", "ลูกค้าต้องออนไลน์ เพราะยังส่งการส่งต่องานไม่ได้"),
    ("ฉันอาจ{vt}{ot}หลังอาหารกลางวัน", "{p}อาจจะ{vt}{o2t}วันนี้", "เราอาจต้องเลื่อนการส่งต่องานถ้าลูกค้ามาช้า"),
    ("ฉันอาจทำ{ot}เร็วเกินไป", "{p}อาจพลาด{o2t}", "เราอาจส่งไฟล์ปล่อยงานผิด"),
    ("ฉันต้อง{vt}{ot}ก่อนอาหารกลางวัน", "{p}ต้อง{vt}{o2t}วันนี้", "เราต้องยืนยันการส่งต่องานให้ลูกค้าภายในวันศุกร์"),
    ("คุณต้องไม่แชร์{ot}นอกทีม และไม่จำเป็นต้องพิมพ์", "{p}ต้องไม่ลบ{o2t} และไม่จำเป็นต้องคัดลอกอีก", "เราต้องไม่พลาดกำหนดส่ง และไม่จำเป็นต้องทำงานดึกถ้าไฟล์พร้อม"),
    ("คุณควร{vt}{ot}ก่อนโทร", "{p}ควร{vt}{o2t}วันนี้", "เราควรยืนยันการส่งต่องานกับลูกค้า"),
    ("ลูกค้าน่าจะได้รับ{ot}ตอนเที่ยง แต่ตอนนี้ยังไม่มา", "{p}ควรตรวจ{o2t}ก่อนส่ง", "ตอนนี้เราควรรู้สถานะการส่งต่องานแล้ว"),
    ("คุณควร{vt}{ot} เราควรส่ง{o2t}ได้เวลาแล้ว", "{p}ควรอัปเดต{o2t} ถึงเวลาที่ลูกค้าควรรู้แล้ว", "เราควรยืนยันการปล่อยงาน ถึงเวลาโทรหาลูกค้าแล้ว"),
    ("เมื่อก่อนฉันจะ{vt}{ot}ทุกวันศุกร์", "{p}จะอัปเดต{o2t}หลังประชุมทุกครั้ง", "ฉันจะขอบคุณมากถ้าคุณช่วยยืนยันการส่งต่องาน"),
    ("คุณช่วย{vt}{ot}ได้ไหม คุณต้องการ{o2t}ไหม", "คุณต้องการให้{p}อัปเดต{o2t}ไหม {p}ช่วยส่งให้ลูกค้าได้ไหม", "เรายืนยันการส่งต่องานกันไหม คุณอยากเข้าร่วมสายไหม"),
    ("ถ้าฉัน{vt}{ot} ฉันจะส่ง{o2t}", "ถ้า{p}อัปเดต{o2t} {p}จะช่วยลูกค้า", "ถ้าเรามีเวลา เราจะตรวจการปล่อยงาน แต่ถ้ามีเวลามากกว่านี้เราจะทดสอบอีกครั้ง"),
    ("ถ้าฉันรู้รหัสผ่าน{ot} ฉันจะตรวจมัน ฉันอยากรู้จริงๆ", "ถ้า{p}มี{o2t} {p}จะอัปเดต และอยากให้มีอยู่ตอนนี้", "ถ้าเรารู้วันปล่อยงาน เราจะวางแผนการส่งต่องาน เราอยากรู้วันนั้น"),
    ("ถ้าฉันทำ{ot}เสร็จ ฉันคงพบปัญหา และอยากทำให้เสร็จ", "ถ้า{p}ทำ{o2t}เสร็จ {p}คงช่วยลูกค้าได้ และอยากทำให้เสร็จ", "ถ้าเราตรวจแผนปล่อยงาน เราคงเลี่ยงความล่าช้าได้ เราอยากตรวจให้เร็วกว่านี้"),
    ("ฉันอยากรู้รหัสผ่าน{ot}", "{p}อยากให้อัปเดต{o2t}แล้ว", "เราอยากให้ลูกค้าตอบวันนี้"),
    ("{ot}ถูกตรวจทุกเช้า", "{o2t}ถูกอัปเดตหลังการโทร", "รายงานปล่อยงานถูกตรวจสอบก่อนสาธิต"),
    ("{ot}ต้องถูกตรวจสอบก่อนเที่ยง", "{o2t}ถูกอัปเดตเรียบร้อยแล้ว", "แผนปล่อยงานกำลังถูกทีมตรวจ"),
    ("มีรายงานว่า{ot}พร้อมแล้ว", "มีการประกาศว่า{o2t}เปลี่ยนแล้ว", "มีรายงานว่า{p}อนุมัติการปล่อยงาน"),
    ("{ot}ควรพร้อมตอนเที่ยง", "{p}ควรอัปเดต{o2t}วันนี้", "เราควรยืนยันการส่งต่องานก่อนวันศุกร์"),
    ("ฉันให้ผู้เชี่ยวชาญตรวจ{ot}", "{p}ให้คนอัปเดต{o2t}ก่อนโทร", "เราจะให้ทีมตรวจรายงานปล่อยงานพรุ่งนี้"),
    ("ผู้ตรวจบอกว่าตนตรวจ{ot}แล้ว", "Arun บอกว่าเขาจะอัปเดต{o2t}", "ลูกค้าบอกว่าการส่งต่องานพร้อมแล้ว"),
    ("ผู้ตรวจบอกฉันให้{vt}{ot}", "ผู้จัดการขอให้ Arun อัปเดต{o2t}", "ฉันบอกทีมว่าอย่าแชร์แผนปล่อยงาน"),
    ("คุณ{vt}อะไรสำหรับ{ot}", "{p}อัปเดต{o2t}แล้วหรือยัง", "เราจะยืนยันการส่งต่องานก่อนวันศุกร์ไหม"),
    ("{p}ถามว่า{ot}อยู่ที่ไหน", "ฉันสงสัยว่า Arun อัปเดต{o2t}แล้วหรือยัง", "ลูกค้าถามว่าเราจะส่งแผนปล่อยงานเมื่อไร"),
    ("ฉันคิดว่า{ot}พร้อม และฉันก็คิดเช่นนั้น", "ฉันหวังว่า{o2t}พร้อม ฉันหวังว่าใช่", "ฉันไม่คิดว่าการส่งต่องานช้า ฉันไม่คิดเช่นนั้น"),
    ("{ot}พร้อมแล้วใช่ไหม", "{p}อัปเดต{o2t}แล้วใช่ไหม", "เรายืนยันการส่งต่องานได้ใช่ไหม"),
    ("ฉันชอบ{vt}{ot}", "{p}หยุดอัปเดต{o2t}", "เราเลี่ยงการแชร์แผนปล่อยงานนอกทีม"),
    ("ฉันตัดสินใจจะ{vt}{ot}", "{p}จำได้ว่าต้องอัปเดต{o2t}", "เราตกลงจะยืนยันการส่งต่องาน"),
    ("ฉันอยากให้คุณ{vt}{ot}", "{p}ขอให้ Arun อัปเดต{o2t}", "เราต้องการให้ลูกค้ายืนยันการส่งต่องาน"),
    ("ฉันจำได้ว่าเคย{vt}{ot}เมื่อวาน", "{p}จำได้ว่าต้องอัปเดต{o2t}", "เราเสียใจที่ส่งการส่งต่องานฉบับเก่า"),
    ("ฉันลอง{vt}{ot}ในเบราว์เซอร์ใหม่", "{p}จำเป็นต้องอัปเดต{o2t}", "เราช่วยลูกค้ายืนยันการส่งต่องาน"),
    ("ฉันชอบ{vt}{ot}", "{p}ชอบอัปเดต{o2t}", "เราอยากยืนยันการส่งต่องานวันนี้"),
    ("ฉันชอบ{vt}{ot}ตอนเช้า", "{p}ชอบอัปเดต{o2t}หลังอาหารกลางวัน", "เราอยากยืนยันการส่งต่องานทางโทรศัพท์มากกว่า"),
    ("ก่อน{vt}{ot} ฉันตรวจวาระ", "{p}ออกไปหลังอัปเดต{o2t}", "เราคุยเรื่องการยืนยันการส่งต่องาน"),
    ("ตอนนี้ฉันคุ้นกับ{ot}แล้ว", "{p}กำลังคุ้นกับการอัปเดต{o2t}", "เราคุ้นกับการทำงานกับกระบวนการส่งต่องาน"),
    ("เราทำสำเร็จในการตรวจ{ot}ตรงเวลา", "{p}ยืนยันว่าจะอัปเดต{o2t}ด้วยตนเอง", "พวกเขาทำสำเร็จในการยืนยันการส่งต่องาน"),
    ("ไม่มีประโยชน์ที่จะตรวจ{ot}ซ้ำ", "การอัปเดต{o2t}ตอนนี้คุ้มค่า", "ไม่มีประโยชน์ที่จะเลื่อนการส่งต่องาน การยืนยันจึงคุ้มค่า"),
    ("ฉันเปิด{ot}เพื่อดูข้อมูล", "{p}ใช้{o2t}สำหรับลูกค้า", "เราอัปเดตแผนปล่อยงานเพื่อให้ทีมลงมือได้"),
    ("{ot}ตรวจได้ง่าย", "{o2t}พร้อมส่ง", "เรายินดีที่จะยืนยันการส่งต่องาน"),
    ("ฉันรับผิดชอบการตรวจ{ot}", "{p}กังวลเรื่องการอัปเดต{o2t}", "เราสนใจที่จะยืนยันการส่งต่องาน"),
    ("ฉันเห็น{p}ตรวจ{ot}", "เราได้ยิน Arun กำลังอัปเดต{o2t}", "ลูกค้าเห็นเรายืนยันการส่งต่องาน"),
    ("ขณะตรวจ{ot} ฉันพบตัวเลขหายไป", "ขณะอัปเดต{o2t} {p}โทรหาลูกค้า", "เมื่อยืนยันการส่งต่องาน เราปิดทิกเก็ต"),
    ("เราต้องการข้อมูลเกี่ยวกับ{ot}", "มีโน้ตสองสามรายการใน{o2t}", "เราต้องการการอัปเดตหลายรายการก่อนการปล่อยงาน"),
    ("เราต้องการข้อมูลจาก{ot}", "ใน{o2t}มีความคิดเห็นมากแค่ไหน", "กรุณาส่งข้อมูลหนึ่งชิ้นเกี่ยวกับการปล่อยงาน"),
    ("ฉันต้องการสำเนาของ{ot}", "{p}เก็บโน้ตบางส่วนสำหรับ{o2t}", "เราต้องการข้อมูลบางส่วนก่อนการปล่อยงาน"),
    ("ฉันเปิด{ot}หนึ่งรายการ แล้วตรวจรายการนั้นภายหลัง", "{p}เขียน{o2t}หนึ่งรายการ และรายการนั้นพร้อมแล้ว", "เราเห็นแผนปล่อยงานหนึ่งแผน และแผนนั้นดูมีประโยชน์"),
    ("{ot}อยู่บนโต๊ะ", "กรุณาเปิด{o2t}ที่เราคุยกัน", "ดวงอาทิตย์ส่องผ่านหน้าต่างสำนักงานระหว่างการส่งต่องาน"),
    ("{p}อยู่ที่ทำงานและกำลังตรวจ{ot}", "หลังเลิกงาน {p}ไปสำนักงานเพื่ออัปเดต{o2t}", "สำนักงานอยู่ใกล้สถานี"),
    ("ลูกค้าใช้{ot}ที่ทำงาน", "ลูกค้าที่อยู่ในการส่งต่องานนี้ต้องการ{ot}", "รายงานช่วยทีม แต่รายงานของวันนี้ต้องตรวจ"),
    ("โทรศัพท์อยู่ข้าง{o2t}", "เสือเป็นตัวอย่างที่ดีเมื่อพูดถึง{ot}", "คนมีฐานะควรสนับสนุนทีมปล่อยงาน"),
    ("Rin ตรวจ{ot}ที่กรุงเทพฯ", "Arun อัปเดต{o2t}ที่ประเทศไทย", "เส้นทางแปซิฟิกกระทบตารางปล่อยงาน"),
    ("ธนาคารโลกเผยแพร่รายงานเกี่ยวกับ{ot}", "Google แชร์{o2t}กับทีม", "ฉันอ่าน Financial Times ก่อนโทรเรื่องการปล่อยงาน"),
    ("{ot}หนึ่งรายการมีสามแท็บ", "{o2t}มีสองงาน", "{p}อัปเดต{ot}หลายรายการและโน้ตก่อนอาหารกลางวัน"),
    ("{ot}แสดงรายงานยอดขาย", "{p}บันทึก{o2t}ไว้ในโฟลเดอร์โครงการ", "เราตรวจตารางปล่อยงานหลังอาหารกลางวัน"),
    ("แดชบอร์ดของลูกค้าพร้อมแล้ว", "ชื่อของ{o2t}ชัดเจน", "ตารางปล่อยงานเปลี่ยนวันนี้"),
    ("ฉันตรวจ{ot}ด้วยตัวเอง", "ฉันส่ง{o2t}ให้ตัวเอง", "เราเตรียมตัวเองสำหรับการส่งต่องานให้ลูกค้า"),
    ("ฉันตรวจ{ot}ด้วยตัวเองโดยลำพัง", "{p}อัปเดต{o2t}ด้วยตัวเอง", "เราทำการส่งต่องานให้เสร็จด้วยตัวเอง"),
    ("มี{ot}อยู่บนหน้าจอ มันชัดเจน", "มี{o2t}สองรายการในโฟลเดอร์ ถึงเวลาตรวจแล้ว", "มีความล่าช้าในการปล่อยงาน มันน่าหงุดหงิด"),
    ("คุณต้องการความช่วยเหลือเกี่ยวกับ{ot}ไหม", "คุณมีคำถามเกี่ยวกับ{o2t}ไหม", "เราไม่มีการอัปเดตใดๆ สำหรับการปล่อยงาน"),
    ("ไม่มีการอัปเดตของ{ot}", "ไม่มีตัวเลือกใดใน{o2t}ที่เสร็จ", "ไม่มีใครอนุมัติแผนปล่อยงาน"),
    ("เรามีรายงานหลายฉบับและข้อมูลมากเกี่ยวกับ{ot}", "เวลามีน้อยแต่งานส่งต่องานมีมาก", "เรามีผลทดสอบสองสามรายการและความช่วยเหลือมากมายสำหรับการปล่อยงาน"),
    ("ข้อมูลของ{ot}ทั้งหมดเป็นปัจจุบัน", "เวลามาถึงส่วนใหญ่พร้อมแล้ว", "ไม่มีไฟล์ปล่อยงานใดเสร็จสมบูรณ์"),
    ("แผน{ot}อย่างใดอย่างหนึ่งใช้ได้สำหรับสาธิต", "หนึ่งใน{o2t}สองรายการก็ใช้ได้", "แผนใดแผนหนึ่งใช้ได้กับการปล่อยงาน"),
    ("ข้อมูลของ{ot}ทั้งหมดพร้อมแล้ว", "ทุก{o2t}มีผู้รับผิดชอบ", "เราตรวจแผนปล่อยงานทั้งแผน"),
    ("{ot}แต่ละรายการมีผู้รับผิดชอบแยกกัน", "ทุก{o2t}ต้องมีวันที่", "ไฟล์ปล่อยงานแต่ละไฟล์มีป้ายกำกับ"),
    ("นักวิเคราะห์ที่ตรวจ{ot}เป็นผู้นำสาย", "{o2t}ที่{p}อัปเดตพร้อมแล้ว", "ไฟล์ปล่อยงานที่เราทดสอบปลอดภัย"),
    ("{ot}ที่ฉันตรวจพร้อมแล้ว", "{o2t}ที่{p}อัปเดตชัดเจน", "ไฟล์ปล่อยงานที่เราทดสอบผ่าน"),
    ("นักวิเคราะห์ที่มี{ot}ซึ่งฉันตรวจโทรมา", "ลูกค้าที่{p}โทรหาอนุมัติ{o2t}", "สำนักงานที่เราลงนามเรื่องการปล่อยงานอยู่ใกล้"),
    ("{p}ซึ่งตรวจ{ot}เป็นผู้นำสาย", "{o2t}ซึ่ง Arun อัปเดตพร้อมแล้ว", "กรุงเทพฯ ซึ่งทีมปล่อยงานทำงานอยู่คึกคัก"),
    ("{ot}ซึ่งดูง่ายมีข้อมูลซับซ้อน", "{p}ซึ่งปกติอัปเดต{o2t}ไม่อยู่วันนี้", "แผนปล่อยงานซึ่ง Arun อนุมัติยังต้องมีวันที่"),
    ("{ot}ที่ตรวจเมื่อวานพร้อมแล้ว", "{o2t}ที่{p}อัปเดตชัดเจน", "ไฟล์ปล่อยงานที่ทีมทดสอบผ่าน"),
    ("{ot}ทำให้สับสน ดังนั้นฉันจึงสับสน", "{o2t}ทำให้สบายใจ และ{p}รู้สึกสบายใจ", "การปล่อยงานที่ล่าช้าน่าหงุดหงิด และลูกค้าก็หงุดหงิด"),
    ("{ot}ใหม่ชัดเจน", "{o2t}ฉบับละเอียดดูมีประโยชน์", "แผนปล่อยงานใช้งานได้จริงและพร้อม"),
    ("{ot}ชัดเจน และ{p}อธิบายได้ชัดเจน", "{o2t}สมบูรณ์ และ Arun ตรวจอย่างรอบคอบ", "แผนปล่อยงานมีประโยชน์ และเราใช้ได้อย่างมีประสิทธิภาพ"),
    ("{p}ตรวจ{ot}ได้ดี", "Arun อัปเดต{o2t}อย่างรวดเร็วและทำงานหนัก", "เรามาถึงช้า แต่การปล่อยงานแทบไม่ล่าช้า"),
    ("{ot}ชัดเจนมากจนทุกคนใช้ได้", "มันเป็น{o2t}ที่มีประโยชน์มากจนเราแชร์", "การปล่อยงานประสบความสำเร็จมากจนลูกค้าโทรมา"),
    ("{ot}ชัดเจนพอที่จะแชร์", "{o2t}ยาวเกินกว่าจะอ่านเร็วๆ", "การปล่อยงานเร็วพอที่จะทันกำหนดส่ง"),
    ("{ot}ค่อนข้างชัดเจน", "{o2t}มีประโยชน์มากสำหรับทีม", "แผนปล่อยงานค่อนข้างซับซ้อนแต่ใช้งานได้พอสมควร"),
    ("{ot}ใหม่ชัดเจนกว่าของเก่า", "{o2t}มีประโยชน์กว่าอีเมล", "การปล่อยงานครั้งนี้เร็วกว่าเดิม"),
    ("{ot}ใหม่ชัดเจนกว่าของเก่ามาก", "{o2t}มีประโยชน์กว่าข้อความแชตมาก", "วันนี้การปล่อยงานเร็วขึ้นบ้างไหม"),
    ("{ot}ชัดเจนพอๆ กับรายงาน", "{o2t}ไม่ยาวเท่าฉบับเก่า", "{p}ทำงานละเอียดเท่า Arun ในการปล่อยงาน"),
    ("นี่คือ{ot}ที่ชัดเจนที่สุดในทีม", "{o2t}เป็นเอกสารที่มีประโยชน์ที่สุดวันนี้", "นี่คือการปล่อยงานที่เร็วที่สุดปีนี้"),
    ("เราตรวจ{ot}ในห้องประชุมก่อนอาหารกลางวัน", "{p}อัปเดต{o2t}ที่สำนักงานหลังโทร", "เราส่งรายงานปล่อยงานให้ลูกค้าวันศุกร์"),
    ("{p}มักตรวจ{ot}ก่อนอาหารกลางวัน", "Arun อัปเดต{o2t}อย่างรอบคอบแล้ว", "เรายืนยันการปล่อยงานอย่างรวดเร็ว"),
    ("ฉันยังต้องการ{ot}", "{p}ไม่ต้องการ{o2t}อีกต่อไป", "เรายืนยันการปล่อยงานแล้วหรือยัง เราตรวจแล้ว"),
    ("แม้แต่ลูกค้าก็ตรวจแดชบอร์ด", "{p}ยังอัปเดต{o2t}แม้ในวันอาทิตย์", "เรายืนยันแม้แต่รายละเอียดเล็กที่สุดของ{ot}"),
    ("แม้จะล่าช้า เราก็ตรวจ{ot}", "แม้จะอัปเดต{o2t}ช้า {p}ก็โทรหาลูกค้า", "แม้การปล่อยงานเสี่ยง เราก็ส่งมัน"),
    ("นำสำเนา{ot}ไปเผื่อเครือข่ายล่ม", "{p}บันทึก{o2t}ไว้เผื่อลูกค้าถาม", "เราจะพกไฟล์สำรองเผื่อการปล่อยงานหยุด"),
    ("ถ้า{ot}ยังไม่พร้อม เราจะเลื่อนสาย", "ตราบใดที่{o2t}ชัดเจน เราก็เดินหน้าต่อได้", "เราจะปล่อยอัปเดตเมื่อการทดสอบผ่าน"),
    ("ขณะที่ฉันตรวจ{ot} {p}ก็โทรมา", "ขณะที่ Arun อัปเดต{o2t} ลูกค้าก็รอ", "เพราะการปล่อยงานล่าช้า เราจึงเปลี่ยนตาราง"),
    ("{ot}ดูเหมือนรายงาน", "{p}ทำหน้าที่เป็นผู้ดูแล{o2t}", "อย่างที่คุยกัน การปล่อยงานพร้อมแล้ว"),
    ("{p}พูดราวกับว่ารู้จัก{ot}", "{o2t}ดูราวกับว่าพร้อมแล้ว", "Arunทำเหมือนว่าการตรวจ{ot}เริ่มแล้ว"),
    ("ระหว่างการส่งต่องาน เราตรวจ{ot}", "{p}ทำงานกับ{o2t}เป็นเวลาสองชั่วโมง", "ขณะที่เราทดสอบการปล่อยงาน ลูกค้าก็รอ"),
    ("ฉันจะตรวจ{ot}ภายในเที่ยง", "{p}จะรอจนกว่า{o2t}จะพร้อม", "เมื่อถึงเวลาปล่อยงาน ลูกค้าจะได้รับรายงานแล้ว"),
    ("เราตรวจ{ot}ตอนเก้าโมง", "{p}อัปเดต{o2t}วันจันทร์", "การปล่อยงานเริ่มในเดือนกันยายน"),
    ("เราทำ{ot}เสร็จตรงเวลา", "{p}ส่ง{o2t}ทันเวลาสำหรับสาย", "ตอนจบการปล่อยงานเราเข้าใจปัญหา และสุดท้ายลูกค้าก็เห็นด้วย"),
    ("ไฟล์อยู่ในโฟลเดอร์{ot}", "{p}อยู่ที่โต๊ะ{o2t}", "โน้ตอยู่บนกระดานข้าง{o2t}"),
    ("เราพบกันในพื้นที่{ot}", "{p}วาง{o2t}ไว้ที่โต๊ะ{ot}", "ทีมปล่อยงานทำงานที่ 20 Market Street"),
    ("ฉันตรวจ{ot}บนรถไฟ", "{p}ตรวจ{o2t}ที่สถานี", "เราคุยเรื่องการปล่อยงานในแท็กซี่"),
    ("ฉันไปสำนักงานลูกค้าเพื่อตรวจ{ot}", "{p}มาถึงโต๊ะ{o2t}", "เราย้ายเข้าไปในห้องปล่อยงาน"),
    ("ฉันอยู่ที่ทำงานและกำลังตรวจ{ot}", "{p}อยู่บ้านกับ{o2t}", "Arunเข้าเวรระหว่างการปล่อยงาน"),
    ("ฉันส่ง{ot}ทางอีเมล", "{o2t}ถูกเตรียมโดย{p}", "เราต้องทำการปล่อยงานเสร็จภายในวันศุกร์"),
    ("เหตุผลของการตรวจ{ot}ชัดเจน", "{p}อธิบายสาเหตุของความล่าช้าใน{o2t}", "เราพบทางแก้ปัญหาการปล่อยงาน"),
    ("ฉันรับผิดชอบ{ot}", "{p}สนใจ{o2t}", "เราพร้อมสำหรับการปล่อยงาน"),
    ("ฉันกลัวการสูญเสียข้อมูลใน{ot}", "{p}เก่งเรื่องการอัปเดต{o2t}", "เราพอใจกับผลการปล่อยงาน"),
    ("ฉันฟังลูกค้าเรื่องแผนมื้ออาหารขณะทบทวน{ot}", "{p}คุยกับเชฟเรื่อง{o2t}", "เราโบกมือให้ผู้ขายเมื่อของมาส่ง"),
    ("เราคุยกันเรื่อง{ot}", "{p}ขอ{o2t}", "Arunดูแลการปล่อยงาน"),
    ("เราคิดเรื่อง{ot}", "{p}คิดเกี่ยวกับ{o2t}", "Arunเตือนเราเรื่องวันปล่อยงาน"),
    ("ลูกค้าร้องเรียนเรื่องข้อผิดพลาดใน{ot}", "{p}จ่ายค่าบริการใน{o2t}", "เราพึ่งพาปฏิทินปล่อยงาน"),
    ("เราเชื่อในข้อมูลจาก{ot}", "{p}ใส่{o2t}ลงในโฟลเดอร์", "Arunเห็นด้วยกับทีมปล่อยงาน"),
    ("กรุณาค้นหา{ot}ก่อนโทร", "{p}ดำเนินการทบทวน{o2t}", "เราโทรกลับหลังการปล่อยงาน"),
    ("กรุณาเข้าสู่ระบบบัญชีก่อนเปิดดู{ot}", "{p}เช็กเอาต์จากโรงแรมหลังอ่าน{o2t}", "ลูกค้าเข้าสู่ระบบ และ Arun ออกจากระบบหลังการส่งต่องาน"),
    ("กรุณาหาสาเหตุที่{ot}ล่าช้า", "{p}ชี้ข้อผิดพลาดใน{o2t}", "เราหาทางออกสำหรับตารางปล่อยงาน"),
    ("เปิดอุปกรณ์ก่อนตรวจ{ot}", "{p}ปิดการแจ้งเตือนโทรศัพท์หลังอ่าน{o2t}", "เราเริ่มเซิร์ฟเวอร์ปล่อยงาน"),
    ("กรุณาทำงานประชุมต่อหลังตรวจ{ot}", "{p}ถอดป้ายเก่าออกจาก{o2t}", "เราดำเนินงานต่อหลังการปล่อยงานล่าช้า"),
    ("กรุณาเร่งการรีเฟรช{ot}", "{p}จดรายละเอียด{o2t}", "เราลดความล่าช้าของการปล่อยงาน"),
    ("เราต้องเพิ่มความเข้มงวดในการตรวจ{ot}", "{p}ใช้วงเงิน{o2t}จนหมด", "เราสรุปการประชุมปล่อยงาน"),
    ("กรุณาเดินไปที่หน้าจอ{ot}", "{p}หยิบ{o2t}ขึ้นมา", "เราหยิบเอกสารปล่อยงานขึ้นมา"),
    ("กรุณาส่ง{ot}กลับมาภายในวันศุกร์", "{p}ถอยห่างจากกระบวนการส่งต่องานเดิม", "ฉันโทรกลับหาลูกค้าหลังการปล่อยงานและทบทวน{o2t}"),
]

def thai_examples(i, t):
    if i < 1 or i > len(THAI_EXAMPLE_TEMPLATES):
        raise AssertionError(i)
    values = {
        'p': t['person'], 'ot': t['obj_th'], 'o2t': t['obj2_th'],
        'vt': t['verb_th'], 'st': t['scenario_th'], 'plt': t['place_th'],
        'tmt': t['time_th'],
    }
    return [template.format(**values) for template in THAI_EXAMPLE_TEMPLATES[i - 1]]

def build():
    lessons=[]
    for i,title in enumerate(titles,1):
        t=topic(i); family_key,family_th=family(i)
        clean=' '.join(title.split())
        pattern=PATTERNS[i-1]
        ex=examples(i)
        words=vocab(i)
        context_keywords=[bare(t['obj']).split()[0], bare(t['obj2']).split()[0]]
        joined_examples=' '.join(x['en'].casefold() for x in ex)
        if sum(1 for keyword in context_keywords if keyword.casefold() in joined_examples) < 2:
            raise AssertionError(f'lesson {i} examples do not contain two context keywords: {context_keywords} / {joined_examples}')
        goal=f"ใช้ {clean} เพื่อสื่อสารเรื่อง{t['scenario_th']} ในงานและชีวิตประจำวันได้ชัดเจน"
        explanation=(f"เริ่มจากความหมายที่ต้องการสื่อ แล้วเลือกโครงสร้างนี้: {pattern} "
                     f"สังเกตเวลา ผู้กระทำ และลำดับคำในตัวอย่าง ก่อนเปลี่ยนคำศัพท์ให้เข้ากับบริบทของตนเอง")
        lesson={
          'id':f'ebook-{i:03d}','unit_id':str(i),'ordinal':i,'title':clean,'context_keywords':context_keywords,
          'goal_th':goal,'explanation_th':explanation,'pattern':pattern,'examples':ex,
          'vocabulary':words,'quiz':make_quiz(i,ex,words,pattern),'shadowing':[x['en'] for x in ex],
          'speaking':{'prompt_en':f"Role-play {t['scenario']} and use the lesson pattern with {words[0]['term']}, {words[1]['term']}, and {words[2]['term']}.",
                       'prompt_th':f"จำลองสถานการณ์{t['scenario_th']} โดยใช้โครงสร้างบทนี้กับคำศัพท์สามคำแรก",
                       'target_vocabulary':['v-01','v-02','v-03'],
                       'success_criteria_th':['ใช้รูปกริยาและลำดับคำของบทนี้ถูกต้อง','สื่อสารสถานการณ์จริงให้คู่สนทนาเข้าใจ','ใช้คำศัพท์เป้าหมายอย่างน้อยสองคำ']},
          'listening':{'prompt_en':f"Listen to a short {bare(t['scenario'])} update, identify the lesson pattern, and repeat one key line.",
                       'prompt_th':f"ฟังข้อความสั้นเกี่ยวกับ{t['scenario_th']} จับโครงสร้างของบท แล้วพูดประโยคสำคัญซ้ำ",
                       'target_vocabulary':['v-04','v-05','v-06'],
                       'success_criteria_th':['จับใจความและระบุโครงสร้างของบทได้','พูดซ้ำโดยรักษาความหมายและจังหวะ','ใช้คำศัพท์เป้าหมายอย่างน้อยสองคำ']},
          'concept_tags':[family_key,clean,t['scenario'].replace(' ','-'),f'unit-{i}']
        }
        lessons.append(lesson)
    duplicate_counts=Counter(x['en'].casefold() for lesson in lessons for x in lesson['examples'])
    assert max(duplicate_counts.values()) == 1, max(duplicate_counts.items(), key=lambda item:item[1])
    course={'id':'learn-ebook','title':'Learn Ebook · Practical English Grammar','version':'2026-09-20.v1',
             'reference':{'id':'grammar-in-use','title':'Optional grammar reference metadata','unit_count':145},'lessons':lessons}
    return course

def audit_course(course):
    """Run deterministic corpus checks before writing the embedded artifact."""
    lessons = course.get('lessons', [])
    assert len(lessons) == 145, f"lesson count = {len(lessons)}, want 145"
    assert sum(len(x.get('vocabulary', [])) for x in lessons) == 1450
    assert sum(len(x.get('quiz', [])) for x in lessons) == 725
    assert sum(len(x.get('shadowing', [])) for x in lessons) == 435
    banned = (
        'placeholder', 'word1', 'vocab-1', 'example sentence', 'fixture',
        'we discussed the', 'the the', 'some the', 'termss',
        'ประโยคนี้ใช้ในสถานการณ์', 'เราพูดถึง',
    )
    awkward_english = (
        re.compile(r'\bnone of the (?:options|terms|files|notes|profiles|times)\b[^.?!]*\b(?:is|was)\b', re.I),
        re.compile(r'\b(?:Unless|As long as|Until)\s+The\b'),
        re.compile(r'\bEven\s+[A-Z][a-z]+(?:\s|$)'),
    )
    global_examples = []
    choice_positions = {'quiz-01': [], 'quiz-02': [], 'quiz-04': []}
    for i, lesson in enumerate(lessons, 1):
        expected = examples(i)
        if lesson.get('examples') != expected:
            raise AssertionError(f'lesson {i} examples differ from its explicit unit contract')
        examples_en = [x['en'] for x in lesson['examples']]
        if len(examples_en) != 3 or len(set(x.casefold() for x in examples_en)) != 3:
            raise AssertionError(f'lesson {i} repeats an example')
        if lesson.get('shadowing') != examples_en:
            raise AssertionError(f'lesson {i} shadowing does not copy examples verbatim')
        thai = [x['th'] for x in lesson['examples']]
        if any(not re.search(r'[\u0e00-\u0e7f]', text) for text in thai):
            raise AssertionError(f'lesson {i} has an example without Thai meaning')
        terms = [word['term'].strip().casefold() for word in lesson['vocabulary']]
        if len(terms) != 10 or len(set(terms)) != 10:
            raise AssertionError(f'lesson {i} vocabulary is not exactly ten unique terms')
        global_examples.extend(value.casefold() for value in examples_en)
        for field in ('title', 'goal_th', 'explanation_th', 'pattern'):
            if any(token in lesson.get(field, '').casefold() for token in banned):
                raise AssertionError(f'lesson {i} contains a banned placeholder in {field}')
        for item in lesson['examples']:
            if any(token in item['en'].casefold() or token in item['th'].casefold() for token in banned):
                raise AssertionError(f'lesson {i} contains an awkward example template')
            if any(pattern.search(item['en']) for pattern in awkward_english):
                raise AssertionError(f'lesson {i} contains an awkward English construction: {item["en"]}')
        for word in lesson['vocabulary']:
            for field in ('term', 'meaning_th', 'example_en', 'example_th'):
                if any(token in word[field].casefold() for token in banned):
                    raise AssertionError(f'lesson {i} contains an awkward vocabulary template')
        quiz_by_id = {item['id']: item for item in lesson['quiz']}
        for quiz_id in ('quiz-01', 'quiz-02', 'quiz-04'):
            item = quiz_by_id[quiz_id]
            options = item.get('options', [])
            if len(options) != 3 or len(set(option.casefold() for option in options)) != 3:
                raise AssertionError(f'lesson {i} {quiz_id} needs three unique choices')
            if len(item.get('answers', [])) != 1 or item['answers'][0] not in options:
                raise AssertionError(f'lesson {i} {quiz_id} must have exactly one accepted choice')
            choice_positions[quiz_id].append(options.index(item['answers'][0]))
        if set(quiz_by_id['quiz-01']['options']) != set(examples_en):
            raise AssertionError(f'lesson {i} quiz-01 options are not the three lesson examples')
        if set(quiz_by_id['quiz-02']['options']) != set(examples_en):
            raise AssertionError(f'lesson {i} quiz-02 options are not the three lesson examples')
        if set(quiz_by_id['quiz-04']['options']) != set(terms[:3]):
            raise AssertionError(f'lesson {i} quiz-04 options are not the first three vocabulary terms')
    duplicates = [item for item, count in Counter(global_examples).items() if count > 1]
    if duplicates:
        raise AssertionError(f'duplicate examples across lessons: {duplicates[:3]}')
    for quiz_id, positions in choice_positions.items():
        if set(positions) != {0, 1, 2}:
            raise AssertionError(f'{quiz_id} correct-choice positions are not rotated: {Counter(positions)}')
    return {
        'lessons': len(lessons),
        'vocabulary': sum(len(x['vocabulary']) for x in lessons),
        'quiz_items': sum(len(x['quiz']) for x in lessons),
        'shadowing_lines': sum(len(x['shadowing']) for x in lessons),
    }

def main():
    course = build()
    totals = audit_course(course)
    out = root/'internal/ebook/learn_ebook_v1.json'
    out.write_text(json.dumps(course, ensure_ascii=False, indent=2)+'\n')
    print(out)
    print(' '.join(f'{key}={value}' for key, value in totals.items()))
    for n in (1,20,73,100,145):
        l=course['lessons'][n-1]
        print('SAMPLE',n,l['title'],'|',l['pattern'])
        print(' goal:',l['goal_th'])
        print(' examples:', ' / '.join(x['en'] for x in l['examples']))
        print(' vocab:', ', '.join(x['term'] for x in l['vocabulary']))
        print(' speaking:', l['speaking']['prompt_en'])

if __name__ == '__main__':
    main()
