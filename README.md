# pdf-extract

HTTP service in **Go**: extract text from PDFs (no OCR), optionally stitch pages vertically into a single **PNG** with white-margin cropping. PDF source is either a **public URL** (JSON) or **file upload** (`multipart/form-data`).

Rendering and text extraction use **Poppler** (`pdftotext`, `pdftoppm`) inside the container.

## Quick start (local)

Requirements: Go 1.26+, `poppler` on `PATH`.

```bash
export PUBLIC_BASE_URL=http://localhost:8000
go run ./cmd/pdf-extract
```

## Environment variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `PUBLIC_BASE_URL` | yes | — | Public origin for absolute PNG URLs (no trailing `/`) |
| `PORT` | no | `8000` | HTTP port |
| `UPLOAD_DIR` | no | `uploads` | Temporary PDF directory |
| `OUTPUT_DIR` | no | `outputs` | Temporary PNG directory |
| `MAX_UPLOAD_BYTES` | no | `33554432` | Multipart body limit (32 MiB) |
| `MAX_DOWNLOAD_BYTES` | no | `33554432` | PDF download limit by URL |
| `HTTP_FETCH_TIMEOUT` | no | `120s` | Outgoing HTTP timeout when fetching by URL |
| `FILE_TTL` | no | `1h` | How long before uploaded PDF and PNG are removed |
| `RENDER_DPI` | no | `150` | DPI for `pdftoppm` |

## API

### POST `/v1/process`

Exactly one request body mode.

#### 1) JSON — PDF by URL

`Content-Type: application/json`

```json
{
  "source": { "type": "url", "url": "https://example.com/doc.pdf" },
  "options": {
    "render_image": false,
    "crop_margins": true
  }
}
```

- **`options`** is optional.
- **`render_image`**: default `false` (text only, lower load). If `true`, a PNG is generated.
- **`crop_margins`**: default `true`; only applies when `render_image: true`.

`200` response:

```json
{
  "status": "success",
  "text": "…",
  "image": null
}
```

With `render_image: true`:

```json
{
  "status": "success",
  "text": "…",
  "image": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "url": "https://your-host/v1/files/550e8400-e29b-41d4-a716-446655440000"
  }
}
```

Pages in `text` are separated by a double newline (`\n\n`), as in the previous Python service.

#### 2) Multipart — file

`Content-Type: multipart/form-data`

- Field **`file`**: PDF (required).
- Field **`options`**: JSON string with the same keys as above (optional).

Example:

```bash
curl -sS -X POST "$BASE/v1/process" \
  -F "file=@./document.pdf" \
  -F 'options={"render_image":false}' \
  --max-time 120
```

### GET `/v1/files/{id}`

Serves the PNG for `id` from the `image.id` response. Files are removed after `FILE_TTL`.

### GET `/health` and GET `/v1/health`

Liveness check and file counts in directories (for orientation).

## Errors

Responses use **`application/problem+json`** (RFC 7807) with fields: `type`, `title`, `detail`, `status`.

## Docker

Copy `.env.example` to `.env` and set `PUBLIC_BASE_URL`.

The compose file attaches to an **external** Docker network named `traefik-public` (typical when Traefik already defines that network). For a **local** machine without Traefik, create it once, then start:

```bash
docker network create traefik-public
docker compose up --build -d
```

On a host where Traefik already uses `traefik-public`, skip the `docker network create` step.

The image includes `poppler-utils`.

Optional: [deploy.sh](deploy.sh) is a **maintainer-only** helper for SSH-based deploys to a remote server. It expects `PUBLIC_BASE_URL` plus `REMOTE_USER` and `REMOTE_HOST_IP` in `.env` (see commented lines in `.env.example`).

### Traefik (example)

Point a router at this service (container listens on port `8000`). A typical rule exposes:

- `GET /health` and `GET /v1/health`
- `PathPrefix` `/v1` for the API

Set **`PUBLIC_BASE_URL`** to the public HTTPS origin clients use (e.g. `https://pdf-api.example.com`), matching the router host.

## License

[MIT](LICENSE).
