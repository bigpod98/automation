# automation

Queue-based automation runner for `jivetalking`, `jivefire`, and `jivedrop` using `github.com/bigpod98/libfilerunner-go/pkg`.

Default backend is `s3`.

## What it does

- Polls JSON jobs from a configurable `libfilerunner` backend (`directory`, `s3`, `azureblob`).
- Claims one job at a time using backend-native in-progress/failed flows.
- Runs pipeline steps in order:
  1. `jivetalking` (optional, enabled by default)
  2. `jivefire` (optional, enabled by default)
  3. `jivedrop` (optional, enabled by default)
- Writes a `<base>.automation-result.json` file to the configured output directory.
- Automatically moves failed jobs into the configured failed directory.

## Run

Directory backend:

```bash
go run ./cmd/automation \
  --backend directory \
  --input-dir ./queue/input \
  --in-progress-dir ./queue/in-progress \
  --failed-dir ./queue/failed \
  --poll-interval 2s
```

S3 backend:

```bash
go run ./cmd/automation \
  --backend s3 \
  --s3-region eu-west-2 \
  --s3-bucket podcast-automation \
  --s3-input-prefix queue/input \
  --s3-in-progress-prefix queue/in-progress \
  --s3-failed-prefix queue/failed
```

Environment variables are also supported:

- `AUTOMATION_BACKEND`
- `AUTOMATION_INPUT_DIR`
- `AUTOMATION_INPROGRESS_DIR`
- `AUTOMATION_FAILED_DIR`
- `AUTOMATION_S3_REGION`
- `AUTOMATION_S3_BUCKET`
- `AUTOMATION_S3_INPUT_PREFIX`
- `AUTOMATION_S3_INPROGRESS_PREFIX`
- `AUTOMATION_S3_FAILED_PREFIX`
- `AUTOMATION_AZURE_ACCOUNT_URL`
- `AUTOMATION_AZURE_CONTAINER`
- `AUTOMATION_AZURE_INPUT_PREFIX`
- `AUTOMATION_AZURE_INPROGRESS_PREFIX`
- `AUTOMATION_AZURE_FAILED_PREFIX`
- `AUTOMATION_POLL_INTERVAL`
- `JIVETALKING_BIN`
- `JIVEFIRE_BIN`
- `JIVEDROP_BIN`

## Job file format

Place JSON files in the queue location for your selected backend.

```json
{
  "input_audio": "/absolute/path/to/episode.wav",
  "output_dir": "/absolute/path/to/output",
  "episode_number": "123",
  "title": "Kernel Hot Takes",
  "show_title": "Linux Matters",
  "cover_art": "/absolute/path/to/cover.png",
  "jivetalking": {
    "enabled": true,
    "extra_args": []
  },
  "jivefire": {
    "enabled": true,
    "channels": 1,
    "encoder": "auto",
    "no_preview": true,
    "extra_args": []
  },
  "jivedrop": {
    "enabled": true,
    "artist": "Linux Matters",
    "album": "Linux Matters",
    "date": "2026-03-03",
    "comment": "https://linuxmatters.sh",
    "stereo": false,
    "extra_args": []
  }
}
```

Notes:

- `input_audio`, `episode_number`, and `title` are required.
- `cover_art` is required when `jivedrop.enabled` is true.
- If `output_dir` is omitted, outputs are written beside `input_audio`.
- If `jivetalking` runs, downstream steps use `<input>-processed<ext>` as source audio.
