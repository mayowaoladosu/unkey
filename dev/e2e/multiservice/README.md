# Multi-resource E2E fixture

One immutable image exercises the Phase 5 runtime:

- public `web` service;
- private `api` service;
- private Node.js function;
- worker bound to the private API;
- single-region CronJob.

The public service calls both private HTTP resources, proving binding injection and Cilium caller grants. Build locally as `layerrail-e2e-multiservice:local` and load it into Minikube.
