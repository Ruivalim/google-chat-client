# Security

The app holds Google OAuth credentials and refresh tokens for its users' accounts, so security problems matter.

## Reporting

Please **don't open a public issue**. Use GitHub's private vulnerability reporting: the [Security tab](https://github.com/Ruivalim/google-chat-client/security/advisories/new) of the repository, "Report a vulnerability". Include what you found, how to reproduce it and what an attacker could do with it.

You'll get an answer within a week. Fixes go into a new release, and the advisory is published once users had the chance to update.

## Scope

In scope: the app's code in this repository, such as how it stores and sends credentials, the local OAuth callback, links and files coming from messages, and the release workflow.

Out of scope: Google's own services and their policies, and an attacker who already controls the user's account or system keyring.

## How credentials are kept

- The OAuth client secret and each account's refresh token stay in the system keyring (Keychain, Credential Manager, Secret Service) and in the Go process; the web UI never receives them.
- `accounts.json` holds no secrets: project client IDs and, per account, name, email, domain and project.
- Sign-in uses the system browser with PKCE and a `state` check, on a callback that listens on `127.0.0.1` only, for three minutes at most.
