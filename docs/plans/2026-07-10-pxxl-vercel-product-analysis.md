# Pxxl and Vercel Product Analysis

**Date:** 2026-07-10
**Purpose:** Establish the product and architecture contract for the Option C transformation of Unkey into a framework-aware deployment platform.

## Scope and method

This analysis used three evidence sources:

1. Authenticated page-by-page inspection of Pxxl and Vercel in the integrated browser.
2. Public Pxxl product pages and first-party Pxxl documentation.
3. First-party Vercel product pages, documentation, and June 2026 launch material.

No projects, deployments, domains, databases, keys, integrations, or billing resources were created or changed during the inspection. Account identifiers, tokens, environment values, and other secrets are intentionally omitted.

The date matters. Vercel introduced two relevant public beta capabilities on June 30, 2026:

- OCI container image deployments from `Dockerfile.vercel` or `Containerfile.vercel`.
- Vercel Services, which atomically builds and deploys multiple frontends and backends in one project.

Any comparison based on older assumptions that Vercel cannot deploy Dockerfiles or multi-service applications is now incorrect.

## Executive conclusion

Pxxl and Vercel solve related problems with different core abstractions.

Pxxl is primarily a container platform with a developer-friendly control plane. A project is built into an OCI image, placed in a registry, started as a long-running container on a selected runtime server, health checked, and routed through an edge proxy. The product exposes explicit ports, commands, CPU, memory, volumes, terminal access, proxy controls, and managed databases.

Vercel is primarily an immutable deployment and routing platform. A build produces a typed set of resources: static assets, functions, prerenders, route metadata, middleware, image rules, cron definitions, and now optional OCI container services. Each deployment is immutable. Mutable aliases and environment targets select which immutable deployment receives traffic.

The Unkey fork already has most of the Pxxl-shaped infrastructure engine: Git source integration, workflows, image builds, registry publication, runtime deployment, edge routing, custom domains, logs, metrics, and rollback. The missing product layer is the Vercel-shaped deployment contract.

The recommended architecture remains:

`Source -> Framework Adapter -> Immutable Deployment Manifest -> Build and Publish -> Runtime and Edge Materialization`

The first implementation slice should be the framework detection contract, not a dashboard redesign. Detection is the earliest missing seam and determines every later build, output, routing, and onboarding decision.

## Pxxl page analysis

### Public product surface

#### Home

The home page positions Pxxl as an Africa-first deployment platform for websites, APIs, full-stack applications, databases, domains, and cron jobs. The primary promise is repository-to-URL deployment with framework detection and no YAML.

Observed product claims include:

- GitHub and GitLab repository connection.
- Framework detection and automatic configuration.
- Static, API, and full-stack application deployment.
- Automatic HTTPS and custom domains.
- CDN delivery, monitoring, environment variables, teams, databases, cron jobs, and API access.
- Node.js, Go, Rust, HTML, PHP, Python, and JavaScript support.

The home page uses strong personality and informal copy. That may appeal to individual developers, but some copy is too casual for enterprise trust. Metrics such as `19.8+ frameworks` are also not credible as written because a framework count should be an integer.

Source: https://pxxl.app/

#### About

The About page clearly defines Pxxl as an Africa-first Vercel alternative. Its differentiators are Naira-aware pricing, local payment accessibility, practical backend hosting, managed databases, and a single dashboard for code, DNS, SSL, logs, and infrastructure.

Source: https://pxxl.app/about

#### Pricing

The pricing model is resource-allocation oriented rather than request-compute oriented. Plans expose project count, database count, domains, per-project CPU and memory, build minutes, artifact storage and retention, concurrency, storage, bandwidth, CDN credits, tunnels, and team seats.

Observed monthly tiers:

| Tier | Price | Project compute | Build and artifact posture |
| --- | --- | --- | --- |
| Free | Free | 0.5 vCPU, 0.75 GB RAM | 300 build minutes, 25 GB artifacts, 24-hour retention, one build at a time |
| Student | About $6.99 | 1 vCPU, 1.5 GB RAM | 5,000 build minutes, 45 GB artifacts, two-day retention, two concurrent builds |
| Pro | About $13.99 | 1.5 vCPU, 2 GB RAM | 5,000 build minutes, 250 GB artifacts, seven-day retention, five concurrent builds |
| Enterprise | About $69.99 | 2 vCPU, 4 GB RAM | 100,000 build minutes, 2 TB artifacts, 30-day retention, 15 concurrent builds |

The very low bandwidth allowances displayed for lower plans need clarification. The dashboard and public page also use slightly different labels and values in places, which creates trust risk.

Source: https://pxxl.app/pricing

#### Domains

The public domain flow combines search, registration, TLD pricing, DNS lookup, and later project attachment. This is a genuine product differentiator from a deployment-only platform.

Source: https://pxxl.app/domain

#### Status

