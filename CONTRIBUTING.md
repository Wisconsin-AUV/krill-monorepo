# Contributing to Krill

Read this before opening your first PR.

## Getting set up

Install these tools:

| Tool | Used for |
|---|---|
| [Go 1.26+](https://go.dev/dl/) | `api/` |
| [Node 22+](https://nodejs.org) and [pnpm](https://pnpm.io/installation) | `web/` |
| [uv](https://docs.astral.sh/uv/) | `worker/` |
| [buf](https://buf.build/docs/installation) | `proto/` codegen |
| [sqlc](https://docs.sqlc.dev/en/latest/overview/install.html) | `api/` query codegen |
| [ffmpeg](https://ffmpeg.org/download.html) | Frame extraction when running `api/` on the host |
| [just](https://github.com/casey/just) | task runner |
| [golangci-lint v2](https://golangci-lint.run/welcome/install/) | Go lint and format |
| Docker | Postgres, MinIO, images |

Then:

```sh
cp .env.example .env
just setup   # install deps for every project
just dev     # start Postgres + MinIO
just api
just web
```

## Branches

We use a lightweight git-flow with two long-lived branches.

| Branch    | Purpose                                                     |
|-----------|-------------------------------------------------------------|
| `develop` | Integration branch. All feature work merges here.           |
| `master`  | Production. Every commit on `master` is a tagged release.   |

### Short-lived branches

Branch from `develop` unless noted. Name branches `<type>/<short-description>` in kebab-case, using the same types as commit messages. Include the issue number when there is one.

```
feat/click-to-track
fix/42-drift-flag-threshold
docs/labeling-guideline-bins
chore/bump-vite
```

| Branch | From      | Merges into                        | Use for                 |
|---|-----------|------------------------------------|-------------------------|
| `feat/*`, `fix/*`, `docs/*`, `refactor/*`, `perf/*`, `test/*`, `build/*`, `chore/*` | `develop` | `develop`                          | Normal work             |
| `release/vX.Y.Z` | `develop` | `master`, then back into `develop` | Preparing a release     |
| `hotfix/vX.Y.Z` | `master`  | `master`, then back into `develop` | Urgent production fixes |

Keep branches small and short-lived. If a PR drags on for more than a few days, you probably have feature creep and should split it. You may want to use [stacked PRs](https://docs.github.com/en/pull-requests/how-tos/stacked-pull-requests).

### Merging

- Feature branches merge into `develop` with a **squash merge**. The squashed commit message must follow Conventional Commits (it is usually the PR title).
- `release/*` and `hotfix/*` merge into `master` with a **merge commit** so the history on `master` maps one-to-one to releases.
- After a release or hotfix lands on `master`, merge `master` back into `develop` so the version bump and any fixes are not lost.
- Delete your branch after it merges.
- Rebase on `develop` to pick up changes. Do not merge `develop` into your feature branch.

### Cutting a release

1. Branch `release/vX.Y.Z` from `develop`.
2. Bump the version (see [Versioning](#versioning)) and update `CHANGELOG.md`.
3. Only bug fixes, docs, and version bumps go on a release branch. No new features.
4. Open a PR into `master`. Once approved, merge it with a merge commit.
5. Tag the merge commit on `master` and push the tag:
   ```sh
   git checkout master && git pull
   git tag -a vX.Y.Z -m "vX.Y.Z"
   git push origin vX.Y.Z
   ```
6. Merge `master` back into `develop`.
7. On the box, check out the tag and rebuild:
   ```sh
   git fetch --tags && git checkout vX.Y.Z
   just build vX.Y.Z
   docker compose -f deploy/docker-compose.yml --profile app --profile gpu up -d
   ```

### Hotfixes

1. Branch `hotfix/vX.Y.Z` from `master` (bump the patch version).
2. Fix the bug, bump the version, update `CHANGELOG.md`.
3. PR into `master`, merge, tag, deploy as above.
4. Merge `master` back into `develop`.

### Rolling back

Check out the previous tag on the box and rebuild. Database migrations must be backwards compatible with the previous release so a rollback never requires a manual schema change.

## Versioning

We use [Semantic Versioning 2.0.0](https://semver.org). The whole monorepo shares one version, and tags are `vMAJOR.MINOR.PATCH`.

| Bump | When | Commit types that trigger it |
|---|---|---|
| **MAJOR** | Breaking change to the protobuf API, the export format, or the database in a way that needs manual migration | Any commit with `!` or a `BREAKING CHANGE:` footer |
| **MINOR** | New backwards-compatible feature | `feat` |
| **PATCH** | Backwards-compatible fix | `fix`, `perf` |

Other types (`docs`, `chore`, `refactor`, `test`, `build`, `ci`, `style`) do not bump the version on their own. They ship with the next release.

While we are `0.y.z`, the API is not stable. Breaking changes bump MINOR instead of MAJOR. We move to `1.0.0` once the tool is used for real labeling sessions.

Pre-releases use a suffix: `v0.3.0-rc.1`. Build metadata is not used.

The version lives in:

- `web/package.json` (`version`)
- `worker/pyproject.toml` (`project.version`) and `worker/src/krill_worker/__init__.py`
- The Go API receives it at build time from the tag (`-ldflags "-X main.version=..."`)

Bump all of them together on the release branch.

### Changelog

`CHANGELOG.md` follows [Keep a Changelog](https://keepachangelog.com). Add entries under `## [Unreleased]` in the PR that makes the change. On release, rename that section to the new version and date.

## Commit messages

We use [Conventional Commits 1.0.0](https://www.conventionalcommits.org).

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Type

| Type | Use for |
|---|---|
| `feat` | A new feature |
| `fix` | A bug fix |
| `perf` | A performance improvement |
| `refactor` | Code change that neither fixes a bug nor adds a feature |
| `docs` | Documentation only |
| `test` | Adding or fixing tests |
| `build` | Build system, Dockerfiles, dependencies |
| `ci` | CI configuration |
| `chore` | Anything else that does not touch source or tests |
| `style` | Formatting only, no code change |
| `revert` | Reverts a previous commit |

### Scope

Optional but encouraged. Use the area of the repo:

`api`, `web`, `worker`, `proto`, `deploy`, `docs`, `m0`, or a feature name like `export`, `verify`, `tracker`, `auth`. Leave it out for changes that span everything.

### Subject

- Imperative case: "add", not "added" or "adds"
- Lowercase first letter, no trailing period
- 72 characters max for the whole first line

### Body

Optional.

### Footer

- `BREAKING CHANGE: <description>` for breaking changes. You can also put `!` after the type or scope: `feat(proto)!: rename Track.role to Track.attribute`
- `Refs: #123` or `Closes: #123` to link issues

### Examples

```
feat(tracker): propagate SAM 3 masks through a clip

fix(export): split train/val by video instead of by frame

Frame-level splits leaked near-identical frames into validation and
inflated mAP.

Closes: #58

feat(proto)!: move role from Annotation to Track

BREAKING CHANGE: Annotation.role is removed. Clients must read role
from the parent Track.

chore(web): bump vite to 8.3
docs: add bins section to labeling guideline
```

### Other commit rules

- One logical change per commit. Do not mix a refactor with a feature.
- Do not hand-edit generated code. Change the `.proto` or `.sql` and run `just gen`.
- Never commit secrets, `.env`, video footage, frames, or model weights. `.gitignore` covers the common cases, but check `git status` before committing.

## AI policy

You may use AI tools (ChatGPT/Codex, Claude, Cursor, etc.) to write code, tests, and docs, under two rules.

### AI code is your responsibility

You must understand and be able to justify every line of AI-written code you commit, as if you wrote it yourself. In review, "the AI wrote it" is not an answer. If you cannot explain why a line is there, what it does, and why it is correct, remove it or rewrite it until you can. You are responsible for its bugs, its security, and its licensing.

### Disclose AI use

- **Commits:** add an `Assisted-by:` trailer naming the tool and model to any commit with meaningful AI-written content. Claude Code and other harnesses may automatically append a `Co-Authored-By:` trailer; which satisfies this rule.
  ```
  feat(export): add perceptual-hash dedup

  Assisted-by: Claude Code (Claude Opus 4.5)
  ```
  For squash merges, keep the trailer in the final squashed commit message.
- **PRs:** fill in the AI disclosure section of the PR template: which tool, and which parts of the change it wrote.

Autocomplete of a line or two does not need disclosure. When in doubt, disclose.

## Pull requests

- PR title follows Conventional Commits. It becomes the squash commit message.
- Target `develop` (or `master` for release and hotfix branches).
- Fill in the PR template, including the AI disclosure.
- Run `just lint` and `just test` before asking for review.
- At least one approving review before merging.
- Keep PRs focused. Under 400 changed lines (excluding generated code and lockfiles) is a good target.
- UI changes include a screenshot or short clip.

## Code standards

### General

- Match the style of the code around you.
- Comments explain why something non-obvious is done. Do not restate the code or narrate history ("added for X", "old version did Y", "TODO later").
- Prefer clear names over comments.
- No dead code or commented-out code.
- Handle errors; do not swallow them.

### Protobuf (`proto/`)

- Follows `buf lint` STANDARD rules.
- Never reuse or renumber a field. Mark removed fields `reserved`.
- `buf breaking` should pass against `master`. If it cannot, the change is breaking and needs a `!`.
- Generated code in `api/gen`, `web/src/gen`, and `worker/gen` is committed so each project builds without buf. Regenerate with `just gen` in the same commit as the `.proto` change.

### Go (`api/`)

- `golangci-lint run` and `golangci-lint fmt` (gofumpt + goimports) must be clean.
- Pass `context.Context` as the first argument to anything that does I/O.
- Wrap errors with context: `fmt.Errorf("load clip %d: %w", id, err)`.
- Use `log/slog` for logging.
- SQL lives in `internal/db/queries` and is compiled with sqlc.
- Migrations live in `internal/db/migrations` (goose format) and must be backwards compatible with the previous release.
- Table-driven tests where it makes sense.

### TypeScript / React (`web/`)

- `pnpm lint` (oxlint), `pnpm format:check` (Prettier), and `pnpm typecheck` must pass.
- Strict TypeScript. Usage `any` must include a comment justifying it.
- Function components and hooks only.
- Styling with Tailwind utility classes. No separate CSS files per component.
- Frame images are fetched from presigned MinIO URLs. Never send image bytes over RPC.
- Import from `@/` instead of long relative paths.

### Python (`worker/`)

- `ruff check`, `ruff format --check`, and `mypy --strict` must pass.
- Type hints on every function.
- No CUDA-only dependencies (custom CUDA extensions, NVIDIA-only flash-attn builds). The worker must run on ROCm and CUDA from the same code. Use PyTorch's built-in SDPA attention.
- Do not add `torch` to `pyproject.toml`. It comes from the Docker base image.
- New pre-labelers implement the `PreLabeler` protocol in `krill_worker/prelabelers`.

### Data and labels

- Train/val splits are always by video, never by frame.
- Display-only image corrections (CLAHE, white balance) are never applied to exported images.
- Changes to label semantics need a matching update to `docs/labeling-guideline.md`.
