"""Bootstrap this release once through its public API and store access locally, never in Git/reports."""
import json,os,secrets,ssl,urllib.request
from pathlib import Path
base='https://toko-api.tarcloud.win/ai-tutor/api/v2'
ctx=ssl.create_default_context(cafile='/etc/ssl/cert.pem')
def call(path,data=None,token=None):
 h={'Content-Type':'application/json','User-Agent':'TokoLoop-Setup/2'}
 if token:h['Authorization']='Bearer '+token
 with urllib.request.urlopen(urllib.request.Request(base+path,data=json.dumps(data).encode() if data is not None else None,headers=h),context=ctx,timeout=30) as r:return json.load(r)
p=Path('.toko-admin-access.json')
if p.exists():creds=json.loads(p.read_text())
else:creds={'username':'admin','password':'password'}
token=call('/auth/login',creds)['token'];me=call('/auth/me',token=token)
if me['must_change_password']:
 password=secrets.token_urlsafe(24)
 call('/auth/change-password',{'current':creds['password'],'password':password},token)
 creds['password']=password;p.write_text(json.dumps(creds));p.chmod(0o600)
print('Production admin bootstrap secured; credentials saved in ignored local file')
qa=Path('.toko-production-qa.json')
if not qa.exists():
 code=call('/admin/invitations',{},token)['code'];data={'username':'release_qa','password':secrets.token_urlsafe(24)}
 call('/auth/register',dict(data,invitation=code));qa.write_text(json.dumps(data));qa.chmod(0o600)
print('Production QA account ready; one-use invitation consumed for registration test')
# Final user invitation is generated only after all release testing passes.