The status page reports a load balancer, three runtime servers, one build server, and one database server. All observed components were in Denmark. This is a useful disclosure because it shows the actual topology behind broader global marketing claims.

The first render remained in a loading state for several seconds before component health appeared. A stronger status product would preserve historical uptime, incidents, response latency, and regional impact rather than only current component state.

Source: https://pxxl.app/status

#### Guides

Pxxl has a very large SEO guide catalog covering competitor alternatives, deployment recipes, migrations, frameworks, databases, domains, and African city or country variants.

The breadth creates discoverability, but many pages use visibly repeated templates. Some titles and summaries are grammatically inconsistent or duplicate the same promise. This weakens authority. LayerRail should prefer a smaller set of verified, original, task-oriented guides.

Source: https://pxxl.app/guide

### Authenticated global dashboard

The dashboard uses a persistent dark sidebar and workspace switcher. Global navigation includes:

- Overview
- Projects
- Deployments
- Database
- Domain
- Team
- CDN
- Cron Jobs
- Integrations
- API Keys
- Usage
- Billing
- Pricing
- Settings
- Help

#### Overview

The overview page leads with quota cards for projects, domains, databases, and build artifacts. It then shows recent project activity and edge traffic summaries.

This is useful for plan awareness, but it prioritizes inventory and quotas over deployment health. A deployment platform home should answer these questions first:

1. Is production healthy?
2. What changed recently?
3. Are any deployments failing or waiting?
4. Is usage or cost anomalous?
5. Which action requires attention?

#### Projects

The projects page provides search, project cards, repository context, detected language, domain count, last commit, and recency. It is visually clear and directly leads into a project workspace.

#### Deployments

The global deployments page provides cross-project search and filters. During this inspection it rendered the shell but did not populate the same deployment history visible inside the project, suggesting either delayed loading or inconsistent query behavior.

#### Database

The create flow exposed PostgreSQL, MySQL, MongoDB, Redis, MariaDB, ClickHouse, KeyDB, and DragonflyDB. Daily encrypted backups were plan-gated.

The Pxxl docs describe project and workspace ownership, region and resource selection, and explicit credential copying. Linking a database to the architecture canvas does not inject credentials. Users must add connection values as environment variables and redeploy.

This separation is technically clean but should be made explicit in the UI to avoid a false expectation that a visual connection creates network or credential bindings.

#### Domain

The global domain product covers registration, external domains, DNS, nameservers, subdomains, SSL, invoice and registrant details, proxy controls, and transfer.

The edge proxy controls are unusually deep for a developer platform:

- Force HTTPS and canonical host redirects.
- WebSocket support.
- Security headers and custom request or response headers.
- Maintenance mode.
- Rate limiting and WAF checks.
- IP, CIDR, country, method, and content-type rules.
- Circuit breaker, upstream retries, and in-flight request limits.

This is a strong model for Unkey's Frontline capabilities. These controls should become typed route policy resources rather than ad hoc domain settings.

#### Team

Pxxl calls team workspaces “Spaceships.” The creation panel supports a name, description, and avatar.

The metaphor is visually distinctive but adds terminology without adding domain clarity. “Workspace” or “Team” is easier to understand and maps better to access control, billing, and ownership.

#### CDN

Pxxl combines object storage, build artifacts, edge proxy analytics, and a points balance under CDN. Public and private assets are distinct. Private assets use short-lived signed URLs.

This breadth is useful, but CDN storage and request proxying are separate domains and should not share one ambiguous resource model.

#### Cron Jobs

Cron jobs are scheduled HTTP requests with:

- Name
- HTTP method
- URL
- Timeout
- Cron expression and presets
- Custom headers
- Manual validation and testing

This is more flexible than a function-only cron model, but it requires idempotency and retry semantics to be documented and observable.

#### Integrations

The observed create flow only offered a Pxxl OAuth application. It did not expose the provider marketplace breadth implied by the global navigation.

#### API Keys

Keys support a name, description, product scope, and read-only or read-write permission. Observed scopes included all APIs, CDN, cron, domain reselling, and sandbox.

The model is simple and appropriate for an initial platform API. It should later support resource selectors, expiration, rotation, and audit history.

#### Usage

Usage includes:

- Artifact storage and average artifact size
- Build time
- Deployment success rate
- Requests, visitors, blocked requests, and errors
- Per-project artifact and build usage
- Geographic traffic
- CDN point balance and ledger
- Daily project, deployment, deletion, and webhook activity

The page is more actionable than a raw invoice. It still mixes infrastructure utilization, security telemetry, and billing units without a clear cost attribution model.

#### Billing and pricing

Billing uses points and invoices in addition to subscriptions. The model supports local payments, but points obscure the underlying unit cost. A LayerRail billing surface should always show both the product unit and its currency effect.

#### Settings

Observed account settings include:

