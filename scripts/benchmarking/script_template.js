import http from 'k6/http';
import { check } from 'k6';
import { Trend } from 'k6/metrics';

const url = __ENV.URL || "http://localhost"

const serviceTimeMetric = new Trend('service_time', true);

const scenarios = {
  rps{{RPS}}: {
    executor: 'constant-arrival-rate',
    timeUnit: '1s',
    preAllocatedVUs: 10000,
    startTime: '0s',
    rate: {{RPS}},
    duration: '30s',
  },
}

export const options = {
  scenarios: scenarios,
};

export default function () {
  const res = http.get(url);

  check(res, {
    'status is 200': (r) => r.status === 200,
  });

  const jsonResponse = res.json();
  serviceTimeMetric.add(jsonResponse.serviceTime, {
          topology: jsonResponse.topology,
          cni: jsonResponse.cni,
          traceId: jsonResponse.traceId,
      });
}