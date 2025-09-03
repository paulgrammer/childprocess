## Super API - Generic Command Executor

Run locally:

```bash
go mod tidy
go run ./cmd/api
```

HTTP endpoints:

- POST `/v1/jobs` to queue a command execution job
- GET `/v1/jobs/{id}` to get status
- GET `/healthz` for health check

Example create job:

```bash
curl -s -X POST localhost:8080/v1/jobs \
  -H 'content-type: application/json' \
  -d '{
    "command": "echo",
    "args": ["hello", "world"],
    "working_dir": "",
    "webhook_url": "http://localhost:9090/webhook"
  }'
```


