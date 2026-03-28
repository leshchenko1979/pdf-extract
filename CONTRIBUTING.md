# Contributing

- Run tests from the repo root: `go test ./... -count=1`
- Format: `gofmt -w .` (or your editor’s Go format on save)
- Open pull requests against `main`; describe the change and any deploy or config impact.

CI runs tests and builds the Docker image on relevant pushes to `main`.
