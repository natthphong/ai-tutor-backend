import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
    vus: 100, // Number of Virtual Users
    duration: '30s', // Test duration
    gracefulStop: '5s', // รอให้คำขอทั้งหมดเสร็จสิ้นก่อนหยุด

};

export default function () {
    const url = 'http://localhost:8080/api/v1/login';
    const payload = JSON.stringify({
        appCode: 'test',
        username: 'test',
        password: 'test',
    });

    const params = {
        headers: {
            'Content-Type': 'application/json',
        },
    };

    const res = http.post(url, payload, params);

    check(res, {
        'is status 200': (r) => r.status === 200,
        'code is 000': (r) => JSON.parse(r.body).code === '000',
    });

}
