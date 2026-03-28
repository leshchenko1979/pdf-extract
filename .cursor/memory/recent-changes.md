# Recent changes

1. **CI/CD**: GitHub Actions mirrors ai-gateway — tests, GHCR push (`main` / `sha-*`), SSH deploy with `docker compose pull && up -d`.
2. **Compose**: Prebuilt `image` from GHCR (`GHCR_IMAGE` / `IMAGE_TAG`) instead of server-side `build` for routine deploys.
3. **Ops**: Added `scripts/sync-vds-service.sh` for bootstrap/refresh of VDS directory; `deploy.sh` retained for emergency tarball+build.
4. **Docs**: README, CONTRIBUTING, `.env.example`, VS Code tasks aligned with the above.
