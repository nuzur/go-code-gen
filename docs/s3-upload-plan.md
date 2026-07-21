# Plan: S3 file upload in go-code-gen (config-builder driven, deploy pulls team creds)

## Implementation status (2026-07-21)
All four layers implemented and verified locally:
- **A — go-code-gen (DONE, verified):** `project/storage.go` `StorageConfig`; new `storage/`
  generator (`storagegen.go` + `templates/{client,handlers,register}.go.tmpl`) emitting a
  self-contained aws-sdk-go-v2 client + `/upload` + `/sign` handlers registered on the default
  mux; `aws:` block in `config_base.go.tmpl`; REST router forwards the two paths; `main.go.tmpl`
  `fx.Invoke(storage.Register)`; wired in `v1/v1.go` (+ root `go mod tidy`). `go build ./...` OK;
  the generated storage package was rendered into a temp module and **compiles + vets against
  real aws-sdk-go-v2**.
- **B — extension (DONE pending release):** `config/configvalues.go` `ObjectStore` field;
  `manager/execute.go` sets `StorageConfig.Enabled = ObjectStore != ""`. Compiles against
  go-code-gen HEAD via a local replace. **Blocked on: (1) a go-code-gen release + version bump in
  the extension go.mod; (2) adding the `object_store` (UUID / ENTITY_TYPE_DB_STORE) field to the
  go-code-gen ConfigurationEntity JSON in nem so the CLI's `BuildConfigFromJSON` accepts the key.**
- **C — nuzur-web (DONE, verified):** object-store `Select` + "Manage object stores" link added to
  `code_gen_custom_handler.tsx` (reuses `getTeam`/`objectStores`); saves the store UUID as
  `object_store`. Full project `tsc --noEmit` = 0 errors.
- **D — nuzur-cli (DONE, verified):** `--storage` flag + `deploy_config.go` plumbing;
  `resolveObjectStoreForDeploy` (via `GetObjectStoreWithSecret`); `provided["object_store"]`;
  `BootstrapParams` S3 fields + `aws:` block in `bootstrap.sh.tmpl` (0600). `go build ./...` OK;
  `deploy` tests pass (bootstrap template renders).

### Remaining (cross-repo, not code-completable here)
1. Tag/release go-code-gen, bump `github.com/nuzur/go-code-gen` in the extension's go.mod.
2. Add the `object_store` field to the go-code-gen extension's ConfigurationEntity (nem data /
   the extension-config-entity builder) — required for CLI validation to accept the key.
3. Optional: add the new `extensions.code_gen.object_store*` i18n strings to the web locale files
   (English fallbacks are inline, so the UI works without them).

## Context
A nuzur project's file fields (File/Image/Audio/Video) can be uploaded to S3 today only
**through the nuzur platform** (`UploadRecordFieldFile`). The backend that `go-code-gen`
produces exposes **no** file handling — file fields are just string/`[]byte` columns. So a
deployed nuzur app can't accept uploads on its own.

Goal: let a generated backend accept S3 uploads, configured from the **web config builder**,
and have `nuzur-cli deploy` pull the team's S3 credentials (from an existing team ObjectStore)
so the user isn't forced to paste secrets. Per-team S3 storage already exists end-to-end
(KMS-encrypted `ObjectStore` records + `GetObjectStoreWithSecret` RPC), so this is mostly
wiring plus new generated code.

## Locked design decisions
1. **Generic, non-entity-scoped HTTP endpoints** in the generated backend (avoids the
   multi-level nested-entity problem):
   - `POST /upload` (multipart) → app streams the file to S3, returns `{ url, key }`.
   - `POST /sign` → returns a presigned URL for a given key, on demand (for private buckets).
   - The caller puts the returned URL string into normal create/update payloads.
   - **File fields stay plain string columns** → the ~8 duplicated type switches and entity
     CRUD are **untouched**.
   - **Always available over HTTP regardless of the API-surface choice** (REST / gRPC / both):
     mounted on the REST server when REST is enabled; for gRPC-only projects, a minimal HTTP
     listener is generated/started on the existing http port just for these two endpoints.
     (No gRPC upload RPCs — multipart is HTTP-native.)
2. **No new bucket config.** Public/private is the caller's decision; `/sign` is always exposed.
3. **No custom endpoint / R2 / S3-compatible** in this task. Use nem's existing `ObjectStore`
   (Region/Key/Secret/Bucket) + AWS S3. **No nem proto change.** R2/endpoint = separate task,
   handled manually server-side for now.
4. **Web config builder** mirrors the DB-connection UX: pick an existing team ObjectStore or
   create a new one (link to team settings); otherwise provide credentials manually. Deploy
   resolves the chosen store's creds from the team via KMS.

## Key architectural insight (drives the whole split)
- **Generation** (web ZIP or CLI) needs only a boolean "storage enabled" (derived from an
  `object_store` reference being set). It emits the endpoints + an empty `aws:` config block.
