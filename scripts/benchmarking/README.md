# Benchmarking using k6

The frontend service returns a JSON response with the total measured service time (last response reception time - initial request time).

We get this response with k6 benchmarks and store them in a timescaledb database. I use a custom version of xk6-output-timescaledb at https://github.com/KillianCdP/xk6-output-timescaledb to store the different tags I used directly in columns, without having to parse the tags columns which store the tags values in JSON.
