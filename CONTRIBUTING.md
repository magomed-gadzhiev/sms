# Contributing

Thanks for your interest in improving SMS Platform.

## Development Workflow

1. Create a branch from `main`.
2. Keep changes focused and include tests for behavior changes.
3. Run the relevant checks before opening a pull request.
4. Describe the change, motivation, and verification steps in the pull request.

## Backend Checks

```bash
go test ./...
go vet ./...
```

## Frontend Checks

```bash
cd portal-frontend
npm ci
npm run typecheck
npm test
npm run build
```

## Commit Style

Use short, descriptive commit messages. Conventional prefixes are welcome:

- `feat:` for new behavior;
- `fix:` for bug fixes;
- `docs:` for documentation;
- `test:` for tests;
- `chore:` for maintenance.

## Security

Do not include secrets or private infrastructure details in issues, pull requests, logs, screenshots, fixtures, or documentation. See [SECURITY.md](SECURITY.md).