- **Credentials** (region/bucket/key/secret) are runtime config, injected at **deploy** into
  `prod.yaml` from the team ObjectStore. The generator never sees the secret → **no
  credential-resolver needed in the extension runtime.** (Manual creds = deploy flags / editing
  prod.yaml.)

---

## Approach — four layers, all opt-in behind one `object_store` config field

### A. go-code-gen (generated code)
- **New generator config**: add `project/storage.go` → `StorageConfig{ Enabled bool }` (keep it
  minimal — region/bucket are runtime, not generation-time). Add the field to `Project` and
  `ProjectParams` in `project/project.go`, defaulted in `project.New(...)`. Mirror the opt-in
  `project/custom.go` + `{{ if .CustomConfig.Enabled }}` pattern.
- **Config struct/yaml**: extend the already-present unused `AWS` struct in
  `config/templates/config.go.tmpl` (region/key_id/secret/bucket — no endpoint), and emit an
  `aws:` block (empty placeholders) in `config/templates/config_base.go.tmpl`, guarded by
  `{{ if .StorageConfig.Enabled }}` (mirror the Kafka guard).
- **Generated storage package**: new templates under a `storage/` generator package emitting a
  small self-contained S3 client using **aws-sdk-go-v2** (`manager.Uploader` for `Upload`,
  `s3.NewPresignClient` for `SignURL`) — same calls nuzur's `aws/client.go` uses, but do **not**
  import nuzur's internal `aws` pkg (generated output must stand alone).
- **HTTP endpoints** (multipart is HTTP-native; no gRPC upload RPCs): the two handlers
  (`handler_upload.go.tmpl`, `handler_sign.go.tmpl`) live in a storage HTTP package so they can
  be mounted two ways. `upload` parses multipart, generates a key (`uploads/<uuid>/<filename>`),
  uploads, returns `{url,key}`; `sign` takes `{key, expiry?}` → `{url}`.
  - **Surface includes REST** → register the routes on the generated REST server
    (`rest/handler.go`, `rest/templates/router.go.tmpl`).
  - **gRPC-only** → emit a minimal HTTP server (`net/http` mux) bound to the existing http port,
    started from `main.go` when `StorageConfig.Enabled && !RESTEnabled`, serving only `/upload`
    and `/sign`. The http port is already allocated by deploy, so no new port is needed.
- **Wiring**: provide the S3 client via fx in `main/templates/main.go.tmpl`
  (`params.Provider.Get("aws").Populate(&cfg)` in a provider), inject into whichever server
  mounts the storage routes — all guarded by `StorageConfig.Enabled`.
- **Deps**: generated code imports aws-sdk-go-v2; the existing `go mod tidy` step in `v1/v1.go`
  resolves them (verify the go.mod template doesn't pin a curated list that needs the entries
  added explicitly).

### B. Extension config schema + mapping — `extensions/extension-go-code-gen`
- **Schema (data in nem)**: add an `object_store` field to go-code-gen's `ConfigurationEntity`
  (`type: UUID`, `type_config.uuid.entity_type: ENTITY_TYPE_DB_STORE`). `DB_STORE` already
  resolves to team ObjectStores in MCP/web/CLI, so **no proto/MCP/web-schema code change** — the
  field just needs to exist in the extension version's `configuration_entity` JSON (add via the
  web extension-config-entity builder or a seed/migration). Required so the CLI's
  `BuildConfigFromJSON` accepts the key (unknown keys are rejected).