- Profile
- GitHub installations
- Bulk project management
- Connected OAuth apps
- Referrals
- Password and passkeys
- Billing
- Deployment notifications
- Dashboard customization
- Login history
- Signed webhooks

The signed webhook model is sound. Event subscriptions include project creation, updates, deletion, deployment creation, redeploy queueing, GitHub deployment, deployment status changes, environment updates, and profile updates. Requests include event, action, timestamp, and HMAC signature headers.

### Pxxl deployment onboarding

The Git import page lists accessible repositories with language, visibility, and update recency. Selecting Import opens a configuration review before provisioning.

Observed configuration fields:

- Project name and generated Pxxl subdomain.
- Domain suffix.
- Runtime port.
- Git branch and exact commit.
- Detected language or framework.
- Install, build, and start commands.
- Root or working directory.
- Environment variables.
- CPU and memory.
- Build server selection.
- Multi-service mode.
- Blue-green deployment.
- Autoscaling.
- Project webhooks.
- Build cache.
- Preview environment metadata.

The flow correctly delays resource creation until final confirmation.

#### Detection quality

A large Go monorepo was detected only as Go with “framework not detected” and generic commands. The generated start command assumed a root `main.go`, which is unsafe for a complex repository.

This demonstrates why a detection contract must return evidence, confidence, warnings, and an explicit “needs review” state. It must never silently turn weak evidence into authoritative commands.

#### Multi-service contradiction

The live onboarding UI can enable multiple services inside one container, each with a name, base directory, port, commands, URL, and environment overrides.

Pxxl's current docs instead recommend separate projects for a web process and worker. The docs and product therefore expose two competing service models. This needs resolution before LayerRail borrows either approach.

### Authenticated project workspace

Pxxl's project workspace is its strongest product surface.

#### Overview architecture canvas

The canvas visually connects:

- Global and project environment scopes.
- The application service.
- Port and root directory.
- Last build and redeploy action.
- Connected monitoring.
- CPU and memory snapshots.
- Additional services and notes.

This is useful when it reflects real resources and edges. It should not become a decorative diagram. Every node and edge should correspond to a typed resource or binding.

#### Deployments

The deployment list includes status, commit, branch, author, source type, build duration, artifact state, and actions.

A deployment details page includes:

- Status and total duration.
- Build start and end times.
- Branch and commit.
- Artifact size and retention state.
- Searchable timestamped build logs.
- Detected provider, framework, package manager, and runtime.
- Build and runtime server identity.
- Registry publication.
- Runtime pull and startup.
- Health check.
- Graceful rollover.
- Activation.

The observed pipeline is:

`Git clone -> Buildpack analysis -> Install -> Build -> OCI image commit -> Registry publish -> Runtime task -> Image pull -> Container start -> Health check -> Graceful rollover -> Route activation`

This is close to Unkey's existing deployment engine.

Artifact expiration disables true instant rollback. The history remains visible, but an expired image requires rebuilding the old commit. This is weaker than Vercel's immutable deployment retention contract and must be made explicit in rollback UX.

#### Live Logs

The project had no runtime logs even though deployment logs and historical runtime activity existed. Build and runtime logs are correctly separate concepts, but the empty live view needs a clear explanation of whether the container is stopped, logging is unavailable, or the retention window expired.

#### Environment Variables

The page supports individual variables, bulk editing, history, masked values, and project or global scope. It does not visibly distinguish production, preview, and development values.

For Vercel-like behavior, environment is part of the variable identity. A single project scope is insufficient.

#### Domains

The project-level page supports aliases and domains. The sampled project showed no domains here while Monitoring listed an active generated hostname. The product needs a clear distinction among generated deployment URL, project alias, branch alias, and custom domain.

#### Monitoring

Monitoring exposes runtime state, uptime, CPU, memory, domain traffic, visitors, bandwidth, latency, success rate, route diagnostics, top paths, status codes, project signals, and recent edge requests.

The route diagnostic was especially useful because it classified the failure as a stopped container and displayed the upstream address and port. This is the kind of cross-layer diagnosis LayerRail should preserve.

Observed state was contradictory in places. A deployment could be marked deployed while the current runtime was stopped, and domain counts differed between tabs. LayerRail must derive status from one explicit state model rather than page-specific interpretations.

#### Terminal

The terminal connects to the running project container and opens the application workspace. This is valuable for container workloads, but it is not a substitute for immutable deployment inspection. Mutations made through a terminal must be treated as ephemeral and clearly labeled.

#### Infra and Scaling

The page covers persistent volumes, vertical CPU and memory allocation, replicas, autoscaling, and weighted traffic between baseline and scaled runtimes.

This is a valuable LayerRail differentiator. It belongs to a runtime service resource, not the framework deployment manifest itself.

#### Issues

Issues combine GitHub-synced issues and custom operational notes. This can support incident handoff, but issue tracking should remain an integration rather than a core deployment primitive.

