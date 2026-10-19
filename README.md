<p align="center"><img src="resources/icon.png" width="96" height="96" alt=""></p>

<h1 align="center">Google Chat Client</h1>

<p align="center">A desktop app for Google Chat with several Google Workspace accounts in one window, including accounts from different companies.</p>

<p align="center"><strong>English</strong> · <a href="README.pt-BR.md">Português</a></p>

Each account keeps its own session, conversations, notifications and drafts. Runs on Linux, Windows and macOS, in English or Portuguese.

> **Independent project.** This is not a Google product and is not affiliated with Google. Google, Google Chat and Google Workspace are trademarks of Google LLC.

## Download

Installers are on the [Releases](https://github.com/Ruivalim/google-chat-client/releases) page:

| System | File |
| --- | --- |
| Linux | `.AppImage` (runs as is), `.deb`, or `.tar.gz` with `install.sh` |
| Windows | `Google Chat Client Setup … amd64.exe` |
| macOS | universal `.dmg` (Apple Silicon and Intel) |

The packages are **not signed** yet. On Windows, SmartScreen warns: click "More info" and "Run anyway". On macOS, Gatekeeper blocks the first launch: open it from System Settings > Privacy & Security > "Open Anyway".

On Linux the app needs GTK 3, WebKitGTK 4.1 and a Secret Service for the keyring (GNOME Keyring, KWallet or KeePassXC). On Debian and Ubuntu: `sudo apt install libwebkit2gtk-4.1-0 libsecret-1-0`. The tray icon uses `libayatana-appindicator3`. Windows 10 needs the [WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/); Windows 11 ships with it.

## Set up Google Cloud

The app ships no built-in credentials: you create a Google Cloud project and import its JSON. The same steps, with links to each console page and a button to copy the scopes, are in the app under **How to connect**.

1. **Create a project just for this app.** The name shown at sign-in (Branding) and the access rules apply to everything that uses the project. To get the **Internal** audience, create it signed in with the company account, inside its organization; a personal project only offers External.
2. **Enable the Google Chat API** and, on its Configuration tab, fill in the name, an icon URL and a description, with interactive features turned off. Without the API nothing works (`SERVICE_DISABLED`). Without the configuration, reading works but sending fails with "Google Chat app not found".
3. **Google Auth Platform:**
   - **Branding:** the name shown at sign-in and your email.
   - **Audience:** **Internal** accepts only accounts of the organization that owns the project, and anyone else gets `org_internal`. **External** accepts accounts from any company.
   - **Data access:** the scopes in the table below.
4. **With the External audience, choose testing or production:**
   - **In testing**, only accounts listed under Audience > Test users can sign in (up to 100). Others see "has not completed the Google verification process (...) can only be accessed by developer-approved testers". Sessions expire every 7 days.
   - **In production** (Audience > Publish app), sessions don't expire weekly. Without verification, sign-in shows the unverified app screen, capped at 100 users for the project's lifetime. The documentation says restricted scopes need verification in production; if Google blocks sign-in, go back to testing.
5. **Clients:** create a **Desktop app** client, download the JSON, import it in the app (Settings > **Import JSON file**) and use **Add Google account**. Sign-in opens in the system browser.

To sign in from another browser or a container (Firefox or Zen containers, for example), use **Copy link** while sign-in is pending and paste it there. Copy it with that button: the URL of a step already started in a browser doesn't work in another container, and Google answers with a 400 error.

| Scope | Why |
| --- | --- |
| `openid`, `email`, `profile` | Know which account signed in |
| `chat.spaces.readonly` | List conversations and spaces |
| `chat.memberships.readonly` | Name DMs and group chats, which the API returns without a name |
| `chat.messages` | Read and send messages and attachments |

### More than one project

You can import several JSON files, one per project, and each account signs in through the project you pick in Settings > Google Cloud projects. Two cases call for it:

- **Avoid the 7 days where you can.** In the company where you are an admin, a project with the **Internal** audience has no weekly session expiry. Other companies stay on an **External** project.
- **A company that requires its own project.** If a company's admin only allows apps from their own project, import that project's JSON and connect the account through it.

Each account renews its session with the credentials of the project it signed in through. A project can only be removed once no account uses it; to move an account to another project, connect it again through that project.

### What to expect from Google

- **In testing mode, sessions expire after 7 days.** With the External audience and *Testing* status, Google issues refresh tokens that expire in a week. When one expires, the app tells you and you add the account again.
- **`chat.messages` is a restricted scope.** Published without verification, the app shows the "unverified app" screen and accepts at most 100 users for the project's lifetime. The policy says restricted scopes can't be used in production without review; the documentation doesn't say whether Google only warns or blocks sign-in in that case.
- **Each company can block the app.** If signing in with a company account returns `admin_policy_enforced`, that Workspace's admin has to mark the client ID as trusted in Admin console > Security > Access and data control > API controls.

Sources: [publishing and test users](https://support.google.com/cloud/answer/15549945), [scope policy in production](https://developers.google.com/identity/protocols/oauth2/production-readiness/policy-compliance), [refresh token expiration](https://developers.google.com/identity/protocols/oauth2#expiration), [Chat scope classification](https://developers.google.com/workspace/chat/authenticate-authorize), [app access control in Workspace](https://knowledge.workspace.google.com/admin/apps/control-which-apps-access-google-workspace-data).

## What it does

- Several accounts, switched from the left rail, each told apart by its organization's color and initials; several Google Cloud projects, each account in its own.
- Paginated conversation list, with a local filter and tabs for chats and spaces.
- Paginated history, threads in a side panel, sending and replying in threads.
- Safe retries: if sending fails, the text stays and the retry reuses the same `requestId`, so Google doesn't duplicate the message.
- Attachments up to 200 MB: send them (one per message, the text goes with the first), download the ones you receive, and open Drive files in Drive. If sending fails midway, the retry picks up where it stopped.
- System notifications for new messages in any account, with unread counters. The interval (15 s to 5 min) and the on/off switch are in Settings.
- System tray: closing the window hides the app and notifications keep coming; **Quit** in the tray menu exits.
- English and Portuguese, following the system or chosen in Settings. Light, dark or system theme; keyboard and screen reader support.

Tokens and the client secret never reach the UI: they stay in the Go process and the system keyring. `accounts.json` holds only the imported projects (client ID and name) and, for each account, its name, email, domain and the project it signed in through.

## Known limits

- Notifications come from periodic checks, up to one interval late, and only while the app runs (window or tray).
- Unread counts start when the app opens: syncing Google's read state would need the `chat.users.readstate` scope.
- There is no typing indicator or presence for others: the API doesn't offer them. Accepting space invitations from the app isn't possible for the same reason.
- Messages show as text and links, without Google Chat's formatting or cards.

Not yet tested with real accounts: signing in a second organization through the same External project, an account signing in through an Internal project, and the installers running on Windows and macOS (so far they were only built in CI).

What the API offers and what it lacks is in [docs/google-chat-api.md](docs/google-chat-api.md) (in Portuguese).

## Next steps

1. Unread state synced with Google (`chat.users.readstate`).
2. Real-time events (Workspace Events API + Pub/Sub) instead of periodic checks.
3. Message formatting, image previews and cards.
4. Edit, delete, react, search and start new conversations.
5. Signed packages (Developer ID on macOS, a certificate on Windows).
6. Calendar, Meet (starting with opening links) and Gmail.

## Development

| Part | What |
| --- | --- |
| Window and packaging | [MyGo](https://mygo.egoist.dev) 0.5.5, with the system web view (WebKitGTK, WebView2, WKWebView) |
| Backend | Go 1.27: OAuth, system keyring and Chat API calls |
| UI | React 19, TypeScript 7, [Fluent UI](https://react.fluentui.dev) 9 |
| Languages | Catalogs in `locales/`; MyGo generates the Go package and `src/locales.ts` |

```
main.go, desktop.go     window, tray and the methods the UI calls
notify.go, files.go     notifications and files on the desktop side
internal/chat/          OAuth, per-account sessions, Chat API and the message watcher
locales/                language catalogs (en is the source, pt-BR the translation)
src/App.tsx             UI
e2e/                    UI tests against a simulated MyGo bridge
scripts/appimage.sh     packages the Linux build as an AppImage
```

You need Go 1.27+, [Bun](https://bun.sh) (MyGo uses it to read the config and run tasks) and Node 24+.

```sh
make setup      # dependencies and the Chromium used by tests
make dev        # app with live reload
make preview    # UI only, in a browser, without sign-in
make check      # lint, tests and build: the same as CI
make build      # package for the current platform in build/
make appimage   # the Linux build as an AppImage too
make build-all  # Linux x64, Windows x64 and universal macOS
```

`make dev` stores data under `Google Chat Client Dev` and the installed app under `Google Chat Client`, in the system config directory. Their secrets live in separate keyring services, so you can work on the dev app without touching the accounts of the one you use.

`make test` runs both halves: Go with `-race` against a simulated Google (the whole OAuth flow, concurrent token refresh, API errors, the message watcher, upload and download) and the UI in Playwright with the simulated bridge.

To ship a version, update `version` in `mygo.config.ts` and `CHANGELOG.md`, then push the tag `v<version>`: the release workflow builds the packages for the three systems and creates the release.

Contributions are welcome: see [CONTRIBUTING.md](CONTRIBUTING.md). Design decisions are in [docs/design.md](docs/design.md) (in Portuguese).

## License

[MIT](LICENSE).
