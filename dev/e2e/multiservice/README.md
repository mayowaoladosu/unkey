# Multi-resource E2E fixture

One immutable image exercises the Phase 5 runtime:

- public `web` service;
- private `api` service;
- private Node.js function;
- worker bound to the private API;
- single-region CronJob.

The public service calls both private HTTP resources, proving binding injection and Cilium caller grants. Build locally as `layerrail-e2e-multiservice:local` and load it into Minikube.

`outputs.json` is the environment's authored output contract. Apply it to the
production `app_runtime_settings.outputs` row before creating a deployment with
the prebuilt image. A successful single-region run has five
`deployment_resources`, five regional topologies, one CronJob, three Services
(two private), and public routes that select only the `web` resource.
