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

## Configuration

Set these as environment variables or in a `.env` file (see `.env.example`):

| Variable                                | Default                          | Description                                                                                                                                                                                |
| --------------------------------------- | -------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `PORT`                                  | `8080`                           | Port of the API                                                                                                                                                                            |
| `ENV`                                   | `development`                    | Name of the environment, shown in the log                                                                                                                                                  |
| `DB_HOST`, `DB_PORT`                    | `localhost`, `5432`              | PostgreSQL server                                                                                                                                                                          |
| `DB_USER`, `DB_PASSWORD`, `DB_NAME`     | `parlance`, `secret`, `parlance` | PostgreSQL login and database                                                                                                                                                              |
| `DB_SSL_MODE`                           | `disable`                        | PostgreSQL `sslmode`                                                                                                                                                                       |
| `JWT_SECRET`                            | `change-this-secret`             | Signs the sign-in tokens. **Set your own.**                                                                                                                                                |
| `JWT_EXPIRY`                            | `24h`                            | How long a sign-in lasts; refresh tokens last 7 times as long                                                                                                                              |
| `ENCRYPTION_KEY`                        | (none)                           | Encrypts stored API keys of translation servers and access tokens of tasks. Needed only to save them. Changing it makes saved ones unreadable. Generate one with `openssl rand -base64 32` |
| `CORS_ALLOWED_ORIGINS`                  | `http://localhost:3000`          | Comma-separated addresses of the web apps allowed to call the API                                                                                                                          |
| `S3_ENDPOINT`, `S3_REGION`, `S3_BUCKET` | (none)                           | S3-compatible storage for avatars                                                                                                                                                          |
| `S3_ACCESS_KEY`, `S3_SECRET_KEY`        | (none)                           | Credentials for that storage                                                                                                                                                               |

The Docker Compose file sets development values for all of them; set your own
`JWT_SECRET` and `ENCRYPTION_KEY` anywhere else.

## Machine translation

Parlance can suggest translations with Tencent's
[Hy-MT2](https://github.com/Tencent-Hunyuan/Hy-MT2) translation models. The
suggestions are drafts in the editor; nothing is saved without review.

### Running a model

Any OpenAI-compatible server running a Hy-MT2 model works. The Compose file
includes one, llama.cpp's `llama-server` with the 1.8B model (about 1.1 GB,
downloaded on the first start):

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
      - "tencent/Hy-MT2-1.8B-GGUF:Q4_K_M"
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

`TRANSLATOR_MODEL` picks another model, for example the better but slower
`tencent/Hy-MT2-7B-GGUF:Q4_K_M`. vLLM and SGLang also serve the models, but need an
NVIDIA GPU.

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
  in the text are sent along with their translation.
- **Placeholders:** `{name}` placeholders are sent as `{1}`, `{2}`, which the model
  keeps, and restored afterwards.
- **Checks:** answers that are empty or far longer than the source are rejected.
  Missing or extra placeholders, a different number of plural forms (`|`) and new
  line breaks are flagged for review.
- **Time limit:** a request for one entry ends after 100 seconds; languages not
  done by then are reported as timed out.

The code is in `internal/translate` (prompt, cleanup, client) and
`internal/translators` (servers from the database).

## Tasks

Tasks do work on their own, set up by admins under **Automation** in the web app:

- **Machine translation** fills missing translations from the default (or a chosen)
  language, as drafts. Existing translations, drafts included, are kept, and
  results the checks flag are left out. One run makes at most 200 translations by
  default; the rest wait for the next run.
- **Git export** renders the chosen formats for each language, for example
  `lang/{locale}.json`, and proposes them as a GitHub pull request from its own
  branch. Only approved translations are exported unless drafts are included. The
  branch is rebuilt from the base branch on every run, and nothing is pushed while
  the repository or the branch already hold the files. It needs a token with
  read and write access to the repository's contents and pull requests (a
  fine-grained token works), which is stored encrypted, so it needs `ENCRYPTION_KEY`.

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

## API

All endpoints are Connect RPC calls: `POST /parlance.v1.<Service>/<Method>` with a
JSON (or binary protobuf) body. The definitions are in
[`proto/parlance/v1/`](proto/parlance/v1/). `AuthService`, `SetupService` and
reading languages work without signing in; every other call needs the access
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
`update_locale`.

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

`ListEntries` accepts `scopeIds`, `translationStatus` (`missing` or `complete`) and
`targetLocaleIds`. Status is evaluated before pagination against active, non-empty
saved translations. Missing means at least one target language lacks a
translation; complete means none do. An empty target list checks all active
locales. Omit `translationStatus` to include entries regardless of status.

### Import and export

`ImportService` and `ExportService` handle JSON, PHP arrays (Laravel), gettext PO
and ZIP files with one file per language. Imports take a `localeId`, optional
`scopeIds`, a `filename`, a `mode` and the file as `fileContent` (base64 in JSON
requests). The modes are create or update (the default), create only, update only,
and replace all. Exports take a `localeCode` and optional `scopeIds`; without
scopes they export all active entries. ZIP imports and exports cover all languages
at once, so they take no language.

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
