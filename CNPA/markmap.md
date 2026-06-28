---
title: Certified Cloud Native Platform Engineering Associate (CNPA)
markmap:
  colorFreezeLevel: 2
---

# CNPA

## Platform Engineering Core Fundamentals (36%)
### Platform Concepts
- Platform goals & approaches
  - Reduce cognitive load
    - Abstract Kubernetes YAML behind a typed API
    - Score (score.yaml) workload spec
    - Self-service templates (Backstage Software Templates)
  - Golden paths / paved roads
    - Opinionated scaffolding (cookiecutter / `backstage scaffolder`)
    - Reference repo with Helm chart + CI workflow
    - Spotify "golden path" tutorial pattern
- Platform as a product
  - Product manager + roadmap for the platform
  - User personas (app dev, SRE, security)
  - Platform team topology (Team Topologies: platform team)
- Platform architecture & capabilities
  - Building blocks and abstractions
    - Control plane vs data plane separation
    - Capability: provisioning, delivery, observability, security
  - Composable platform (CNCF Platforms WG whitepaper)
### Declarative Resource Management
- Desired state vs actual state
  - `kubectl apply` (declarative) vs `kubectl create` (imperative)
  - Reconciliation loop (control loop)
- Kubernetes manifests & YAML
  - `apiVersion` / `kind` / `metadata` / `spec` / `status`
  - Kustomize overlays (`kustomization.yaml`, `kubectl apply -k`)
  - Helm chart (`Chart.yaml`, `values.yaml`, templates/)
- Reconciliation mindset
  - Level-triggered vs edge-triggered
  - Idempotent apply / convergence
### Application Environments & Infrastructure
- Dev / staging / prod environments
  - Namespace-per-env vs cluster-per-env
  - Per-environment `values-<env>.yaml`
- Environment promotion
  - Promote via Git PR (dev → staging → prod folders)
  - Image tag / digest promotion
- Infrastructure abstraction
  - Crossplane Composition / XRD as the abstraction
  - Terraform module as reusable unit
### DevOps Practices
- Collaboration & shared ownership
  - "You build it, you run it"
  - Blameless postmortems
- Automation culture
  - Everything-as-code in Git
  - Trunk-based development
### CI Fundamentals
- Continuous Integration pipelines
  - GitHub Actions (`.github/workflows/*.yml`)
  - GitLab CI (`.gitlab-ci.yml`) / Tekton Pipeline
- Build, test, artifact creation
  - `docker build` / Buildpacks / `ko build`
  - Unit/lint stage (`golangci-lint`, `pytest`)
  - Push image to OCI registry (`docker push`, Harbor)
### Continuous Delivery & GitOps
- CD pipelines
  - Deploy stage gated on tests
  - Argo CD / Flux apply manifests
- Git as source of truth
  - Config repo separate from app repo
  - Signed commits / protected branches
- Declarative delivery
  - GitOps Argo CD `Application` CR
  - Flux `Kustomization` / `HelmRelease` CR

## Platform Observability, Security, and Conformance (20%)
### Observability
- Signals
  - Metrics
    - Prometheus counters/gauges/histograms
    - PromQL queries
  - Logs
    - Structured JSON logs
    - Loki / Fluent Bit / Fluentd collectors
  - Traces
    - Spans / trace context (W3C `traceparent`)
    - Jaeger / Tempo backend
  - Events
    - Kubernetes `Event` objects
    - CloudEvents spec
- Tooling
  - Prometheus + Alertmanager
  - OpenTelemetry Collector (OTLP protocol)
  - Grafana dashboards
### Security
- Kubernetes security
  - RBAC: `Role` / `RoleBinding` / `ClusterRole`
  - Pod Security Standards (privileged/baseline/restricted)
  - Secrets (`kind: Secret`, base64) + encryption at rest
- Secure service communication
  - mTLS between services
  - Service mesh (Istio / Linkerd) sidecar
- CI/CD pipeline security
  - Image scanning (Trivy / Grype)
  - SBOM generation (Syft → SPDX/CycloneDX)
  - Image signing (Sigstore Cosign)
### Governance & Conformance
- Policy engines for governance
  - OPA/Gatekeeper (`ConstraintTemplate`, Rego)
  - Kyverno (`ClusterPolicy` YAML)
- Compliance enforcement
  - Admission control (validating/mutating webhook)
  - CIS Kubernetes Benchmark (kube-bench)
  - Conformance testing (Sonobuoy)

## Continuous Delivery & Platform Engineering (16%)
### CI Pipeline Overview
- Pipeline stages and gates
  - build → test → scan → publish → deploy
  - Quality gate (coverage threshold, approval)
- CI/CD relationships
  - CI artifact (image digest) feeds CD
  - Trigger CD on registry push
### GitOps
- GitOps basics & workflows
  - Pull-based reconciliation
    - Agent in-cluster polls Git (Argo CD / Flux)
  - Argo CD / Flux concepts
    - Argo CD `Application` / sync status
    - Flux `GitRepository` + `Kustomization`
- GitOps for application environments
  - Environment-per-branch / per-folder
  - Argo CD ApplicationSet (generator per env)
  - Drift detection & auto-sync / self-heal
### Operations
- Incident response
  - Detection (alert fires from Prometheus rule)
  - Remediation (runbook execution)
  - Rollback (`kubectl rollout undo`, Argo CD rollback)

## Platform APIs and Provisioning Infrastructure (12%)
### Kubernetes Control Plane
- Reconciliation loop
  - Watch → diff → act on `spec`/`status`
- Controllers and control loops
  - kube-controller-manager (Deployment, ReplicaSet)
  - Informer / work queue pattern
### Self-Service Platform APIs
- Custom Resource Definitions (CRDs)
  - `kind: CustomResourceDefinition`
  - OpenAPI v3 schema validation
- Platform APIs as abstractions
  - Custom Resource (CR) as a service request
  - kubectl as the platform interface
### Operator Pattern
- Kubernetes operator pattern
  - Operator = CRD + custom controller
  - Operator SDK / Kubebuilder / controller-runtime
- Controllers automating operations
  - Reconcile() loop owns lifecycle (e.g. DB backup)
### Infrastructure Provisioning
- Infrastructure as Code
  - Terraform / OpenTofu (`.tf`, `terraform apply`)
  - Pulumi (general-purpose language IaC)
- Crossplane / provisioning concepts
  - Provider (provider-aws) installs Managed Resources
  - Composition + XRD = self-service infra API

## IDPs and Developer Experience (8%)
### Internal Developer Platforms
- Simplified platform access
  - Single portal / `platform` CLI
- API-driven service catalogs
  - Backstage Software Catalog (`catalog-info.yaml`)
- Developer portals
  - Backstage (IDP portal, plugins)
  - Port / Cortex alternatives
### Developer Experience
- Self-service workflows
  - Backstage Software Templates (scaffolder)
  - One-click environment provisioning
- AI/ML in platform automation
  - LLM-assisted manifest/PR generation
  - AIOps anomaly detection on metrics

## Measuring your Platform (8%)
### Platform Metrics
- Platform efficiency
  - Provisioning lead time
  - Cost per environment (FinOps / OpenCost)
- Team productivity
  - Onboarding time to first deploy
  - Adoption rate of golden paths
### Delivery Metrics
- DORA metrics
  - Deployment frequency
  - Lead time for changes
  - Change failure rate
  - Mean time to restore (MTTR)
- Developer experience surveys
  - SPACE framework
  - DevEx / DXI score
