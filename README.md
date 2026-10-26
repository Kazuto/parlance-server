# Parlance Server

The API of Parlance, a self-hosted translation management app: entries with
translations per language, scopes, a glossary, import and export, users and roles,
and optional machine translation.

It is written in Go with [Connect RPC](https://connectrpc.com/), GORM and
PostgreSQL. The web app lives in
[parlance-web](https://github.com/Kazuto/parlance-web); its README explains how to use
Parlance.

## Running the server

### With Docker Compose

`docker/docker-compose.yml` starts the API with PostgreSQL and MinIO (S3-compatible
storage for avatars):

```bash
make dev          # docker compose up -d
make dev-logs     # follow the logs
make dev-down     # stop and remove the containers
```

The API listens on <http://localhost:8080>; `GET /health` answers once it runs. On
start it creates or updates the database tables and the built-in permissions and
roles.

On a fresh database, the web app asks for the first administrator
(`SetupService/Initialize`). To fill the database with demo data instead, run
`make seed` (see [CONTRIBUTING.md](CONTRIBUTING.md#demo-data)).

### Production

Version tags publish the image `ghcr.io/kazuto/parlance-api`, which runs the API on
port 8080. It needs PostgreSQL and S3-compatible storage, configured with the
variables below.

PostgreSQL must be version 13 or later and offer the `pg_trgm` extension, as the
official images do; the translation memory compares texts with it. The API creates
the extension on start, which needs the `CREATE` privilege on the database; the
database's owner has it. Without it, the API does not start.

## Configuration

Set these as environment variables or in a `.env` file (see `.env.example`):

| Variable                                | Default                          | Description                                                                                                                                                                                                                                                      |
| --------------------------------------- | -------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `PORT`                                  | `8080`                           | Port of the API                                                                                                                                                                                                                                                  |
| `ENV`                                   | `development`                    | Name of the environment, shown in the log                                                                                                                                                                                                                        |
| `DB_HOST`, `DB_PORT`                    | `localhost`, `5432`              | PostgreSQL server                                                                                                                                                                                                                                                |
| `DB_USER`, `DB_PASSWORD`, `DB_NAME`     | `parlance`, `secret`, `parlance` | PostgreSQL login and database                                                                                                                                                                                                                                    |
| `DB_SSL_MODE`                           | `disable`                        | PostgreSQL `sslmode`                                                                                                                                                                                                                                             |
| `JWT_SECRET`                            | `change-this-secret`             | Signs the sign-in tokens. **Set your own.**                                                                                                                                                                                                                      |
| `JWT_EXPIRY`                            | `24h`                            | How long a sign-in lasts; refresh tokens last 7 times as long                                                                                                                                                                                                    |
| `ENCRYPTION_KEY`                        | (none)                           | Encrypts stored API keys of translation servers, access tokens of tasks and signing secrets of webhooks, and client secrets of sign-in providers. Needed only to save them. Changing it makes saved ones unreadable. Generate one with `openssl rand -base64 32` |
| `CORS_ALLOWED_ORIGINS`                  | `http://localhost:3000`          | Comma-separated addresses of the web apps allowed to call the API; single sign-on only returns to these                                                                                                                                                          |
| `S3_ENDPOINT`, `S3_REGION`, `S3_BUCKET` | (none)                           | S3-compatible storage for avatars, which the API serves at `/avatars/{id}`; the bucket is created if missing and needs no public access                                                                                                                          |
| `S3_ACCESS_KEY`, `S3_SECRET_KEY`        | (none)                           | Credentials for that storage                                                                                                                                                                                                                                     |

The Docker Compose file sets development values for all of them; set your own
`JWT_SECRET` and `ENCRYPTION_KEY` anywhere else.

## Machine translation

Parlance can suggest translations with Tencent's
[Hy-MT2](https://github.com/Tencent-Hunyuan/Hy-MT2) translation models. The
suggestions are drafts in the editor; nothing is saved without review.

### Running a model

Any OpenAI-compatible server running a Hy-MT2 model works. The Compose file
includes one, llama.cpp's `llama-server` with the 7B model (about 4.6 GB,
downloaded on the first start, and as much memory):

```bash
docker compose -f docker/docker-compose.yml --profile translation up -d translator
```

It runs on the CPU. For a GPU, also an integrated one (Intel, AMD), use the Vulkan
image and pass the device in a `docker-compose.override.yml` next to the Compose
file:

```yaml
services:
  translator:
    image: ghcr.io/ggml-org/llama.cpp:server-vulkan
    devices:
      - /dev/dri:/dev/dri
    command:
      - -hf
      - "tencent/Hy-MT2-7B-GGUF:Q4_K_M"
      - --host
      - 0.0.0.0
      - --port
      - "8080"
      - --parallel
      - "2"
      - --ctx-size
      - "8192"
      - -ngl
      - "99"
```

`TRANSLATOR_MODEL` picks another model, for example the 1.8B model
`tencent/Hy-MT2-1.8B-GGUF:Q4_K_M` (about 1.1 GB) for small hardware. It is faster
but makes more mistakes, and often ignores a language's form of address: asked for
"du", it kept German instructions such as "Geben Sie Ihren Namen ein" formal, where
the 7B model wrote "Gib deinen Namen ein". vLLM and SGLang also serve the models,
but need an NVIDIA GPU.

### Connecting it

1. In the web app, add the server under **Settings → Machine translation**, for
   example `http://translator:8080/v1`, and press **Test**. Set "Requests at a
   time" to the server's `--parallel`.
2. Give the roles that may translate the permission `use_machine_translation`.

You can add several servers: requests are spread over all enabled ones, each taking
at most its number of requests at a time, and a request that fails on one server is
tried on the others.

### How it translates

- **One language per request:** the model translates one text into one language,
  so an entry with eight empty languages makes eight requests.
- **Prompt:** every prompt asks for user interface style. Glossary terms that occur
  in the text are sent along with their translation; placeholders such as
  `{language}` don't count as the term "language".
- **Form of address:** a language set to formal or informal (`Locale.formality`)
  asks for it by its pronoun, e.g. "addressing the user as Sie" or "as du". German,
  French, Spanish, Italian, Dutch and Russian have their pronouns; other languages
  ask for "politely" or "casually". Without the setting the model picks one per
  text, and mixes them.
- **Placeholders:** `{name}` placeholders are sent as `{1}`, `{2}`, which the model
  keeps, and restored afterwards.
- **Dashes:** a dash the model puts between words (" – ") becomes a comma unless
  the source has one too. The models like joining clauses that way, which German
  and most target languages rarely do in interface text.
- **Checks:** answers that are empty or far longer than the source are rejected.
  Missing or extra placeholders, a different number of plural forms (`|`), new
  line breaks and, in the languages above, the other form of address ("du" where
  "Sie" was asked for) are flagged for review.
- **Time limit:** a request for one entry ends after 100 seconds; languages not
  done by then are reported as timed out.

The code is in `internal/translate` (prompt, cleanup, client) and
`internal/translators` (servers from the database).

## Tasks

Tasks do work on their own, set up by admins under **Automation** in the web app:

- **Machine translation** fills missing translations from the default (or a chosen)
  language, as drafts. Existing translations, drafts included, are kept, and
  results the checks flag are left out. One run makes at most 200 translations by
  default; the rest wait for the next run. With **Also redo machine drafts by other
  models**, it also translates again drafts a model wrote that the translation
  servers no longer serve, e.g. after switching to a bigger model. Only unedited
  machine drafts qualify: each translation records whether a model or a person
  wrote it, and which model. Approved, edited and rejected ones stay, and the old
  text stays in the history.
- **Git export** renders the chosen formats for each language, for example
  `lang/{locale}.json`, and proposes them as a GitHub pull request from its own
  branch. Only approved translations are exported unless drafts are included. The
  branch is rebuilt from the base branch on every run, and nothing is pushed while
  the repository or the branch already hold the files. It needs an access token
  (see below), which is stored encrypted, so it needs `ENCRYPTION_KEY`.

### GitHub access token

Create a [fine-grained personal access token](https://github.com/settings/personal-access-tokens/new)
with **Repository access** limited to the export's repository, and these
repository permissions:

| Permission    | Access         | Used for                                                              |
| ------------- | -------------- | --------------------------------------------------------------------- |
| Contents      | Read and write | Reading the base branch, committing the files, updating the PR branch |
| Pull requests | Read and write | Finding the open pull request of the branch, opening or updating it   |
| Metadata      | Read-only      | Reading the repository; GitHub always adds it                         |

Nothing else is needed. For a repository of an organization, choose the
organization as the token's resource owner; the organization may have to approve
the token first. Branch protection must allow force pushes to the pull request
branch (the base branch is never pushed to).

**Check access** in the task form tells whether the repository and base branch
can be read and the token's user may push. GitHub reports the user's access there,
not the token's, so a token with read-only Contents passes the check but fails on
its first run with a 403.

### Running tasks

A task runs on its cron schedule (5 fields, UTC), after events, or when started by
hand. Events, for now approving translations (in the editor or with an approving
import), start a run after the task's delay; further events join that run, so a
burst of approvals runs once. Each task has at most one queued and one running
run.

Tasks and their settings are kept in the database and set up in the app; nothing
about them is configured at deployment. The switch at the top of the Automation
page pauses all tasks: nothing is scheduled or run until they are resumed. Runs
triggered in the meantime wait, and a schedule that came due fires once on resuming.

The runner runs inside the API server (`internal/taskrunner`) and looks for work
every 15 seconds; starting a run wakes it right away. Runs and scheduled
firings are claimed with row locks, so several instances can share the work. A
run that stops reporting, for example because its server stopped, fails after two
minutes. The last 100 runs of each task are kept with their logs.

Permissions: `read_task`, `create_task`, `update_task`, `delete_task`, and
`run_task` to start and cancel runs.

## Webhooks

Webhooks tell other services about events in Parlance, for example a CI pipeline
that builds once translations are approved, or a chat channel. Admins set them up
under **Webhooks** in the web app: a URL, the events it gets, a format, when it
sends, and optionally a signing secret.

The format decides the request body:

- **JSON** posts the event with all its details, as below. Use it for your own
  services.
- **Slack** posts a short message as `{"text": …}`, for Slack and chat services
  that take Slack's incoming webhooks, such as Mattermost.
- **Discord** posts a short message as Discord's webhooks take it, with mentions
  turned off.

Chat services turn down other bodies (Discord answers 400, "Cannot send an empty
message"). The web app picks the format when you type a Discord or Slack address.
Deliveries keep the event and render the body when sent, so after changing the
format, retries and **Send again** use the new one.

| Event                                   | Sent when                                                                                                                                        | `data`                                                                                                    |
| --------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------- |
| `translations.awaiting_approval`        | Translations become drafts: new or changed in the editor, by machine translation (in the editor or a task), or by an import that doesn't approve | `translations` (`id`, `entry_id`, `key`, `locale`, `translation`), `user`, and `task` for a task's drafts |
| `translations.approved`                 | Translations are approved in the editor or by an approving import                                                                                | `translations` and `user`                                                                                 |
| `translations.rejected`                 | A reviewer sends translations back                                                                                                               | The same, plus the reviewer's `comment`                                                                   |
| `entries.created`                       | Entries are created in the app or by an import                                                                                                   | `entries` (`id`, `key`, `description`) and `user`                                                         |
| `task_run.succeeded`, `task_run.failed` | A task run ends, including runs that stopped responding; canceled runs don't                                                                     | `task` (`id`, `name`, `type`) and `run` (`id`, `trigger`, `status`, times, `error`, `summary`, `user`)    |
| `digest`                                | A webhook with a schedule sends what it collected                                                                                                | `since`, `count`, `events` (the bodies above, oldest first) and `truncated`                               |
| `ping`                                  | Someone presses **Send test**                                                                                                                    | `webhook` (`id`, `name`, `events`)                                                                        |

One action sends one event, however many translations or entries it touches: the
lists hold the first 1000, `count` says how many there were and `truncated` whether
some were left out. `user` is who did it, or `null`. In the editor, each saved
translation is one action; a batch of machine translations, an import or a task
run is one too.

**Digests:** a webhook sends each event right away, or collects them and sends
one `digest` on a schedule: a standard 5-field cron expression in UTC, picked as
hourly, daily, weekly or custom in the app. Nothing is sent when nothing was
collected. In the chat formats, a digest is one message with a line per kind of
change, for example "12 translations submitted for approval (Rita, Task
“Nightly”): …", and one per task run. A digest carries up to a megabyte of events;
`count` holds them all. Removing the schedule sends what was collected right away;
turning the webhook off drops it.

In the JSON format, each event is posted like this:

```http
POST /hooks/parlance HTTP/1.1
Content-Type: application/json
User-Agent: Parlance-Webhook
X-Parlance-Event: translations.approved
X-Parlance-Delivery: 6f4a0281-b730-4d0a-bde3-15a61bb44519
X-Parlance-Signature: sha256=5d2c9a…

{"id":"29f4342d-…","event":"translations.approved","created_at":"2026-10-23T18:21:09.286Z",
 "data":{"count":1,"translations":[{"id":"…","entry_id":"…","key":"button.save","locale":"de",
 "translation":"Speichern"}],"truncated":false,"user":{"id":"…","name":"Rita"}}}
```

- **Ids:** the body's `id` names the event and is the same in every delivery of it,
  to every webhook, retries and redeliveries included. `X-Parlance-Delivery` names
  one delivery and stays the same across its retries.
- **Signature:** with a secret, `X-Parlance-Signature` is `sha256=` followed by the
  hex HMAC-SHA256 of the raw body as sent, keyed with the secret. Check it before trusting
  the body, comparing in constant time:

  ```js
  import { createHmac, timingSafeEqual } from "node:crypto";

  const expected = `sha256=${createHmac("sha256", secret).update(rawBody).digest("hex")}`;
  const valid =
    signature.length === expected.length &&
    timingSafeEqual(Buffer.from(signature), Buffer.from(expected));
  ```

- **Answers:** any 2xx answer counts as delivered. Other answers, redirects (they
  aren't followed) and no answer within 10 seconds count as failed.
- **Retries:** a failed delivery is tried again after 1 minute, 10 minutes, 1 hour
  and 6 hours, then given up. Deliveries can arrive out of order, and, as a server
  may stop between sending and recording, more than once: skip event ids you have
  seen.
- **Testing:** **Send test** posts a `ping`, and **Send again** on a delivery posts
  its body anew. Both send right away, once, also to webhooks that are off.
- **Turning off** a webhook fails its pending deliveries and drops the events it
  collected.
- **History:** the last 100 deliveries of each webhook are kept, with the body sent
  and the start of the answer.

URLs may point into your own network, such as `http://jenkins:8080/…`; only users
with the webhook permissions can set them. Secrets are stored encrypted, so they
need `ENCRYPTION_KEY`.

The dispatcher runs inside the API server (`internal/webhook`) and looks for due
digests and deliveries every 5 seconds. Both are claimed with row locks, so several
instances share the work; one whose server stopped while sending is sent again a
minute later. Changes queue their events in their own transaction, so a change
that fails sends nothing.

Permissions: `read_webhook`, `create_webhook`, `update_webhook` (also to send
tests and redeliver) and `delete_webhook`.

## Single sign-on

Besides email and password, people can sign in with OpenID Connect providers such
as Keycloak, Authentik, Authelia, Zitadel, Microsoft Entra ID, Google or Okta.
Admins set them up under **Settings → Single sign-on** in the web app:

1. At the provider, create a confidential client (or a public one, without a
   secret) for the authorization code flow, and register the redirect URI the
   settings show: the web app's address followed by `/sso/callback`. The web app's
   address must be in `CORS_ALLOWED_ORIGINS`.
2. In Parlance, add the provider with its issuer URL, exactly as the provider names
   it (Authentik's ends in a slash), the client ID and secret, and press **Test**.
   The secret is stored encrypted, so it needs `ENCRYPTION_KEY`.
3. Choose what happens to people who have no account yet: create one with a role,
   or create none, so only users an admin added can sign in.

The sign-in page then offers "Sign in with …" for each enabled provider.

- **Flow:** authorization code with PKCE, asking for `openid profile email`. The
  API keeps the state, nonce and verifier of each sign-in for ten minutes and uses
  them once, verifies the ID token's signature, issuer, audience and nonce, and
  then issues the usual Parlance tokens. Provider sessions aren't tracked:
  signing out of Parlance doesn't sign out of the provider.
- **Accounts:** the first sign-in with an account at the provider finds the
  Parlance user by email, ignoring case, and links them; later sign-ins find them
  by the provider's id of the account (`sub`), even after the email changed. An
  email the provider marks as unverified (`email_verified: false`) is refused.
  Without an email in the ID token, the userinfo endpoint is asked.
- **New accounts** get the provider's role, the name from the `name` or
  `preferred_username` claim, and no password, so they can only sign in through a
  provider until an admin sets one. Deleted users can't sign in.
- **Passwords** keep working for everyone who has one, so a provider that is down
  locks nobody out. Deleting a provider removes its links to users; accounts it
  created stay, but need a password from an admin to sign in.

Permissions: `read_sso_provider`, `create_sso_provider`, `update_sso_provider`
(also to test) and `delete_sso_provider`. The code is in `internal/sso`, with a
test provider in `internal/sso/ssotest`.

## API

All endpoints are Connect RPC calls: `POST /parlance.v1.<Service>/<Method>` with a
JSON (or binary protobuf) body. The definitions are in
[`proto/parlance/v1/`](proto/parlance/v1/). `AuthService` (including the single sign-on
calls), `SetupService` and reading languages work without signing in; every other call needs the access
token from `AuthService/Login`:

```bash
curl -X POST http://localhost:8080/parlance.v1.AuthService/Login \
  -H "Content-Type: application/json" \
  -d '{"email": "admin@parlance.dev", "password": "admin123"}'
# {"accessToken": "…", "refreshToken": "…", "user": {…}}

curl -X POST http://localhost:8080/parlance.v1.EntryService/ListEntries \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -d '{"pagination": {"page": 1, "perPage": 20}}'
```

Every call checks the caller's permissions, such as `read_entry` or
`update_locale`. `AccountService` needs none: everyone signed in may change their
own name, email, language, avatar and password; changing the password asks for
the current one, unless the account has none yet, like accounts single sign-on
created. Avatars are PNG, JPEG, WebP or GIF images of up to 2 MB; their type is
read from the image.

`AuthService/Register` creates an account with the viewer role. It is open until
an admin closes it in the app under **Settings → Registration**, or with `UserService/UpdateRegistrationSettings`
(`create_user`); while closed, admins still add users.

### API tokens

Scripts and CI use an API token instead of signing in. Create one in the app under
**My account → API tokens**, or with `AccountService/CreateApiToken`, and send it
like an access token:

```bash
curl -X POST http://localhost:8080/parlance.v1.ExportService/ExportJson \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer plc_…" \
  -d '{"localeCode": "de"}'
```

- **Permissions:** a token acts as the user who created it, with that user's
  permissions as they are at each request.
- **Secret:** the token is shown once; only its SHA-256 hash is stored. Its first
  characters (`plc_` and six more) tell tokens apart.
- **Lifetime:** 1 to 3650 days, or no expiry. Revoking one takes effect at once,
  and deleting a user revokes theirs. A user has at most 50.
- **Limits:** a request made with a token can't list, create or revoke tokens, and
  can't change its user's password or email; those need a sign-in.

### Create and update requests

Create and update requests share the same shape. Every `Update…` request is a
partial update:

- **Omitted field:** the current value is kept.
- **Sent field:** the value is set. An empty value clears it; required fields
  (e.g. `key`, `name`, `email`) reject an empty value with `InvalidArgument`.
- **Related ids** (`scopes`, `permissions`, `roles`) are sent as
  `{ "ids": [...] }`. Omit the field to keep the current set, send
  `{ "ids": [] }` to remove all, or send ids to replace the set.
- **Locale names** are sent as `names: { "values": { "en": "German" } }`.

Lists and maps are wrapped because protobuf cannot tell an empty list or map
from one that was not sent. Ids are validated, and changes to the record and its
relations are saved in one transaction.

### Entry translation filters

`ListEntries` accepts `scopeIds`, `translationStatus` and `targetLocaleIds`. Status
is evaluated before pagination against active, non-empty saved translations:

- `missing`: at least one target language lacks a translation.
- `complete`: none do.
- `review`: at least one translation is a draft awaiting approval.
- `outdated`: at least one translation's default text changed since it was written
  or approved.

An empty target list checks all active locales. Omit `translationStatus` to include
entries regardless of status.

### Import and export

`ImportService` and `ExportService` handle JSON, PHP arrays (Laravel), gettext PO
and ZIP files with one file per language. Imports take a `localeId`, optional
`scopeIds`, a `filename`, a `mode` and the file as `fileContent` (base64 in JSON
requests). The modes are create or update (the default), create only, update only,
and replace all. Exports take a `localeCode` and optional `scopeIds`; without
scopes they export all active entries. ZIP imports and exports cover all languages
at once, so they take no language.

With `dryRun`, an import changes nothing and reports what it would do: the same
counts, `translationsAdded`, `translationsChanged` and `translationsRemoved`, how
many translations in other languages a changed default text would mark outdated,
and the first 500 changes by key with their old and new text. A ZIP dry run imports
its files in one transaction, so each sees the ones before it, as a real import does.

**JSON:** imports accept flat dotted keys and nested objects, also mixed:
`{"entries": {"create": "Create Entry"}}` becomes `{"entries.create": "Create Entry"}`.
Values must be strings (empty ones too). Arrays, numbers, booleans, null, empty
keys, duplicate keys and keys that collide once flattened are rejected before
anything is saved. Exports are flat.

**PO:** singular, key-based UTF-8 files. `msgid` is the entry key and `msgstr` the
translation:

```po
msgid "button.save"
msgstr "Speichern"
```

- **Import:** multiline strings, escapes, comments and a UTF-8 BOM are understood.
  Broken syntax and non-UTF-8 files are rejected before anything is saved.
  Context (`msgctxt`), plural, fuzzy and duplicate records are reported in
  `result.errors` and counted as failed. Obsolete records and empty `msgstr` values
  are skipped, keeping existing translations. Replace all rejects files with
  unsupported or duplicate records before removing anything. The language comes
  from `localeId`, not from the file's Language header.
- **Export:** sorted by key, with UTF-8 and language headers and entry descriptions
  as `#.` comments. Entries without a translation get an empty `msgstr`.

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for tools, the proto workflow, tests and the
project layout.

## License

GNU General Public License, see [LICENSE](LICENSE).