#### Settings

Project settings include:

- Name, port, and assigned runtime server.
- Install, build, start, and deploy-ignore configuration.
- Git repository, branch, and base directory.
- Auto-deploy and pause controls.
- Preview protection by password, token, email, team session, or none.
- Transfer, archive, and delete actions.

### Pxxl reliability findings

The inspection exposed several defects or trust risks:

- The dashboard Notifications link returned a 404.
- Multiple pages logged chart sizing warnings with negative width and height.
- Push notification setup repeatedly raised unsupported-browser errors in the embedded session.
- Some global pages loaded shells without the data visible in corresponding project pages.
- The public pricing route initially rendered only its footer before later data became available through first-party page content.
- Project domain and runtime status differed between tabs.
- Runtime logs were empty while deployment history was populated.
- Several page transitions were delayed or intercepted by animated overlays.
- The status page initially remained in a loading state.
- Marketing language implies a broad global edge network while the disclosed build and runtime topology was concentrated in Denmark.
- The guide catalog contains visible template duplication and quality inconsistencies.

These findings are reference-only. They are not evidence that the underlying platform is universally unavailable.

## Vercel page analysis

### Public positioning in July 2026

Vercel now positions itself as “Agentic Infrastructure.” The homepage groups the platform into three jobs:

1. Build agents on durable workflows, sandboxes, model gateway, and Fluid compute.
2. Ship applications with global delivery, environments, functions, and firewall.
3. Host platforms with tenant isolation, domain management, custom certificates, and preview URLs.

The homepage also announces first-class containers. This is a material expansion beyond frontend and serverless positioning.

Source: https://vercel.com/

### New Project flow

The new project flow supports:

- Natural-language generation through v0.
- Git repository URL input.
- Git provider repository import.
- Drag and drop files or folders.
- Templates.
- Team and project selection.
- Application preset detection.
- Root directory.
- Build and output settings.
- Environment variables by target environment.

The import page can detect an application preset from repository metadata. It also gates private organization imports by plan.

The most important UX behavior is that detection remains editable. Framework preset, root directory, commands, output, and variables can be reviewed before deployment.

### Team-level dashboard

#### Overview and Projects

The team overview combines:

- Search and project filtering.
- Add New action.
- Current usage against plan allowances.
- Alerts and upgrade state.
- Recent preview deployments.
- Project cards with repository, current production domain, last commit, branch, and production checklist score.

This makes deployments and usage visible without forcing a project selection.

#### Deployments

The global deployments page supports filters for date, author, environment, repository, branch, and status. Each row distinguishes Preview and Production, status, duration, project, commit, branch, author, and URL.

This is the correct global model: a deployment is first-class and queryable independently of a project page.

#### Logs, Analytics, Firewall, CDN, and Images

These team-level entry points require selecting a project when the underlying data is project-specific. This avoids presenting misleading aggregate controls where the operation needs one project context.

#### Observability

Team observability aggregates edge requests, transfer, function invocations, middleware, project request counts, query exploration, environments, and time ranges.

The key product insight is that logs and metrics share dimensions such as project, deployment, environment, route, status, region, and runtime. This enables drill-down instead of isolated charts.

#### Environment Variables

The team environment page has Project and Shared views. It groups variables by project, environment, editor, status, and sensitivity. It can warn when a likely secret was stored without the sensitive type.

This is much deeper than a flat secret list. LayerRail needs environment and sensitivity in the variable model from the beginning.

#### Domains

The team domain page supports search, filtering, external connection, transfer, purchase, bulk selection, registrar state, project connection, DNS, nameservers, Vercel CDN state, and certificate inventory.

Domain details separate:

- Registration ownership.
- DNS authority.
- Connected projects and redirects.
- DNS records.
- Nameservers.
- Managed or custom SSL certificates.

#### Connect

Vercel Connect manages OAuth connectors and short-lived access to third-party APIs. This is separate from marketplace integrations and from static environment secrets.

#### Integrations and Storage

The marketplace manages installed providers and can provision storage or service resources. Storage displays provider, plan, project attachment, status, and creation time. Credentials can be injected into project environments through integrations.

#### Flags, Agent, AI Gateway, Sandboxes, Workflows, and Images

These are platform expansion products, not Phase 1 deployment requirements. Their product shapes are still useful:

- Flags are typed resources with provider integrations.
- Agent reviews are tasks tied to pull requests and sandbox validation.
- AI Gateway has key management, model routing, usage, latency, token, and spend views.
- Sandboxes are isolated, on-demand Linux environments.
- Workflows are durable executions with retries and state.
- Images are OCI registry resources.

#### Usage

The usage page is the most complete description of Vercel's internal product model. It meters networking, edge requests, cache, ISR reads and writes, functions, active CPU, provisioned memory, edge compute, builds, artifacts, Blob, queues, cron, drains, observability, image optimization, flags, Connect, sandboxes, snapshots, and OCI image storage.

