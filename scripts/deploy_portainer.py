"""Deploy only the named Toko app. Private PostgreSQL + durable volumes; preserve previous image/container."""
import http.client,json,os,re,secrets,sys,time,urllib.request,urllib.error,urllib.parse,ssl
E=os.environ;name=E['APP_NAME'];release=E['RELEASE_ID'];image=E['IMAGE_TAG'];port=int(E['EXTERNAL_PORT'])
if not re.fullmatch(r'[a-zA-Z0-9_.-]+',name) or not re.fullmatch(r'[a-zA-Z0-9_.-]+',release):raise SystemExit('Invalid app/release identifier')
base=E['PORTAINER_URL'].rstrip('/')+'/api/endpoints/'+E['ENDPOINT_ID']+'/docker'
ctx=ssl.create_default_context(cafile='/etc/ssl/cert.pem') if os.path.exists('/etc/ssl/cert.pem') else ssl.create_default_context()
def api(method,path,data=None,body=None):
 headers={'X-API-Key':E['PORTAINER_API_KEY']}
 if data is not None:body=json.dumps(data).encode();headers['Content-Type']='application/json'
 if body is not None and data is None:headers['Content-Type']='application/x-tar'
 req=urllib.request.Request(base+path,data=body,method=method,headers=headers)
 try:
  with urllib.request.urlopen(req,timeout=180,context=ctx) as r:b=r.read()
 except urllib.error.HTTPError as e:raise RuntimeError(f'Portainer {method} {path.split("?")[0]} HTTP {e.code}') from None
 try:return json.loads(b) if b else {}
 except:return b
containers=api('GET','/containers/json?all=true');old=next((c for c in containers if '/'+name in c['Names']),None)
for c in containers:
 if c.get('State')=='running' and any(x.get('PublicPort')==port for x in c.get('Ports',[])) and (not old or c['Id']!=old['Id']):raise SystemExit(f'Port {port} belongs to another container. Deployment stopped without changes.')
network='toko-loop-private';labels={'app':'toko-loop','managed-by':'toko-deploy'}
nets=api('GET','/networks');existing=next((x for x in nets if x['Name']==network),None)
if existing and existing.get('Labels',{}).get('app')!='toko-loop':raise SystemExit('Network name is already owned by another app')
if not existing:api('POST','/networks/create',{'Name':network,'Labels':labels,'CheckDuplicate':True})
for volume in ['toko-loop-postgres','toko-loop-audio']:
 existing=next((v for v in (api('GET','/volumes').get('Volumes') or []) if v['Name']==volume),None)
 if existing and existing.get('Labels',{}).get('app')!='toko-loop':raise SystemExit('Volume already owned by another app: '+volume)
 if not existing:api('POST','/volumes/create',{'Name':volume,'Labels':labels})
dbname='toko-loop-db';db=next((c for c in containers if '/'+dbname in c['Names']),None)
if db:
 info=api('GET','/containers/'+db['Id']+'/json')
 if info['Config'].get('Labels',{}).get('app')!='toko-loop':raise SystemExit('Database container belongs to another app')
 dbvars=dict(x.split('=',1) for x in info['Config']['Env'] if '=' in x);password=dbvars['POSTGRES_PASSWORD']
 if not info['State']['Running']:api('POST','/containers/'+db['Id']+'/start')
else:
 password=secrets.token_urlsafe(32)
 api('POST','/images/create?fromImage=postgres&tag=17-alpine')
 db=api('POST','/containers/create?name='+dbname,{'Image':'postgres:17-alpine','Env':['POSTGRES_USER=toko','POSTGRES_PASSWORD='+password,'POSTGRES_DB=toko_loop'],'Labels':labels,'HostConfig':{'NetworkMode':network,'Binds':['toko-loop-postgres:/var/lib/postgresql/data'],'RestartPolicy':{'Name':'unless-stopped'}},'Healthcheck':{'Test':['CMD-SHELL','pg_isready -U toko -d toko_loop'],'Interval':5000000000,'Timeout':3000000000,'Retries':12}})
 api('POST','/containers/'+db['Id']+'/start')
for _ in range(60):
 if api('GET','/containers/'+db['Id']+'/json')['State'].get('Health',{}).get('Status')=='healthy':break
 time.sleep(1)
