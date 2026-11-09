import test from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { cohortInsights, validateCohortConfig } from './cohorts.mjs';

const config = { cutoff:'2026-01-01T00:00:00Z', asOf:'2026-02-01T00:00:00Z', observationScope:'synthetic', exclusionsReviewed:true, excludedUserIds:[], excludedWorkspaceIds:[], excludedRunIds:[] };
test('requires explicit UTC boundaries and reviewed exclusions', () => {
 for (const patch of [{cutoff:null},{asOf:'today'},{asOf:'2025-01-01T00:00:00Z'},{exclusionsReviewed:false},{excludedUserIds:["secret@example.com"]},{observationScope:'live-ish'}]) {
  assert.throws(()=>validateCohortConfig({...config,...patch}));
 }
});
test('generates reproducible bounded queries without merging result kinds', () => {
 const specs=cohortInsights(config);
 assert.deepEqual(specs,cohortInsights(config));
 const sql=specs.map(s=>s.query.source.query).join('\n');
 assert.match(sql,/pdi\.person\.properties\.acquisition_channel/);
 assert.match(sql,/signup_kind = 'new_user'/);
 assert.match(sql,/INTERVAL 7 DAY/);
 assert.match(sql,/completed\.run_id = created\.run_id/);
 assert.match(sql,/completed\.user_id = created\.user_id/);
 assert.match(sql,/completed\.workspace_id = created\.workspace_id/);
 assert.match(sql,/imported_local_evidence/);
 assert.match(sql,/hosted_run/);
 assert.match(sql,/invited_activation/);
 assert.match(sql,/unknown/);
 assert.doesNotMatch(sql,/web\.auth\.completed/);
 assert.ok(specs.some(s=>s.name.includes('Observed active users and workspaces')));
});
test('provisioner dry-run requires no credentials and never calls fetch', () => {
 const output=execFileSync(process.execPath,['--input-type=module','-e',`globalThis.fetch=()=>{throw new Error('network forbidden')}; process.argv.push('--dry-run','--config','scripts/posthog/cohort-example.json'); await import('./scripts/posthog/provision-dashboard.mjs');`],{cwd:new URL('../../',import.meta.url),env:{PATH:process.env.PATH},encoding:'utf8'});
 const artifact=JSON.parse(output);
 assert.equal(artifact.dry_run,true);
 assert.equal(artifact.cohort_config.observationScope,'synthetic');
 assert.ok(artifact.insights.length>8);
});

test('generated SQL handles mature cohorts, retries, exact joins, unknowns and exclusions on synthetic events', () => {
 const id=(n)=>`00000000-0000-4000-8000-${String(n).padStart(12,'0')}`;
 const events=[];
 const channels={1:'claude',2:'codex',3:'unknown',4:'claude',5:'claude',6:'codex',7:'unknown',8:'unknown',9:'unknown'};
 const add=(user,event,day,properties={})=>events.push({event,distinct_id:id(user),timestamp:`2026-01-${String(day).padStart(2,'0')}T00:00:00Z`,person:{acquisition_channel:channels[user]},properties:{schema_version:1,...properties}});
 const signup=(user,day,kind='new_user')=>add(user,'product.account.signup_completed',day,{user_id:id(user),signup_kind:kind});
 signup(1,2);signup(1,3);signup(2,3);signup(3,4);signup(4,3,'invited_activation');signup(5,3);signup(6,29);signup(9,4,'legacy-unknown');
 add(8,'web.auth.completed',3); // Existing-user login is not a new account.
 const run=(user,n,createdDay,completedDay,host='unknown',extra={})=>{
  add(user,'product.run.created',createdDay,{run_id:id(n),workspace_id:id(100+user),observed_host:host,...extra});
  add(user,'product.run.completed',completedDay,{run_id:id(n),workspace_id:id(100+user),...extra});
 };
 run(1,201,3,4,'claude');add(1,'product.run.completed',5,{run_id:id(201),workspace_id:id(101)});
 run(2,202,3,10,'codex'); // Exactly seven days after signup is excluded.
 run(4,204,4,5,'claude');run(5,205,4,5,'claude');run(6,206,30,31,'codex');
 run(3,203,5,6,'unknown',{is_sample:true});run(3,213,5,6,'unknown');
 add(3,'product.run.created',5,{run_id:id(207),workspace_id:id(103),observed_host:'claude'});
 add(2,'product.run.completed',6,{run_id:id(207),workspace_id:id(103)}); // Wrong creator UUID must not join.
 add(3,'product.run.completed',6,{run_id:id(208),workspace_id:id(103)}); // Missing creation stays audit-only.
 for(const user of [1,2]) add(user,'product.sdk_report.imported',6,{sdk_report_id:id(300+user),workspace_id:id(100+user),observed_host:channels[user]});
 add(1,'product.sdk_report.imported',7,{sdk_report_id:id(301),workspace_id:id(101),observed_host:'claude'});
 add(3,'product.tryout.claimed',6,{tryout_id:id(403),workspace_id:id(103),observed_host:'unknown'});
 const request=(user,host,status)=>add(user,'cli.command.invoked',6,{observed_host:host,status_code:status,workspace_id:id(100+user)});
 add(10,'product.sdk_report.imported',6,{sdk_report_id:id(310),workspace_id:id(110),observed_host:'codex',surface:'cli'});
 add(11,'product.sdk_report.imported',6,{sdk_report_id:id(311),workspace_id:id(111),observed_host:'codex',surface:'web'});
 request(1,'claude',200);request(1,'claude',200);request(2,'codex',201);request(8,'codex',500);request(8,'unknown',200);
 events.push({event:'cli.command.invoked',distinct_id:'anonymous',timestamp:'2026-01-06T00:00:00Z',properties:{observed_host:'claude',status_code:200}});
 const insights=cohortInsights({...config,excludedUserIds:[id(5)],excludedRunIds:[id(213)]});
 const result=JSON.parse(execFileSync('python3',['scripts/posthog/query-fixture.py'],{cwd:new URL('../../',import.meta.url),input:JSON.stringify({insights,events}),encoding:'utf8'}));
 const rows=result[insights[1].name];
 assert.deepEqual(rows.map(r=>[r.acquisition_channel,r.mature_new_accounts,r.accounts_with_hosted_run,r.accounts_with_imported_evidence]),[['claude',1,1,1],['codex',1,0,1],['unknown',1,0,0]]);
 assert.deepEqual(result[insights[0].name].map(r=>[r.observed_host,r.observed_active_users,r.observed_active_workspaces]),[['claude',1,1],['codex',2,2],['unknown',1,1]]);
 assert.ok(result[insights[2].name].some(r=>r.signup_kind==='invited_activation'&&r.accounts===1));
 assert.ok(result[insights[2].name].some(r=>r.signup_kind==='unknown'&&r.accounts===1));
 assert.ok(result[insights[2].name].some(r=>r.maturity==='immature'&&r.accounts===1));
 assert.ok(result[insights[4].name].some(r=>r.coverage==='unmatched_or_invalid_creation'&&r.entities===2));
 const empty=JSON.parse(execFileSync('python3',['scripts/posthog/query-fixture.py'],{cwd:new URL('../../',import.meta.url),input:JSON.stringify({insights,events:[]}),encoding:'utf8'}));
 assert.deepEqual(empty[insights[1].name],[]); // No mature data is not zero conversion.
});
