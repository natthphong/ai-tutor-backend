"""Explicit local QA fixture bootstrap, never intended for production."""
import urllib.request,json,os
from pathlib import Path
BASE='http://localhost:8080/ai-tutor/api/v2'
def call(path,data=None,token=None):
 h={'Content-Type':'application/json'}
 if token:h['Authorization']='Bearer '+token
 with urllib.request.urlopen(urllib.request.Request(BASE+path,data=json.dumps(data).encode() if data is not None else None,headers=h),timeout=55) as r:return json.load(r)
p=Path('.toko-qa.json')
if p.exists():
 creds=json.loads(p.read_text());call('/auth/login',creds);print('Local QA fixture already exists');raise SystemExit
admin=call('/auth/login',{'username':'admin','password':'password'})['token']
call('/auth/change-password',{'current':'password','password':'Toko-Local-Admin-2026!'},admin)
invite=call('/admin/invitations',{},admin)
creds={'username':'qa_learner','password':'Toko-Local-QA-2026!'}
call('/auth/register',dict(creds,invitation=invite['code']))
p.write_text(json.dumps(creds));p.chmod(0o600)
print('Local QA learner created; bootstrap password changed for local DB only')
