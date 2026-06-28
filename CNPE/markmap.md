---
title: Certified Cloud Native Platform Engineer (CNPE)
markmap:
  colorFreezeLevel: 2
---

# CNPE

## Platform Architecture and Infrastructure (15%)
### Platform Design
- Platform as a product
  - Platform team + product roadmap
  - User personas & golden paths
- Reference architectures
  - CNCF Platforms WG reference architecture
  - Control plane / data plane split
- Multi-tenancy & isolation
  - Namespace + ResourceQuota + LimitRange
  - vCluster / Capsule for soft multi-tenancy
  - Network isolation via NetworkPolicy
### Kubernetes Foundations
- Cluster architecture
  - Control plane (kube-apiserver, etcd, scheduler)
  - Node components (kubelet, kube-proxy, CRI)
- Workload scheduling & resource management
  - requests/limits, QoS classes
  - nodeAffinity / taints & tolerations
  - PriorityClass / Pod topology spread
- Networking & storage abstractions
  - CNI (Cilium / Calico)
  - Service / Ingress / Gateway API
  - CSI driver, StorageClass, PVC/PV
### Infrastructure Provisioning
- Infrastructure as Code
  - Terraform / OpenTofu
    - HCL modules, `terraform plan/apply`, remote state
  - Crossplane compositions
    - Provider + Managed Resource
    - Composition + XRD self-service API
- Cluster lifecycle (Cluster API concepts)
  - Cluster API CRDs (`Cluster`, `MachineDeployment`)
  - clusterctl bootstrap
  - Provider (CAPA/CAPZ/CAPG)

## GitOps and Continuous Delivery (25%)
### GitOps Foundations
- Git as single source of truth
  - Config repo, signed commits, protected branches
- Pull-based reconciliation
  - In-cluster agent polls Git (no push creds)
- Drift detection & self-healing
  - Argo CD `selfHeal: true` / auto-sync
  - Flux prune + reconcile interval
### GitOps Tooling
- Argo CD
  - Applications, ApplicationSets
    - `Application` CR, sync waves/hooks
    - ApplicationSet generators (Git, list, cluster)
  - App-of-apps pattern
    - Parent Application referencing child apps
- Flux
  - Kustomize / Helm controllers
    - `GitRepository` source CR
    - `Kustomization` + `HelmRelease` CRs
  - Image automation (`ImagePolicy`, `ImageUpdateAutomation`)
### Continuous Delivery
- Progressive delivery
  - Canary, blue-green
    - Traffic split via service mesh / Gateway API
  - Argo Rollouts
    - `Rollout` CR, `AnalysisTemplate`, metric checks
  - Flagger automated canary
- Environment promotion strategies
  - Folder-per-env in config repo
  - Image digest promotion via PR
- Rollback and release management
  - `kubectl rollout undo`, Argo CD history rollback
  - Helm release revisions (`helm rollback`)

## Platform APIs and Self-Service Capabilities (25%)
### Self-Service Abstractions
- Custom Resource Definitions (CRDs)
  - `kind: CustomResourceDefinition`
  - OpenAPI v3 schema, additionalPrinterColumns
- Platform APIs / golden paths
  - CR as a service request (e.g. `kind: PostgresDB`)
  - Score (score.yaml) workload abstraction
- Internal Developer Platforms (IDP)
  - Backstage / developer portals
    - `catalog-info.yaml`, Software Catalog
    - Backstage Kubernetes/Argo CD plugins
  - Service catalogs and templates
    - Backstage Software Templates (scaffolder)
    - Port / Cortex alternatives
### Kubernetes Extensibility
- Operator pattern
  - Controllers & reconciliation loops
    - controller-runtime Reconcile() loop
    - Informer / work queue / cache
  - Custom controllers
    - Kubebuilder / Operator SDK scaffolding
    - OLM (Operator Lifecycle Manager)
- Admission control & webhooks
  - ValidatingWebhookConfiguration
  - MutatingWebhookConfiguration
  - CEL validation rules in CRDs
### Provisioning Workflows
- Crossplane providers & compositions
  - provider-aws/azure/gcp Managed Resources
  - Composition Functions (Pipeline mode)
- On-demand environment provisioning
  - Ephemeral preview env per PR
  - vCluster / namespace provisioning via CR

## Observability and Operations (20%)
### Telemetry Signals
- Metrics (Prometheus)
  - `/metrics` endpoint, exporters, PromQL
- Logs (aggregation pipelines)
  - Fluent Bit/Fluentd → Loki/Elasticsearch
  - Structured JSON logging
- Traces (OpenTelemetry)
  - OTLP, OpenTelemetry Collector
  - Jaeger / Tempo backend, W3C trace context
- Events & alerting
  - Kubernetes `Event` objects, CloudEvents
### Monitoring & Alerting
- Prometheus / Alertmanager
  - PrometheusRule (recording/alerting rules)
  - Alertmanager routing/silencing/receivers
- Grafana dashboards
  - PromQL panels, provisioned dashboards as JSON
- SLOs / SLIs / error budgets
  - SLI = good/total ratio
  - error budget burn-rate alerts (Pyrra / Sloth)
### Operations
- Incident response & on-call
  - Runbooks, PagerDuty/Opsgenie escalation
  - Blameless postmortem
- Capacity planning & autoscaling
  - HPA, VPA, Cluster Autoscaler
    - HPA on CPU/custom metrics (`HorizontalPodAutoscaler`)
    - Cluster Autoscaler / Karpenter node scaling
    - KEDA event-driven scaling
- Troubleshooting & remediation
  - `kubectl describe` / `logs` / `events`
  - `kubectl debug` ephemeral container
  - CrashLoopBackOff / OOMKilled diagnosis

## Security and Policy Enforcement (15%)
### Policy as Code
- Kyverno
  - `ClusterPolicy` (validate/mutate/generate)
- OPA / Gatekeeper
  - Rego, `ConstraintTemplate` + Constraint
- Admission policy enforcement
  - ValidatingAdmissionPolicy (CEL, built-in)
### Kubernetes Security
- RBAC & least privilege
  - `Role`/`ClusterRole` + bindings, ServiceAccount
- Pod Security Standards
  - Pod Security Admission (restricted profile)
  - seccomp / runAsNonRoot / drop capabilities
- Secrets management
  - External Secrets, Vault concepts
    - External Secrets Operator + `ExternalSecret`
    - HashiCorp Vault / sealed-secrets / SOPS
### Supply Chain Security
- Image signing & verification (Cosign)
  - Sigstore Cosign keyless signing (OIDC)
  - Kyverno verifyImages policy
- SBOM
  - Syft generate, SPDX / CycloneDX format
- CI/CD pipeline hardening
  - SLSA provenance attestation
  - Trivy/Grype scan gate
- Network policies & zero trust
  - `NetworkPolicy` (default-deny)
  - Cilium L7 policy / mTLS via mesh
