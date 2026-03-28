# Project overview

- **pdf-extract**: Go HTTP API for PDF text extraction and optional stitched PNG (Poppler: `pdftotext`, `pdftoppm`).
- **Stack**: Go 1.26, Chi router, Docker (Alpine + `poppler-utils`).
- **Deploy**: CI builds and pushes to GHCR; production host uses `docker-compose.yml` + `.env` under `/root/services/pdf-extract`. Optional local sync: [scripts/sync-vds-service.sh](../../scripts/sync-vds-service.sh).

See [key-components.md](key-components.md) and [decisions.md](decisions.md).