Every major primitive has an explicit billing unit. LayerRail should adopt this principle even if its prices differ.

### Team settings

Team settings include:

- General identity and URL.
- Preview suffix.
- Toolbar defaults.
- Build concurrency and machines.
- Remote cache.
- Billing and invoices.
- Members and access groups.
- Agent policy.
- Drains and alerts.
- Webhooks.
- Security and privacy.
- Deployment protection defaults.
- Passport.
- Microfrontends.
- Networking.
- Notifications.
- OAuth apps.

Security settings expose verified commit enforcement, secret policy, token revocation, audit logs, SAML, directory sync, two-factor enforcement, IP visibility, IP blocking, and deployment retention.

This is later-phase governance, but deployment retention and verified commits directly affect the core deployment contract.

### Project overview

The project overview leads with the current Production Deployment. It shows:

- Repository.
- Rollback and visit actions.
- Current deployment URL.
- Custom and generated domains.
- Status and creation actor.
- Source branch and commit.
- Production checklist.
- Recent observability.
- Active branches.

The production tile is not a mutable server. It is a pointer from production aliases to one immutable deployment.

### Project deployments

The project deployment list distinguishes Production and Preview, commit and branch, duration, actor, generated URL, and status. Retention state is visible.

A deployment detail contains five durable inspection surfaces:

1. Deployment metadata and aliases.
2. Build and runtime logs.
3. Materialized resources.
4. Source and output file tree.
5. Open Graph metadata.

The sampled deployment exposed dozens of functions and thousands of static assets as separate resources. This is the clearest proof that Vercel's unit of deployment is a manifest of heterogeneous outputs, not one opaque application container.

### Project logs and observability

Runtime logs can be searched live and filtered by method, status, host, request, message, trace, deployment, and time. Observability breaks the same traffic into edge requests, transfer, functions, compute, middleware, ISR, cache, and routes.

LayerRail should avoid separate log products for every service. It needs one event schema with resource-specific fields.

### Project CDN and Firewall

The CDN page maps request volume to regions and shows edge requests, cache hit rate, misses, bypasses, ISR hit rate, reads, and revalidations.

The Firewall page shows active protection, custom rules, allowed and denied traffic, events, and denied IPs. Security is evaluated before application compute.

### Project settings

#### Build and Deployment

Framework settings include an auto-detected preset plus explicit overrides for build, output, install, and development commands. Root directory and unaffected-project skipping are separate. Ignored build behavior is configurable. Node version, concurrency, build machine, deployment checks, rolling releases, and production build priority are modeled independently.

This is the strongest reference for the first slice. Framework detection should produce defaults, not permanently own user configuration.

#### Environments

Vercel has Local, Preview, and Production by default. Pro and Enterprise can add custom environments with branch tracking, domains, and imported variables.

#### Git

Git settings cover repository connection, pull request and commit comments, deployment events, commit status, verified commits, LFS, and deploy hooks.

#### Deployment Protection

Protection combines a scope with a method. Standard protection covers non-production URLs. Methods include Vercel authentication, password, trusted IP, and Passport. Trusted OIDC sources, automation bypass secrets, shareable links, source-map protection, and OPTIONS exceptions support real automation workflows.

#### Functions

Function settings separate Fluid compute, regions, CPU and memory, and advanced runtime controls. These are runtime policies applied to function resources in the deployment manifest.

#### Cron Jobs

Cron is declared in source configuration or Build Output API output. Production deployment activates the schedule. Rollback restores the cron definitions attached to the selected deployment.

#### Security

Project security includes source and build-log protection, fork protection, OIDC federation, and deployment retention.

#### Advanced

Advanced settings include directory listing, skew protection, and bulk redirects. Skew protection pins framework-managed client requests to the deployment that served the page.

## Vercel's current deployment contract

### Build Output API v3

The Build Output API is a filesystem specification under `.vercel/output`.

At minimum, `.vercel/output/config.json` contains `version: 3`. It can also define routes, image rules, wildcard domain values, static overrides, build cache paths, framework metadata, and cron jobs.

The primitive directories are:

- `.vercel/output/static` for immutable static assets served from the CDN.
- `.vercel/output/functions/<path>.func` for functions.
- `.vc-config.json` inside each function for runtime, handler, memory, duration, environment, region, architecture, and streaming behavior.
- Edge runtime functions with an edge entry point and declared environment use.
- Prerender configuration beside a function for expiration, grouping, cache bypass, query behavior, initial headers, status, and optional fallback.

Sources:

- https://vercel.com/docs/build-output-api
- https://vercel.com/docs/build-output-api/configuration
- https://vercel.com/docs/build-output-api/primitives

### Immutable deployments and mutable targets

Preview branches and commits receive generated URLs. Commit-specific URLs identify exact deployments. Branch URLs follow the latest deployment for the branch. Production domains point to the current production deployment.

