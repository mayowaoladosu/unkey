# Framework Detection Contract Plan

**Date:** 2026-07-10
**Branch:** `mayowaoladosu/feat/framework-aware-deploy-setup`
**Starting commit:** `dbcf08eaac9978c5186dcd4c1f8a18d0a232bf40`
**Status:** Implemented and integrated

## Goal

Add a small, pure, typed framework detector that predicts a repository's framework during onboarding without cloning or building the repository.

The result is advisory. Railpack remains the authoritative build-time detector. The dashboard detector must explain its evidence, avoid inventing commands, and return an explicit unknown state when evidence is insufficient. It may recommend commands only when they are grounded in a declared package script and an identified package manager.

## Why this is the first slice

Both Pxxl and Vercel detect a framework before provisioning. The current Unkey onboarding flow can already read the connected repository tree through `github.getRepoTree`, but it only uses that tree to validate Dockerfile paths. Railpack detects the real build plan later on the build machine, after the user has already configured the deployment.

A pure detection contract fills this gap without changing the deployment engine or database schema. It also creates the stable input needed for later framework adapters and a deployment-output manifest.

## Existing architecture

### Onboarding seam

The repository picker saves the GitHub connection and selected branch through `github.selectRepository`. The next step renders deployment build settings.

Relevant files:

- `web/apps/dashboard/app/(app)/[workspaceSlug]/projects/[projectId]/apps/new/steps/select-repo/index.tsx`
- `web/apps/dashboard/app/(app)/[workspaceSlug]/projects/[projectId]/apps/new/steps/configure-deployment/index.tsx`
- `web/apps/dashboard/lib/trpc/routers/github.ts`

### Repository metadata seam

`getRepositoryTree` returns repository-relative paths for the selected branch. The current API rejects truncated trees by returning no tree to callers.

Relevant files:

- `web/apps/dashboard/lib/github.ts`
- `web/apps/dashboard/lib/trpc/routers/github.ts`
- `web/apps/dashboard/app/(app)/[workspaceSlug]/projects/[projectId]/apps/[appId]/(overview)/settings/components/build-settings/use-repo-tree.ts`

### Authoritative build-time detection

When no Dockerfile is selected, the control-plane worker runs Railpack on the actual Git checkout. Railpack generates the build plan and remains authoritative for install, build, runtime, packages, and image construction.

Relevant files:

- `svc/ctrl/worker/deploy/railpack.go`
- `svc/ctrl/worker/deploy/build.go`
- `svc/ctrl/worker/deploy/railpack_test.go`

## Confirmed public test seam

The public seam under test is:

```text
RepositorySnapshot -> detectFramework() -> FrameworkDetection
```

The user request explicitly prioritizes a framework detection contract and requires test-first implementation. This plan selects that seam after repository archaeology.

The detector accepts already-fetched, untrusted repository metadata:

- Blob paths from a Git tree.
- Parsed root `package.json`, or `null` when it is absent or invalid.

It returns a serializable result:

- Framework preset or `null`.
- Runtime family or `null`.
- Confidence level.
- Structured evidence.
- Package manager or `null`.
- Candidate application roots.
- Grounded install, build, and start defaults.
- Output mode and output directory.
- Dockerfile candidates.
- Recommended Dockerfile only when the choice is unambiguous.
- Advisory build strategy.
- Monorepo and service-root hints.
- Warnings for ambiguity or incomplete input.
- Unresolved decisions with explicit options.

## Domain rules

### Advisory, not authoritative

The pure detector does not perform persistence. The integration stores its advisory output as provenance, while Railpack owns final build planning. Advisory command defaults are allowed only when the repository supplies the evidence:

- Install commands require a lockfile or a valid root `packageManager` declaration.
- Build and start commands require matching `scripts` entries in `package.json` and an identified package manager.
- The detector must never synthesize ecosystem commands such as `go run main.go` from a generic marker.

