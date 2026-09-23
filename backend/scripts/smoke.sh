#!/bin/sh
set -eu
base="${API_URL:-http://localhost:8080}"
curl --fail-with-body -sS "$base/readyz"
curl --fail-with-body -sS "$base/api/catalog/meta"
for date in 2026-11-14 2026-12-19; do
  curl --fail-with-body -sS "$base/api/recommendations" -H 'Content-Type: application/json' \
    -d "{\"city\":\"Алматы\",\"category\":\"Ведущий\",\"date\":\"$date\",\"event_format\":\"корпоратив\",\"budget_kzt\":1000000,\"duration_hours\":6,\"language\":\"русский\"}"
done
curl --fail-with-body -sS "$base/api/recommendations" -H 'Content-Type: application/json' \
  -d '{"city":"Алматы","category":"Флорист","date":"2026-11-14","event_format":"свадьба","budget_kzt":300000,"duration_hours":6,"language":"русский"}'
curl --fail-with-body -sS "$base/api/recommendations" -H 'Content-Type: application/json' \
  -d '{"city":"Алматы","category":"Ведущий","date":"2026-11-14","event_format":"корпоратив","budget_kzt":100000}'
curl --fail-with-body -sS "$base/api/recommendations" -H 'Content-Type: application/json' \
  -d '{"city":"Астана","category":"Декоратор","date":"2026-11-14","event_format":"корпоратив","budget_kzt":3000000}'
