// A smoke test, not a capacity test.
//
// Rate limiting is per client, and every virtual user here shares one source
// address, so the whole run counts against a single client's allowance: 120
// requests a minute per path, 10 for sign-in. Raising VUS past a handful stops
// measuring the API and starts measuring the limiter — requests are rejected in
// milliseconds, so latency even looks better.
//
// What this does answer: does a normal client get correct, fast responses under
// steady use. Measuring real capacity needs load generated from many addresses.
import http from "k6/http";
import { check, sleep } from "k6";

export const options={scenarios:{steady:{executor:"constant-vus",vus:Number(__ENV.VUS||2),duration:__ENV.DURATION||"20s"}},thresholds:{http_req_failed:["rate<0.01"],http_req_duration:["p(95)<500","p(99)<1000"],checks:["rate>0.99"]}};
const base=__ENV.BASE_URL||"http://host.docker.internal:8088";
export function setup(){const response=http.post(`${base}/api/v1/auth/login`,JSON.stringify({email:"armat@tappix.kz",password:"Tappix2026!"}),{headers:{"Content-Type":"application/json"}});check(response,{"login succeeds":r=>r.status===200});return{token:response.json("data.accessToken")};}
export default function(data){const params={headers:{Authorization:`Bearer ${data.token}`}};check(http.get(`${base}/api/v1/dashboard`,params),{"dashboard 200":r=>r.status===200});check(http.get(`${base}/api/v1/customers?limit=20`,params),{"customers 200":r=>r.status===200});check(http.get(`${base}/api/v1/analytics/outcomes?days=30`,params),{"analytics 200":r=>r.status===200});sleep(.6);}