Promotion and rollback are alias operations over immutable deployments, except promoting a Preview deployment to Production may rebuild it with Production environment variables.

Sources:

- https://vercel.com/docs/deployments/environments
- https://vercel.com/docs/deployments/promoting-a-deployment
- https://vercel.com/docs/instant-rollback

### Cache hierarchy

Vercel distinguishes:

- CDN response cache.
- Durable ISR cache scoped to a deployment and function region.
- Runtime data cache inside compute.
- Image optimization cache.

ISR metadata is known at build time. This enables request collapsing, stale serving during revalidation, globally coordinated purges, and deployment-specific rollback safety.

Sources:

- https://vercel.com/docs/cdn
- https://vercel.com/docs/incremental-static-regeneration

### Container Images, public beta

As of June 30, 2026, Vercel detects `Dockerfile.vercel` or `Containerfile.vercel`, builds an OCI image, stores it in Vercel Container Registry, and runs it as a Vercel Function on Fluid compute.

The contract includes:

- HTTP server on `PORT`, default 80.
- Git or CLI deployment.
- Preview deployment for each push.
- Automatic scale out and scale in.
- Production scale-in after five idle minutes.
- Preview scale-in after 30 idle seconds.
- `SIGTERM` with a 30-second grace period.
- `stdout` and `stderr` observability.
- Stateless instances.
- Function limits and Active CPU pricing.
- No Secure Compute or Static IP support yet.

Sources:

- https://vercel.com/docs/functions/container-images
- https://vercel.com/blog/dockerfile-on-vercel

### Services, public beta

Vercel Services allows multiple frameworks and runtimes in one project and one atomic deployment. Each service builds independently but shares the deployment lifecycle, routing table, environment, preview, and rollback.

Services are internal by default. Top-level rewrites expose selected services. Bindings inject private service URLs into callers. Container services can set `runtime: container`.

This model supersedes the older assumption that every monorepo service must be a separate Vercel project. Separate projects remain valid for independently deployed applications, while Services is for components that should deploy and roll back atomically.

Sources:

- https://vercel.com/docs/services
- https://vercel.com/blog/vercel-services-run-full-stack-on-vercel

## Reliability observations from the Vercel session

The Vercel inspection also produced client and API errors:

- Repeated unused preload warnings.
- Aborted analytics and status requests.
- A team alerts request returned 500.
- Some security and project pages returned 403, 404, or 428 requests while the surrounding page remained usable.
- A project-domain request reported a missing authentication token.
- The new-project flow logged a page error and a 429 during repository loading.
- Several pages rendered skeletons for noticeable periods.

The core dashboard remained substantially usable despite these failures. LayerRail should still treat partial API failure as a normal UI state and ensure one failed optional request cannot blank a page.

## Direct comparison

| Capability | Pxxl | Vercel | LayerRail implication |
| --- | --- | --- | --- |
| Core runtime | Long-running OCI container | Static, functions, prerenders, middleware, and beta containers | Preserve Unkey containers, add heterogeneous outputs |
| Deployment identity | Build history plus retained image | Immutable deployment with stable ID and URLs | Make deployment immutable and addressable |
| Production | Active container and proxy route | Mutable aliases target immutable deployment | Model environment target and aliases explicitly |
| Preview | Temporary PR container, 72-hour lifecycle | Commit and branch URLs with environment semantics | Support both immutable commit URL and mutable branch alias |
| Framework support | Detection produces commands for a container | Adapter produces infrastructure outputs | Detection must feed adapters, not only command templates |
| Static sites | Served from generated output or preview server | First-class CDN assets | Add static asset publishing independent of app container |
| SSR and APIs | Long-running server | Functions, Fluid compute, or container | Select output mode per adapter and project |
| Multi-service | UI offers services in one container; docs also recommend separate projects | Beta Services build independently and deploy atomically | Use a project manifest with independently built services |
| Rollback | Relaunch retained image; expired artifacts rebuild | Repoint aliases to retained immutable deployment | Retention must be explicit; no false instant rollback claim |
| Domains | Registration, DNS, SSL, deep proxy controls | Registration, DNS, SSL, aliases, protection | Reuse Frontline and type domain policy resources |
| Scaling | Explicit CPU, memory, replicas, autoscaling | Fluid autoscaling and Active CPU | Keep explicit container controls; add scale-to-zero later |
| Terminal and volumes | First-class | Containers are stateless; sandbox is separate | Keep as container-specific LayerRail differentiator |
| Environment variables | Global and project scopes | Team/project plus environment, branch, and sensitivity | Add environment to variable identity early |
| Observability | Runtime and proxy metrics | Shared event model across edge, cache, function, deployment | Define one event schema with deployment and resource dimensions |
| Databases | Native managed resources | Marketplace providers and bindings | Defer marketplace depth, preserve native LayerRail data roadmap |
| Edge cache | Asset CDN plus reverse proxy | Framework-aware multi-tier cache and ISR | Static/CDN separation is a Phase 1 requirement; ISR is Phase 2 |
| Security | Deep domain proxy controls | Firewall, deployment protection, OIDC, governance | Map Frontline policies to project and domain resources |
| Billing | Allocations, points, and plan limits | Explicit metered units per primitive | Always expose real units and currency effect |

