import http from 'k6/http';
import { check, sleep } from 'k6';

// Sustained load against /api/orders while scripts/chaos-demo.sh kills and
// restarts backend1 partway through. Stage timings are intentionally long
// enough to observe the full circuit-breaker cycle configured in
// config.yaml (failure_threshold: 5, timeout: 30s) as well as the health
// checker's 10s poll interval noticing the backend go down and come back.
export const options = {
    stages: [
        { duration: '10s', target: 30 }, // ramp up, both backends healthy
        { duration: '80s', target: 30 }, // hold - chaos-demo.sh kills backend1 ~10s in, restarts it ~50s in
        { duration: '10s', target: 0 },  // ramp down
    ],
    thresholds: {
        // Not pass/fail gates here (some 5xx during the outage is expected) -
        // just visible in the summary for a quick sanity check.
        http_req_failed: ['rate<1'],
    },
};

const TOKEN = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjIwOTUxNDQwNDksInN1YiI6Ims2LWxvYWQtdGVzdCJ9.7NQG8ctJx_my0J_AUYZq88AaIuXznYgWjMrI8qEDdNo';

export default function () {
    const params = {
        headers: {
            'Authorization': `Bearer ${TOKEN}`,
        },
    };

    const res = http.get('http://localhost:8080/api/orders', params);

    check(res, {
        'is status 200': (r) => r.status === 200,
        'has server id': (r) => r.headers['X-Server-Id'] !== undefined,
    });

    sleep(0.1);
}
