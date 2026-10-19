# Contributing

Thanks for wanting to help. [Português](CONTRIBUTING.pt-BR.md).

## Before you start

- For a bug, open an issue with the steps to reproduce, your system and the app version (the release you installed).
- For a feature, open an issue first, so we agree on the scope before you write the code.
- Security problems go through a private report, not an issue: see [SECURITY.md](SECURITY.md).

## Setting up

Follow the Development section of the [README](README.md#development): Go 1.27+, Bun and Node 24+, then `make setup`. `make preview` runs the UI in a browser with no Google account, which covers most UI work.

## Making a change

- New code comes with tests. Cover what actually breaks (invalid input, network errors, concurrency, limits), not only the happy path. Go tests talk to a simulated Google, and UI tests to a simulated MyGo bridge in `e2e/bridge.ts`; follow the existing ones.
- Every text the user sees goes through the catalogs in `locales/`: add the key to `en.json` (the source) and to `pt-BR.json`. A test fails when one catalog misses a key.
- Run `make check` before opening the pull request. It runs the same lint, tests and build as CI.
- If a test fails, assume the code is wrong. If the test itself was wrong, say why in the pull request.
- Keep pull requests small and focused, and explain the reason for the change, not just what changed.

## Style

- Go: `gofmt`, `go vet`. Error messages are in the catalogs, starting in lowercase, as Go errors do; the UI capitalizes them.
- TypeScript and CSS: Biome (`npm run fmt`). Fluent UI components, and the colors of the theme instead of new ones.
- Code comments explain why, not what. Most existing comments are in Portuguese; English is welcome too.

By contributing, you agree that your contribution is licensed under the [MIT License](LICENSE).