- **Decode struct**: `config/configvalues.go` → add `ObjectStore string \`json:"object_store"\``.
- **Mapping**: `manager/execute.go` → set `project.StorageConfig{ Enabled: configvalues.ObjectStore != "" }`
  when building `ProjectParams`. (No credential lookup here — generation doesn't need secrets.)
  Storage is independent of the API-surface selection — enabled purely by an object-store being set.

### C. nuzur-web config builder — `nuzur-web`
- go-code-gen uses a **hardcoded** form (`src/extension-execution/custom_handlers/code_gen_custom_handler.tsx`),
  not the schema-driven one. Hand-build an object-store selector there: load the team via
  `editorRef.current?.productClient()?.getTeam(teamUuid)`, render an antd `Select` of
  `team.objectStores` (filter `ObjectStoreStatus.ACTIVE`), plus a "Manage object stores" link to
  `/team/settings/{teamUuid}/objectstores` and a refresh — mirroring the connection selector in
  `config_entity_field/uuid.tsx`. Save the chosen ObjectStore **UUID** under the `object_store`
  key in the form values.
- **Reuse (already built)**: team object-store CRUD UI `src/team-settings/object_stores.tsx`,
  `UITeam.objectStores` (`src/domain/team.ts`), and product-client store/secret methods
  (`src/client/product.ts`). No new management UI needed.

### D. nuzur-cli deploy — `nuzur-cli`
- **Flag/config**: add `--storage <object-store-uuid>` in `app/command_deploy.go` Flags, plumb
  through `app/deploy_config.go` (`DeployConfig.Storage`, `deploySettings.Storage`,
  `resolveDeploySettings`, `toDeployConfig`) — mirror `--connection`. If the flag is absent, fall
  back to the `object_store` value already saved in the project's config (`lastConfig`).
- **Resolve creds**: new `resolveObjectStoreForDeploy(storeUUID, teamUUID)` in
  `app/command_deploy_connection.go`, mirroring `resolveConnectionForDeploy` but calling
  `GetObjectStoreWithSecret({ObjectStoreUuid, TeamUuid})` (already in the CLI's protodeps) over
  the logged-in user's authenticated ctx. Returns Region/Bucket/Key/Secret.
- **Thread into generation**: set `provided["object_store"] = <uuid>` in the `provided` block
  (~L256-299) so the generated app emits the endpoints.
- **Inject creds into prod.yaml**: add S3 fields to `deploy/bootstrap.go` `BootstrapParams`
  (S3Enabled/S3Region/S3Bucket/S3Key/S3Secret), populate them at `command_deploy.go` ~L625-650,
  and emit an `aws:` block in the prod.yaml heredoc of `deploy/templates/bootstrap.sh.tmpl`
  (guarded by `S3Enabled`, mirroring the JWT block). prod.yaml is already `0600`.
- **Manual fallback**: optionally add `--s3-bucket/--s3-region/--s3-access-key/--s3-secret` flags
  for the no-team-store case; or document editing prod.yaml directly.

---

## Security / caveats to honor
- Unlike the box-generated DB password, S3 secrets are **passed into** the bootstrap script
  (like the external `--db-dsn` path) → the "secrets never leave the machine" property doesn't
  hold for S3 creds. Keep them only in `prod.yaml` (0600) and out of the workspace/git; never log
  them. Prefer the team-ObjectStore reference over `--s3-secret` flags (shell history).
- Storage endpoints are served over HTTP regardless of the API surface (REST/gRPC/both), so no
  surface constraint — a gRPC-only project still gets `/upload` + `/sign` via a minimal HTTP
  listener on the http port.
- Referencing a team ObjectStore keeps the secret KMS-encrypted until deploy; storing raw secrets
  in the web config values (nem `ConfigurationEntityValues`, plain JSON) is **not** acceptable —
  manual creds go through deploy flags / prod.yaml, not the config builder.

## Critical files (by layer)
- **Generator**: `go-code-gen/project/storage.go` (new), `project/project.go`,
  `config/templates/config.go.tmpl`, `config/templates/config_base.go.tmpl`,
  `rest/templates/handler_upload.go.tmpl` + `handler_sign.go.tmpl` (new), `rest/handler.go`,
  `rest/templates/router.go.tmpl`, `main/templates/main.go.tmpl`, new `storage/` templates, `v1/v1.go`.
- **Extension**: `extensions/extension-go-code-gen/config/configvalues.go`,
  `extensions/extension-go-code-gen/manager/execute.go`; + the `object_store` field added to the
  go-code-gen ConfigurationEntity (nem data / seed).
- **Web**: `nuzur-web/src/extension-execution/custom_handlers/code_gen_custom_handler.tsx`
  (reusing `team-settings/object_stores.tsx`, `domain/team.ts`, `client/product.ts`).
- **CLI**: `nuzur-cli/app/command_deploy.go`, `app/deploy_config.go`,
  `app/command_deploy_connection.go`, `deploy/bootstrap.go`, `deploy/templates/bootstrap.sh.tmpl`.

## Verification (end-to-end)
1. **Generator unit**: enable `StorageConfig` in a test `ProjectParams`, run `gocodegen.Generate`
   into a temp dir; assert `/upload` + `/sign` handlers, router routes, the `aws:` block in
   `base.yaml`, and the storage package exist; assert they're **absent** when disabled. Confirm
   `go build ./...` on the generated output (deps resolve via `go mod tidy`).
2. **Runtime smoke**: run the generated server with an `aws:` config pointing at a real/MinIO
   bucket; `curl -F file=@x.png .../upload` → returns `{url,key}`; store the url on a record via
   normal create; `POST /sign` → presigned GET that fetches the object.
3. **Web**: open the go-code-gen config builder, confirm the object-store Select lists team
   stores + the create/manage link, and the saved config carries `object_store: <uuid>`.
4. **Deploy**: `nuzur-cli deploy --project X --storage <uuid>` (or store selected in builder);
   assert the box's `prod.yaml` has the `aws:` block with resolved creds, the app serves `/upload`,
   and an upload lands in the team bucket. Re-deploy is idempotent and doesn't rotate creds.
5. **API-surface independence**: generate with surface = gRPC-only + storage enabled → assert a
   minimal HTTP server serves `/upload` + `/sign` on the http port; repeat with REST and with both.

## Out of scope (explicit)
- Custom S3 endpoint / Cloudflare R2 / MinIO support (separate task; manual server config for now).
- Native gRPC upload/sign RPCs (endpoints are HTTP-only; always available regardless of surface).
- Entity/field-scoped upload endpoints (replaced by the generic `/upload` + `/sign`).
- Changing how existing platform (`UploadRecordFieldFile`) uploads work.