else:raise SystemExit('New database did not become healthy; existing app untouched')
print('New Toko database ready; original database untouched',flush=True)
with open(sys.argv[1],'rb') as f:api('POST','/images/load?quiet=1',body=f.read())
key=E.get('GEMINI_API_KEY')
if not key:raise SystemExit('Set GEMINI_API_KEY in .env.deploy; no AI credentials were uploaded')
env=['DATABASE_URL=postgres://toko:'+password+'@'+dbname+':5432/toko_loop?sslmode=disable','GEMINI_API_KEY='+key,'PUBLIC_BACKEND_URL='+E['PUBLIC_BACKEND_URL'],'ALLOWED_ORIGINS='+E['ALLOWED_ORIGINS'],'RELEASE_ID='+release]
def specification(publish):
 host={'NetworkMode':network,'Binds':['toko-loop-audio:/data/audio'],'RestartPolicy':{'Name':'unless-stopped'},'CapDrop':['ALL'],'SecurityOpt':['no-new-privileges:true']}
 if publish:host['PortBindings']={'8080/tcp':[{'HostPort':str(port)}]}
 return {'Image':image,'Env':env+([] if publish else ['TOKO_READINESS_ONLY=true']),'Labels':dict(labels,release=release),'ExposedPorts':{'8080/tcp':{}},'HostConfig':host}
def wait_ready(cid):
 for _ in range(60):
  state=api('GET','/containers/'+cid+'/json')['State']
  if state.get('Health',{}).get('Status')=='healthy':return True
  if not state.get('Running'):return False
  time.sleep(1)
 return False
candidate=api('POST','/containers/create?name='+name+'-candidate-'+release,specification(False))['Id']
api('POST','/containers/'+candidate+'/start')
if not wait_ready(candidate):
 api('POST','/containers/'+candidate+'/stop?t=5');api('DELETE','/containers/'+candidate)
 raise SystemExit('Candidate readiness failed; old deployment kept running')
api('POST','/containers/'+candidate+'/stop?t=10');api('DELETE','/containers/'+candidate)
# Avoid interrupting an in-flight learner response or Live conversation.
def busy_learning():
 query="SELECT (SELECT count(*) FROM usage WHERE status='reserved' AND created_at>now()-interval '3 minutes') + (SELECT count(*) FROM learning_sessions WHERE state->>'live_active'='true' AND updated_at>now()-interval '90 seconds');"
 eid=api('POST','/containers/'+db['Id']+'/exec',{'AttachStdout':True,'AttachStderr':True,'Cmd':['psql','-U','toko','-d','toko_loop','-tAc',query]})['Id']
 raw=api('POST','/exec/'+eid+'/start',{'Detach':False,'Tty':False})
 if not isinstance(raw,bytes):raise RuntimeError('Unable to check active learner requests')
 chunks=[]
 while len(raw)>=8:
  length=int.from_bytes(raw[4:8],'big');chunks.append(raw[8:8+length]);raw=raw[8+length:]
 return int(b''.join(chunks).decode().strip())
for check in range(31):
 if busy_learning()==0:break
 if check==30:raise SystemExit('Learners are still active; old app kept running. Retry deployment later.')
 if check==0:print('Waiting for active learner requests before switching; existing app stays online',flush=True)
 time.sleep(10)
print('Candidate passed database migration and readiness; switching named app',flush=True)
rollback=name+'-rollback-'+release;new=None;renamed=False;stopped=False
try:
 if old:
  api('POST','/containers/'+old['Id']+'/stop?t=15');stopped=True
  api('POST','/containers/'+old['Id']+'/rename?name='+rollback);renamed=True
 new=api('POST','/containers/create?name='+name,specification(True))['Id'];api('POST','/containers/'+new+'/start')
 if not wait_ready(new):raise RuntimeError('App readiness failed')
 for _ in range(30):
  try:
   with urllib.request.urlopen(urllib.request.Request(E['PUBLIC_BACKEND_URL']+'/ai-tutor/api/v2/health',headers={'User-Agent':'TokoLoop-Deploy/2'}),timeout=8,context=ctx) as r:status=json.load(r)
   if status.get('release')==release:break
   print('HTTPS release check:',status,flush=True)
  except (urllib.error.URLError,TimeoutError) as e:print('HTTPS check pending:',type(e).__name__,str(e),flush=True)
  time.sleep(2)
 else:raise RuntimeError('Public HTTPS did not serve the release')
except Exception:
 if new:
  api('POST','/containers/'+new+'/stop?t=10');api('DELETE','/containers/'+new)
 if old and stopped:
  if renamed:api('POST','/containers/'+old['Id']+'/rename?name='+name)
  api('POST','/containers/'+old['Id']+'/start')
 raise
state={'release':release,'image':image,'container':new,'previous_container':old['Id'] if old else None,'previous_name':rollback if old else None,'public_url':E['PUBLIC_BACKEND_URL']}
with open('.toko-deploy-state.json','w') as f:json.dump(state,f,indent=2)
os.chmod('.toko-deploy-state.json',0o600)
print('Verified HTTPS release '+release+' at '+E['PUBLIC_BACKEND_URL'],flush=True)
