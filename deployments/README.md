# Deployments

This repository ships a few **ready-to-run** deployment options so you can evaluate Playground quickly.

**Full deployment guides:** [hanzo.bot/guides/deployment/overview](https://hanzo.bot/guides/deployment/overview)

## Pick one

### 1) Docker Compose (fastest local evaluation)

Best if you want to try the UI + execute API in minutes on a laptop.

- Docs: [`deployments/docker/README.md`](docker/README.md)

### 2) Helm (recommended for Kubernetes)

Best for production-like installs and customization via `values.yaml`.

- Chart + docs: [`deployments/helm/playground`](helm/playground/README.md)
- Website guide: [hanzo.bot/guides/deployment/helm](https://hanzo.bot/guides/deployment/helm)

### 3) Kustomize (plain Kubernetes YAML)

Best if you want transparent manifests and minimal tooling.

- Docs: [`deployments/kubernetes/README.md`](kubernetes/README.md)
- Website guide: [hanzo.bot/guides/deployment/kubernetes](https://hanzo.bot/guides/deployment/kubernetes)