## Target architecture for Option C

### Source contract

A deployment source should be immutable and provenance-rich:

- Provider and repository identity.
- Commit SHA.
- Branch or pull request context.
- Root directory.
- Trigger actor and trigger kind.
- Source archive or Git reference digest.
- Detection version and adapter version.

### Framework detection contract

Detection should return a typed result rather than a display label:

- Preset identifier.
- Confidence level.
- Evidence list.
- Package manager.
- Runtime family and version hint.
- Root directory candidates.
- Install, build, development, and start command defaults.
- Output mode: static, function, server, container, or mixed.
- Output directory candidates.
- Port requirement.
- Dockerfile candidates.
- Monorepo and service candidates.
- Warnings and unresolved decisions.

User overrides must remain separate from detected defaults. Re-detection must not erase explicit user choices.

### Immutable deployment manifest

The adapter output should eventually describe:

- Manifest schema version.
- Source provenance.
- Adapter identity and version.
- Services.
- Static assets and metadata.
- Dynamic functions.
- OCI container services.
- Prerendered routes and revalidation policy.
- Middleware.
- Route phases, redirects, rewrites, headers, and mitigations.
- Image optimization rules.
- Cron jobs.
- Environment references, never secret values.
- Health checks and runtime requirements.
- Cache metadata.
- Domains and alias intentions.

The manifest must be framework-neutral after adapter output. Frontline, Krane, the control plane, and the dashboard should not need framework-specific branches.

### Deployment and environment model

Use these explicit concepts:

- **Deployment:** Immutable materialized manifest and artifacts.
- **Environment:** Named policy and variable scope such as Preview or Production.
- **Environment target:** Mutable pointer to a deployment.
- **Deployment URL:** Immutable URL for one deployment.
- **Branch alias:** Mutable URL following the latest successful deployment for a branch.
- **Production alias:** Mutable custom or generated domain targeting the current production deployment.
- **Promotion:** Move an environment target or alias to a deployment.
- **Rollback:** Move a target back to a retained prior deployment without rebuilding.
- **Redeploy:** Build a new deployment from the same source and current configuration.

This vocabulary prevents the common mistake of treating rollback and redeploy as synonyms.

### Request pipeline

The desired framework-neutral request pipeline is:

`PoP -> TLS and firewall -> Deployment selection -> Route phases -> Middleware -> Cache lookup -> Static or prerender resource -> Function or container compute -> Response policy`

The deployment selection step resolves aliases, branch targets, rolling-release buckets, and skew-protection pins before resource routing.

### Service model

A project may contain one or more independently built services. Services that share one manifest deploy and roll back atomically. Services that need independent release cadence should remain separate projects.

Bindings should be typed and private by default. Public routing should require an explicit route from the project-level routing table.

### Observability model

Every event should carry stable dimensions where applicable:

- Workspace
- Project
- Environment
- Deployment
- Service
- Resource type and resource ID
- Domain or generated host
- Region
- Route
- Status or outcome
- Trigger and actor
- Trace and request correlation IDs

Sensitive fields and client IP policy must be handled at ingestion, not only at display time.

## What to copy

### From Pxxl

- Review-before-provision onboarding.
- Explicit port, command, CPU, memory, health, and scaling controls for container workloads.
- Architecture canvas backed by real resources.
- Route diagnostics that connect domain, proxy, container, and listener state.
- Domain registration and deep proxy controls.
- Terminal and volume support as container-specific tools.
- Local-currency accessibility and clear resource allocations.

### From Vercel

- Immutable deployments and generated URLs.
- Framework preset as editable defaults.
- Build Output API style manifest.
- Static, function, prerender, middleware, route, image, and cron primitives.
- Environment-scoped variables and branch overrides.
- Promotion, staged production, instant rollback, and retention semantics.
- Unified logs, metrics, resources, source, and provenance on deployment details.
- Framework-aware CDN behavior.
- Monorepo unaffected-project skipping.
- Skew protection.
- Atomic multi-service deployments and private bindings.

## What not to copy

- Pxxl's ambiguous overlap among project, service, container, workspace, and “Spaceship.”
- Pxxl's points-based obscuring of real resource costs.
- Pxxl's conflicting multi-service guidance.
- Pxxl's generated command confidence when detection evidence is weak.
- Pxxl's SEO page multiplication and inconsistent copy quality.
- Vercel's rapidly expanding global navigation before the core product is complete.
- Vercel's plan and add-on complexity.
- UI behavior where optional analytics or status requests can produce noisy failures.
- Either platform's tendency to show different status interpretations on different pages.