These defaults remain separate from effective build settings. They reach the existing Railpack input only after the user explicitly applies them, and remain editable afterward.

### Evidence and confidence

Confidence is based on independent evidence:

- `high`: an exact framework dependency in `package.json`, or a framework-specific marker pair.
- `medium`: a distinctive framework file marker without dependency evidence.
- `low`: a generic ecosystem marker such as `go.mod`, `Cargo.toml`, `requirements.txt`, or `index.html`.
- `none`: no framework match.

Evidence must be structured so UI copy can change without changing detector behavior.

### Framework precedence

Specific frameworks outrank their underlying tools. Examples:

- Next.js outranks Vite or React.
- NestJS outranks Express.
- Django outranks generic Python.
- Rails requires both `Gemfile` and `config/application.rb`.

### Dockerfile handling

Dockerfile discovery is separate from framework detection. A repository can contain both a detectable framework and one or more Dockerfiles.

Supported conventional names include:

- `Dockerfile`
- `Dockerfile.vercel`
- `Containerfile`
- `Containerfile.vercel`
- Case variants of those names

Candidates are sorted by path depth and then lexicographically.

A Dockerfile is recommended only when:

- A conventional file exists at repository root, or
- Exactly one candidate exists in the repository.

Multiple nested candidates produce a warning and no recommendation. The detector must not silently pick one service in a monorepo.

Files under dependency or generated directories such as `node_modules`, `.git`, and `.next` are ignored.

### Package manager handling

An explicit valid root `packageManager` declaration takes precedence. Otherwise lockfiles determine the package manager using this order:

1. pnpm
2. Yarn
3. Bun
4. npm

Multiple lockfile families produce a warning. A root `package.json` without a lockfile does not prove which manager the repository expects.

### Output handling

Output recommendations are preset metadata, not build results:

- A repository-owned Dockerfile implies `container` output.
- Plain static and Vite-like presets can recommend a known static output directory.
- Server frameworks can report `server` output without inventing a start command.
- Hybrid frameworks can report `mixed` output while leaving their materialized directory unset.
- Unknown repositories report `unknown` output.

Railpack or a future framework adapter remains responsible for producing and validating actual output.

### Root and service hints

Root candidates are derived from root and nested ecosystem markers after ignored dependency and generated directories are removed. Workspace declarations and files such as `pnpm-workspace.yaml`, `turbo.json`, `nx.json`, and `lerna.json` are monorepo evidence.

If multiple plausible service roots exist, the detector returns a `select-root-directory` unresolved decision. It does not silently select the shallowest service.

### Input safety

Malformed `package.json` input must not throw. Prototype-inherited dependency keys must not count as declared dependencies. Paths are normalized to forward slashes and leading `./` is removed.

## TDD slices

### Slice 1: Unknown repository

Write one failing test showing that an empty snapshot returns the complete explicit unknown contract with no evidence, package manager, root, defaults, Dockerfile recommendation, warnings, or unresolved decisions.

Add the smallest public types and `detectFramework()` implementation to pass.

### Slice 2: Dependency detection and precedence

Add one test for Next.js dependency evidence and high confidence. Make it pass.

Add one test proving a specific framework wins over an underlying tool. Make it pass.

### Slice 3: Non-Node marker detection

Add one test for a marker-based ecosystem such as Go. Make it pass.

Add one test for a framework-specific marker pair such as Rails. Make it pass.

### Slice 4: Dockerfile strategy

Add one test for an unambiguous root Dockerfile. Make it pass.

Add one test for multiple nested Dockerfiles. Require sorted candidates, no recommendation, and an ambiguity warning. Make it pass.

Add one test proving generated or dependency directories are ignored. Make it pass.

### Slice 5: Package manager and malformed input

Add one test for lockfile detection. Make it pass.

Add one test for conflicting lockfiles and a warning. Make it pass.

Add one test proving malformed manifest input does not throw. Make it pass.

