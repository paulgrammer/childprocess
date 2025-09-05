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
    "command": "ffprobe",
    "args": ["-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", "http://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ForBiggerBlazes.mp4"],
    "working_dir": "",
    "webhook_url": "https://webhook.site/4336c3b9-0464-4b40-88c1-21435d40fed1"
  }'
```


