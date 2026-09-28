import http from 'k6/http';
import { check } from 'k6';
import exec from 'k6/execution';

const rate=Number(__ENV.RATE || 2000);
const duration=__ENV.DURATION || '10m';
const prefix=__ENV.RUN_ID || `load-${Date.now()}`;
const urls=(__ENV.URLS || 'http://localhost:8080,http://localhost:8081').split(',');
const key=__ENV.API_KEY || 'local-development-key';
export const options={
 scenarios:{hot:{executor:'constant-arrival-rate',rate,timeUnit:'1s',duration,preAllocatedVUs:200,maxVUs:600}},
 thresholds:{http_req_duration:['p(99)<300'],http_req_failed:['rate==0'],checks:['rate==1'],dropped_iterations:['count==0']},
 summaryTrendStats:['avg','med','p(95)','p(99)','max']
};
export default function(){
 const n=exec.scenario.iterationInTest;
 const peer=2+(n%999999);
 const mode=__ENV.MODE || 'mixed';
 const incoming=mode==='in' || (mode==='mixed' && n%2===0);
 const source=incoming?peer:1;
 const destination=incoming?1:peer;
 const body={transaction_id:`${prefix}-${n}`,source_id:source,destination_id:destination,amount_minor:1};
 const res=http.post(`${urls[n%urls.length]}/transfers`,JSON.stringify(body),{headers:{Authorization:`Bearer ${key}`,'Content-Type':'application/json'},timeout:'2s'});
 check(res,{'approved':r=>r.status===200 && r.json('decision')==='APPROVED'});
}
export function handleSummary(data){return {['reports/load-'+prefix+'.json']:JSON.stringify(data,null,2),stdout:JSON.stringify({run:prefix,metrics:data.metrics},null,2)+'\n'};}
