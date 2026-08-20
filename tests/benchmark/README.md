# Local benchmark harness

This directory is test-only. It refuses non-local URLs and requires
`TAPPIX_TEST_ENV=1`.

```sh
export TAPPIX_TEST_ENV=1
export BENCHMARK_DATABASE_URL='postgres://tappix:tappix_local@localhost:5432/tappix_bench?sslmode=disable'
./tests/benchmark/reset.sh
./tests/benchmark/seed.sh 3000
```

`reset.sh` drops only the explicitly named benchmark database schema and then
applies every current `*.up.sql` migration. `seed.sh` creates deterministic
reward definitions/rules; run reset before each snapshot so rows never
accumulate. No production pool, SQL, or runtime configuration is changed.