### Slice 6: Grounded defaults and outputs

Add one test proving lockfile and declared package scripts produce package-manager-specific install, build, and start defaults. Make it pass.

Add one test proving absent scripts remain `null` rather than receiving invented commands. Make it pass.

Add one test for a known static output and one for a repository-owned container output. Make them pass in separate cycles.

### Slice 7: Root and service hints

Add one test for nested application markers in a monorepo. Require sorted root and service hints plus a `select-root-directory` unresolved decision. Make it pass.

Add one test proving generated and dependency directories do not become service roots. Make it pass.

## Files

### Add

- `web/apps/dashboard/lib/deploy/framework-detection.ts`
- `web/apps/dashboard/lib/deploy/framework-detection.test.ts`

### Initial contract isolation

- Database schema or generated SQL.
- Railpack or BuildKit workflows.
- GitHub API calls.
- tRPC routers.
- Onboarding UI.
- Persisted build settings.

These areas remained unchanged until the pure contract was reviewed and stable. The later integration phase then added the scoped GitHub, persistence, and onboarding changes documented below.

## Validation

The Windows clone has CRLF task wrappers, so `.mise/tasks/*` cannot execute directly under WSL. Validation will run the exact underlying commands with the locked Linux toolchain and will not modify `.mise/config.toml` or `.mise/mise.lock`.

Targeted test:

```bash
mise exec -- pnpm --dir=web/apps/dashboard exec vitest run lib/deploy/framework-detection.test.ts
```

Targeted type check:

```bash
mise exec -- pnpm --dir=web/apps/dashboard typecheck
```

Formatting check for changed files:

```bash
mise exec -- pnpm --dir=web exec biome check --line-ending=crlf apps/dashboard/lib/deploy/framework-detection.ts apps/dashboard/lib/deploy/framework-detection.test.ts
```

The explicit line ending matches this Windows checkout. It can be omitted in an LF checkout.

Repository state check:

```bash
git status --short
git diff --check
```

## Acceptance criteria

- The detector is pure, synchronous, serializable, and has no network or database dependency.
- Every positive framework match includes structured evidence.
- Unknown input is a first-class result, not an exception.
- Dockerfile ambiguity is visible and never silently resolved.
- Command defaults are grounded in repository declarations and never guessed from generic files.
- Output, root, monorepo, and service hints are explicit parts of the contract.
- User commands and persisted settings are untouched.
- Railpack remains documented as authoritative.
- The targeted tests fail before implementation and pass afterward.
- Type checking and formatting pass, or pre-existing failures are reported exactly.

## AGPLv3 considerations

The detector implementation and matcher data are original code in the AGPLv3 repository. No proprietary Vercel matcher tables or product source are copied. Public product behavior is used only as architectural reference.

## Integration completed

The detector is connected end to end:

1. The GitHub boundary reads an immutable tree SHA and the exact bounded root `package.json` blob through installation authentication.
2. The workspace-scoped `github.detectFramework` mutation revalidates the locked current source, then persists source, tree SHA, version, fingerprint, evidence, and reviewable defaults.
3. The configure-deployment step shows the detected framework, confidence, build strategy, package manager, warnings, and unresolved decisions.
4. Users explicitly apply only unambiguous root, Dockerfile, and build-command defaults by exact fingerprint. Source validation and application are atomic, null recommendations preserve user overrides, and detection provenance remains separate from effective `app_build_settings` values.
5. Accepted root, Dockerfile, and build-command values use the existing control-plane path into BuildKit and `RAILPACK_BUILD_CMD`; no unsupported Railpack configuration was introduced.
6. Railpack and Dockerfile builds continue to verify the actual commit at build time.

## Deferred scope

- Reading nested manifests after a user selects a monorepo root.
- Additional framework adapters and immutable deployment-manifest resources.
- Install and start command overrides, which are not currently supported by the pinned Railpack integration and are therefore not passed as authoritative settings.