## Recommended Phase 1 sequence

1. Framework detection contract with evidence, confidence, warnings, and tests.
2. Deployment source and framework preset persistence with explicit user overrides.
3. Immutable deployment and environment-target model.
4. Adapter interface and first static adapter.
5. Static asset publication through Frontline without an application container.
6. Existing container fallback represented as one manifest service.
7. Generated immutable deployment URL, branch alias, and production alias.
8. Promotion, redeploy, and rollback semantics.
9. Build and runtime logs attached to deployment and service IDs.
10. Dashboard onboarding backed by these APIs.

The first adapter should be a narrow static framework such as Vite or plain static output. SSR and API support should follow after the static path proves the manifest, alias, and routing seams.

## First slice decision

The first slice is **framework detection contract and tests**.

It is selected because:

- The current dashboard and Railpack build path already contain detection-related behavior that must be located and reconciled.
- Both Pxxl and Vercel place detection before configuration and provisioning.
- Pxxl demonstrates the failure mode of weak detection becoming unsafe commands.
- Vercel demonstrates the correct product behavior: detected preset plus editable overrides.
- A pure detector can be tested without Kubernetes, Depot, Rask, or a full local platform.
- The detector becomes a stable input to later adapters and onboarding APIs.

The implementation plan must first map existing GitHub tree access, Railpack detection, dashboard onboarding, project persistence, and tests. It must avoid introducing a second competing detector if one already exists.

## AGPLv3 considerations

The transformed platform remains derived from AGPLv3 software. Planning and implementation must preserve:

- License and copyright notices.
- Corresponding source availability for modified network-accessible software as required by AGPLv3.
- Clear separation between upstream Unkey code, LayerRail modifications, and third-party components.
- Compatible licensing for imported framework detection tables, adapters, and build tooling.
- Original copy and visual assets rather than copied proprietary product text or artwork.

## Source index

### Pxxl

- https://pxxl.app/
- https://pxxl.app/about
- https://pxxl.app/pricing
- https://pxxl.app/domain
- https://pxxl.app/guide
- https://pxxl.app/status
- https://docs.pxxl.app/
- https://docs.pxxl.app/llms.txt
- https://docs.pxxl.app/dashboard/deploy-project.md
- https://docs.pxxl.app/dashboard/projects.md
- https://docs.pxxl.app/dashboard/domains.md
- https://docs.pxxl.app/dashboard/database.md
- https://docs.pxxl.app/dashboard/import-projects.md
- https://docs.pxxl.app/dashboard/settings-usage-webhooks.md
- https://docs.pxxl.app/deployment-reference.md
- https://docs.pxxl.app/framework-recipes.md
- https://docs.pxxl.app/integrations/github-previews.md
- https://docs.pxxl.app/integrations/cdn-proxy.md
- https://docs.pxxl.app/troubleshooting/deployments.md
- https://docs.pxxl.app/troubleshooting/proxy-security.md
- https://docs.pxxl.app/api/pxxl-deploy.md
- https://docs.pxxl.app/api/pxxl-diagnostics.md
- https://docs.pxxl.app/api/pxxl-env.md
- https://docs.pxxl.app/api/pxxl-logs-open.md

### Vercel

- https://vercel.com/
- https://vercel.com/docs/build-output-api
- https://vercel.com/docs/build-output-api/configuration
- https://vercel.com/docs/build-output-api/primitives
- https://vercel.com/docs/deployments/environments
- https://vercel.com/docs/deployments/promoting-a-deployment
- https://vercel.com/docs/instant-rollback
- https://vercel.com/docs/rolling-releases
- https://vercel.com/docs/skew-protection
- https://vercel.com/docs/cdn
- https://vercel.com/docs/regions
- https://vercel.com/docs/incremental-static-regeneration
- https://vercel.com/docs/routing-middleware
- https://vercel.com/docs/environment-variables
- https://vercel.com/docs/monorepos
- https://vercel.com/docs/security/deployment-protection
- https://vercel.com/docs/cron-jobs
- https://vercel.com/docs/functions/container-images
- https://vercel.com/docs/services
- https://vercel.com/blog/dockerfile-on-vercel
- https://vercel.com/blog/vercel-services-run-full-stack-on-vercel

## Unresolved questions

- Which existing Railpack detection output is stable enough to wrap rather than duplicate?
- Should detection live in Go near the build workflow, TypeScript near onboarding, or in a shared generated contract with implementations at both boundaries?
- Which source-tree API can provide enough evidence without cloning a full repository during onboarding?
- What minimum manifest version can represent static and existing container deployments without prematurely committing to ISR semantics?
- What retention guarantee is required before LayerRail can call rollback “instant”?
