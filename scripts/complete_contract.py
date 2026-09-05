"""Add operation contracts. Schemas in contracts/openapi.json are the canonical shared types."""
import json,re
from pathlib import Path
p=Path(__file__).resolve().parents[1]/'contracts/openapi.json';d=json.loads(p.read_text());S=d['components']['schemas']
def ref(n):return {'$ref':'#/components/schemas/'+n}
def arr(n):return {'type':'array','items':ref(n)}
def obj(props,required=None):return {'type':'object','properties':{k:({'type':v} if isinstance(v,str) else v) for k,v in props.items()},'required':list(props) if required is None else required}
S.update(Error=obj({'error':'string'}),OK=obj({'ok':'boolean'}),ID=obj({'id':'string'}),HintLevel={'type':'integer','minimum':0,'maximum':4},SessionState=obj({'stage':'string','step':'integer','hint_level':ref('HintLevel'),'last_pass':'boolean','independent':'integer','live_active':'boolean'},['stage','step','hint_level']),Job=obj({'id':'string','kind':'string','status':{'type':'string','enum':['queued','running','complete','failed']},'result':{},'error':{'type':['string','null']}},['id','kind','status']),JobAccepted=obj({'job_id':'string'}),Hint=obj({'level':ref('HintLevel'),'text':'string'}),TurnResult=obj({'id':'string','feedback':ref('Feedback'),'audio_id':{'type':['string','null']},'independent':'boolean','state':ref('SessionState')},['id','feedback']),ReviewResult=obj({'feedback':ref('Feedback'),'stage':'integer','due_at':'string','rescheduled':'boolean'},['feedback','rescheduled']),Library=obj({'vocabulary':arr('Word'),'patterns':arr('Lesson'),'mistakes':arr('ReviewItem')}),Invitation=obj({'id':'string','code':'string','expires_at':'string'},['id','expires_at']),Ticket=obj({'url':'string','expires_in':'integer'}),TTSResult=obj({'audio_id':'string','job_id':'string'},[]),Input=obj({'request_id':{'type':'string','format':'uuid'},'text':'string','retry_of':'string','hint_level':ref('HintLevel')},['request_id','text']))
S['Session']['properties']['lesson_id']={'type':['string','null']};S['Session']['properties']['scenario_id']={'type':['string','null']}
# These fields are null in PostgreSQL until an audio/retry exists.
for n,keys in [('Attempt',['audio_id','retry_of']),('Turn',['audio_id']),('DailyPlan',['active_session_id'])]:
 for k in keys:S[n]['properties'][k]={'type':['string','null']}
S['Session']['properties']['summary']={'anyOf':[S['Session']['properties']['summary'],{'type':'null'}]}
def route(method,path,out,body=None,status=200,public=False,multipart=False):
 op={'operationId':method+'_'+re.sub(r'[^a-zA-Z0-9]+','_',path).strip('_'),'responses':{str(status):{'description':'Success','content':{'application/json':{'schema':ref(out) if isinstance(out,str) else out}}},**{str(c):{'description':m,'content':{'application/json':{'schema':ref('Error')}}} for c,m in [(400,'Invalid input'),(401,'Login required'),(403,'Forbidden'),(404,'Not found'),(409,'Conflict'),(402,'Budget exceeded'),(429,'Rate limited'),(502,'Gemini unavailable')]}}}
 if public:op['security']=[]
 if '{id}' in path:op['parameters']=[{'name':'id','in':'path','required':True,'schema':{'type':'string'}}]
 if body:
  op['requestBody']={'required':True,'content':{'application/json':{'schema':ref(body) if isinstance(body,str) else body}}}
  if multipart:op['requestBody']['content']['multipart/form-data']={'schema':obj({'request_id':{'type':'string','format':'uuid'},'audio':{'type':'string','format':'binary'},'retry_of':'string','hint_level':ref('HintLevel')},['request_id','audio'])}
 d['paths'].setdefault(path,{})[method]=op
for path in ['health','readiness']:route('get','/'+path,obj({'status':'string','release':'string','ai_configured':'boolean'},['status']),public=True)
route('post','/auth/login',obj({'token':'string','expires_in':'integer'}),obj({'username':'string','password':'string'}),public=True)
route('post','/auth/register','ID',obj({'username':'string','password':'string','invitation':'string'}),201,True)
route('get','/auth/me','User');route('post','/auth/logout','OK');route('post','/auth/change-password','OK',obj({'current':'string','password':'string'}))
route('get','/admin/invitations',arr('Invitation'));route('post','/admin/invitations','Invitation',status=201);route('delete','/admin/invitations/{id}','OK')
route('patch','/profile','OK',obj(S['Profile']['properties'],[]))
for path,out in [('curriculum',arr('Lesson')),('lessons/{id}','Lesson'),('daily-plan','DailyPlan'),('progress','Progress'),('library','Library'),('scenarios',arr('Scenario')),('sessions',arr('Session')),('sessions/{id}','SessionData'),('review',arr('ReviewItem')),('jobs/{id}','Job'),('usage','Usage')]:route('get','/'+path,out)
route('post','/vocabulary','OK','Word',201)
route('post','/scenarios','JobAccepted',obj({'prompt':'string','lesson_id':'string','request_id':'string'},['prompt','request_id']),202)
route('patch','/scenarios/{id}','Scenario','Scenario')
route('post','/sessions','ID',obj({'mode':{'type':'string','enum':['lesson','free','scenario','live','placement']},'lesson_id':'string','scenario_id':'string'},['mode']),201)
route('post','/sessions/{id}/turns','TurnResult','Input',multipart=True)
route('post','/sessions/{id}/hints','Hint',obj({'idea':'string'},[]))
route('post','/sessions/{id}/advance','SessionState');route('post','/sessions/{id}/complete',obj({'attempts':'integer','independent':'integer','mastered':'boolean','message':'string','level':'string'},['attempts','independent','mastered','message']))
route('post','/sessions/{id}/live-ticket','Ticket');route('post','/sessions/{id}/retry','ID',status=201)
route('post','/review/{id}/answer','ReviewResult','Input',multipart=True);route('post','/review/{id}/hint','Hint')
route('post','/audio/tts','TTSResult',obj({'text':'string','voice':'string'},['text']),202)
route('get','/audio/{id}',{'type':'string','format':'binary'});d['paths']['/audio/{id}']['get']['responses']['200']['content']={'audio/wav':{'schema':{'type':'string','format':'binary'}}}
d['paths']['/live']={'get':{'operationId':'live','security':[],'description':'Single-use 30-second ticket, allowed Origin, binary PCM16 16kHz input; JSON Gemini events carry 24kHz PCM output. Heartbeat every 15 seconds.','parameters':[{'in':'query','name':'ticket','required':True,'schema':{'type':'string'}}],'responses':{'101':{'description':'WebSocket upgraded'},'401':{'description':'Invalid or consumed ticket'},'403':{'description':'Origin rejected'}}}}
p.write_text(json.dumps(d,ensure_ascii=False,indent=2)+'\n')
print(len(d['paths']),'API paths')
