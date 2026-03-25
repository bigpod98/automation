# automation

Queue-based automation for `jivetalking`, `jivedrop`, and `jivefire` using `libfilerunner-go`.
The worker calls the vendored Go projects directly (no subprocess exec).

The default queue backend is S3. Directory backend remains available as fallback.

## Queues

Each queue uses `<queue_dir>/{input,in-progress,failed}`:

- `jivetalking` (file queue): one audio file per claim.
- `jivedrop_standalone` (directory queue): one audio file + one metadata file (`.json` or `.yaml`/`.yml`).
- `jivedrop_hugo` (directory queue): one audio file + one Hugo markdown file (`.md`).
- `jivefire_standalone` (directory queue): one audio file + one metadata file (`.json` or `.yaml`/`.yml`).

Outputs are written to each queue's configured `output_dir` and are not kept inside queue claims.

## Run

```bash
go run ./cmd/automation --config config.yaml
```

Process a single polling cycle and exit:

```bash
go run ./cmd/automation --config config.yaml --once
```

For directory queues instead of S3, set `queue_backend: directory` and use each queue's `queue_dir`.

## Metadata Contracts

### `jivedrop_standalone`

`cover` must be a URL in standalone mode. The worker downloads it and passes the local temporary file into jivedrop's encoder/tagging flow.

```yaml
title: "Episode Title"
num: 67
cover: "https://example.com/artwork.png"
artist: "Linux Matters"
album: "All Seasons"
date: "2026-03"
comment: "https://linuxmatters.sh/67"
stereo: false
```

### `jivefire_standalone`

```yaml
episode: 67
title: "Episode Title"
channels: 1
bar_color: "#A40000"
text_color: "#F8B31D"
background_image: "background.png"
thumbnail_image: "thumbnail.png"
no_preview: true
encoder: "software"
output: "episode-67.mp4"
```

`background_image` and `thumbnail_image` are resolved relative to the claimed directory when not absolute.
