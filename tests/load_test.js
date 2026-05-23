import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
    stages: [
        { duration: '5s', target: 50 },  // Ramp up to 50 virtual users
        { duration: '15s', target: 50 }, // Hold at 50 virtual users
        { duration: '5s', target: 0 },   // Ramp down
    ],
    thresholds: {
        http_req_duration: ['p(99)<50'], // 99% of requests must complete below 50ms
        http_req_failed: ['rate<0.01'],  // Less than 1% failure rate
    },
};

const TOKEN = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjIwOTUxNDQwNDksInN1YiI6Ims2LWxvYWQtdGVzdCJ9.7NQG8ctJx_my0J_AUYZq88AaIuXznYgWjMrI8qEDdNo';

export default function () {
    const params = {
        headers: {
            'Authorization': `Bearer ${TOKEN}`,
        },
    };

    // We will test the main route which distributes to backend1 and backend2
    const res = http.get('http://localhost:8080/api/orders', params);

    check(res, {
        'is status 200': (r) => r.status === 200,
        'has server id': (r) => r.headers['X-Server-Id'] !== undefined,
    });

    // Small sleep to simulate realistic user behavior and prevent complete CPU saturation
    sleep(0.01);
}
