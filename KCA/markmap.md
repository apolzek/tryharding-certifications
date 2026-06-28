---
title: Kyverno Certified Associate (KCA)
markmap:
  colorFreezeLevel: 2
---

# KCA

## Fundamentals of Kyverno (18%)
### Policy Engine Concepts
- Kubernetes-native policy engine (no new language; YAML)
  - policies/rules are Kubernetes CRDs (kyverno.io/v1)
  - kubectl get clusterpolicies / policies
- Policy types
  - ClusterPolicy (cluster-scoped, kind: ClusterPolicy)
  - Policy (namespaced, kind: Policy)
- rule structure: spec.rules[].name + match + (validate|mutate|generate|verifyImages)
### Kubernetes Admission Control
- admission webhooks
  - ValidatingWebhookConfiguration (kyverno-resource-validating-webhook-cfg)
  - MutatingWebhookConfiguration (kyverno-resource-mutating-webhook-cfg)
- request flow
  - AdmissionReview request -> Kyverno -> AdmissionResponse (allowed: true/false)
  - rule types evaluated: validate, mutate, generate, verifyImages
  - webhook timeoutSeconds + failurePolicy (Ignore/Fail)
### Resources & Artifacts
- YAML policy manifests applied via kubectl
- OCI images
  - push/pull policies as OCI artifacts (`kyverno oci push`)
  - storing policy bundles in registry

## Installation, Configuration, and Upgrades (18%)
### Deploying Kyverno
- Helm: `helm install kyverno kyverno/kyverno -n kyverno`
- controller deployments
  - admission-controller (webhook handling)
  - background-controller (generate, mutateExisting, background scans)
  - cleanup-controller (CleanupPolicy)
  - reports-controller (PolicyReport generation)
- CRDs installed: ClusterPolicy, Policy, PolicyException, CleanupPolicy, AdmissionReport, PolicyReport
### Controller Configuration
- ConfigMap kyverno (resourceFilters, webhooks, generateSuccessEvents)
  - resourceFilters exclude system namespaces (kube-system)
- webhook failurePolicy: Fail vs Ignore
- namespace exclusions via resourceFilters `[*,kube-system,*]`
- webhookTimeoutSeconds setting
### RBAC Configuration
- ServiceAccounts per controller (kyverno-background-controller)
- aggregated ClusterRoles (kyverno:background-controller:core)
- extra permissions for generate/mutateExisting via ClusterRole with label app.kubernetes.io/part-of: kyverno
### Reliability & Lifecycle
- High Availability
  - replicas: 3 for admission-controller
  - leader election for background/cleanup/reports controllers
- Upgrades
  - `helm upgrade kyverno`; check version compatibility matrix (Kyverno vs k8s)
  - CRD migration between major versions

## Kyverno CLI (12%)
### CLI Installation
- kubectl krew install kyverno OR standalone binary
- verify: `kyverno version`
### Core Commands
- `kyverno apply policy.yaml --resource pod.yaml`
  - offline policy evaluation, `--cluster` against live cluster
  - `--policy-report` to emit report
- `kyverno test .`
  - kyverno-test.yaml (policies, resources, results[])
  - results[].kind/.policy/.rule/.result (pass/fail/skip)
- `kyverno jp query "request.object.metadata.name"`
  - JMESPath expression evaluation, `--input`
### Workflow Integration
- CI/CD: run `kyverno test` / `kyverno apply` in pipeline gate
- dry-run validation before cluster apply

## Applying Policies (10%)
### Cluster Application
- `kubectl apply -f clusterpolicy.yaml`
- validationFailureAction: Audit (report only) vs Enforce (block)
### Resource Selection
- spec.rules[].match.any[] / match.all[]
  - resources.kinds (Pod, Deployment)
  - resources.names / resources.namespaces / resources.selector.matchLabels
  - subjects / roles / clusterRoles
- spec.rules[].exclude (same structure, negated)
### Common Policy Settings
- spec.validationFailureAction: Enforce | Audit
- spec.background: true/false (background scanning)
- spec.applyRules: All | One
- rule evaluation ordering within spec.rules[]

## Writing Policies (32%)
### Validation Rules
- validate.pattern (overlay match, e.g. spec.containers[*].securityContext.runAsNonRoot: true)
- validate.anyPattern[] (logical OR of patterns)
- validate.deny.conditions (any/all with key/operator/value)
- validate.message (failure message)
- preconditions.any[] / preconditions.all[]
- validate.foreach[] (iterate list elements)
### Mutation Rules
- mutate.patchStrategicMerge (add labels/annotations)
- mutate.patchesJson6902 (RFC 6902 op: add/replace/remove, path)
- mutate.foreach[]
- mutate.mutateExistingOnPolicyUpdate: true
- mutate.targets[] (mutate existing resources)
### Generation Rules
- generate.kind / generate.name / generate.namespace
- generate.data (inline) vs generate.clone (sourceRef)
- generate.synchronize: true (sync on source change)
- generated kinds: ConfigMap, Secret, NetworkPolicy, ResourceQuota
### Image Verification
- verifyImages[].imageReferences[]
- attestors.entries[].keys.publicKeys (Cosign key verification)
- attestors.entries[].keyless (issuer, subject, rekor.url)
- attestations[].type / .conditions (in-toto attestation checks)
- Notary verification via attestors.entries[].certificates
### Variables & Data
- variables: {{ request.object.metadata.name }}, {{ request.operation }}
- context entries
  - apiCall (urlPath, jmesPath)
  - configMap (name, namespace)
  - imageRegistry, globalReference, variable
- CEL expressions (spec.rules[].validate.cel.expressions[].expression)
### Automation Helpers
- autogen rules (auto-applied to Deployment/StatefulSet/DaemonSet/Job/CronJob from Pod rule)
  - controlled via pod-policies.kyverno.io/autogen-controllers annotation
- CleanupPolicy / ClusterCleanupPolicy
  - spec.schedule (cron) OR metadata TTL via cleanup.kyverno.io/ttl label
  - spec.match + spec.conditions

## Policy Management (10%)
### Policy Reports
- PolicyReport (namespaced) / ClusterPolicyReport (cluster)
  - kubectl get polr / cpolr
  - results[].result: pass, fail, warn, error, skip
  - summary.pass / summary.fail counts
### Exceptions
- PolicyException (kyverno.io/v2)
  - spec.exceptions[].policyName / .ruleNames[]
  - spec.match (resources excluded from enforcement)
  - enable via --enablePolicyException flag / exceptionNamespace
### Observability
- Kyverno Prometheus metrics (:8000/metrics)
  - kyverno_policy_results_total
  - kyverno_admission_requests_total
  - kyverno_admission_review_duration_seconds
  - kyverno_policy_execution_duration_seconds
