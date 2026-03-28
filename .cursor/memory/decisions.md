# Decisions

- **Server path**: `/root/services/pdf-extract` — same pattern as ai-gateway under `/root/services/`.
- **Secrets**: Reuse GitHub secrets naming (`SSH_*`, `GHCR_PULL_*`) across repos.
- **Image default**: `ghcr.io/leshchenko1979/pdf-extract` in compose (override via `GHCR_IMAGE` for forks).
- **Traefik/Sablier**: Existing compose labels unchanged; only image source moved to GHCR.
- **deploy.sh**: Kept for emergency full-source remote build; not the default path.
