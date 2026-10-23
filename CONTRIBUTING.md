# Contributing to the Parlance server

## Setup

You need Go (the version in `go.mod`), Docker, and for the protos the
[Buf CLI](https://buf.build/docs/installation/). `make install-tools` installs Buf
and golangci-lint with `go install`; `make hooks` then turns on the pre-commit hook
(see [Checks](#checks)).

```bash
make dev       # API, PostgreSQL and MinIO in Docker; the API reloads on changes (air)
```

To run the API on your machine instead, start only PostgreSQL and MinIO from the
Compose file, copy `.env.example` to `.env` and run `make run`. It generates the
proto code, seeds the database and starts the API.

The web app, [parlance-web](https://github.com/Kazuto/parlance-web), expects this
repository at `../server` next to it.

### Demo data

`make seed` **drops the database**, creates it again and fills it with demo entries,
scopes, glossary terms and these users:

| Email                     | Password        | Role       |
| ------------------------- | --------------- | ---------- |
| `admin@parlance.dev`      | `admin123`      | admin      |
| `translator@parlance.dev` | `translator123` | translator |
| `viewer@parlance.dev`     | `viewer123`     | viewer     |
| `john@example.com`        | `password123`   | translator |

## Checks

| Command      | What it does                                   |
| ------------ | ---------------------------------------------- |
| `make test`  | All tests, with the race detector and coverage |
| `make lint`  | golangci-lint (`.golangci.yml`)                |
| `make fmt`   | gofmt; `make fmt-check` only reports           |
| `make vet`   | go vet                                         |
| `make ci`    | fmt-check, vet and tests                       |
| `make hooks` | Run the pre-commit hook before each commit     |

The pre-commit hook (`.githooks/pre-commit`) checks the staged Go files: gofmt,
golangci-lint on the changed lines, then the tests. It only reports, so a commit
never picks up unstaged edits; `make fmt` and `golangci-lint run --fix` fix most
findings. `git commit --no-verify` skips it once.

CI runs vet, the tests, golangci-lint and a build on pushes and pull requests to
`main` and `develop`.

## Tests

Handler tests run against an in-memory SQLite database from
`internal/server/testutil`, so they need no PostgreSQL:

- `SetupTestContext()` creates the tables and a user with every permission.
- `ContextWithPermissions(db, names...)` signs in a user with only the given
  permissions, to test that a call is refused without them.

SQLite has no `uuid_generate_v4()`, so the test database generates ids in a GORM
hook. Add the table to `SetupTestDB` when a test needs a new model.

## Changing the API

1. Edit the protos in `proto/parlance/v1/` and run `make proto-lint`.
   `make proto-breaking` compares with `main`.
2. `make proto-gen` generates the Go code in `gen/`; `make proto-gen-all` also
   generates the web app's Zod schemas (see the web app's CONTRIBUTING.md).
3. Implement the handler in `internal/server/<service>/`, one file per method, and
   register new services in `cmd/api/main.go`.

### Permissions

Every handler checks its permission with `middleware.RequirePermission`. A new
permission needs:

- **A constant** in `internal/permissions/permissions.go`, also listed in
  `AllPermissions()`. The API adds missing permissions to the database on start.
- **Roles:** the admin role gets every permission on start. Add it to the demo
  roles that should have it in `internal/database/seeders/models/roles.go`.
- **Web app:** add it to `test/permissions.ts` in the web app, whose tests check
  that every permission the app uses exists.

### Database

GORM creates and updates the tables on start (`AutoMigrate` in
`internal/database/db.go`); add new models there. There are no SQL migrations.

## Layout

```
cmd/
  api/              API entry point
  seed/             demo data (make seed)
  protoc-gen-zod/   Buf plugin generating the web app's Zod schemas
proto/parlance/v1/  API definitions
gen/                generated Go code
internal/
  server/           one package per service, one file per method
  models/           GORM models
  database/         connection, AutoMigrate, bootstrap (permissions, admin role), seeders
  middleware/       sign-in and permission checks
  permissions/      permission names
  translate/        machine translation: prompt, cleanup, OpenAI-compatible client
  translators/      translation servers from the database
  secret/           encryption of stored API keys
  gettext/          PO files
docker/             Dockerfile and Compose file
```
