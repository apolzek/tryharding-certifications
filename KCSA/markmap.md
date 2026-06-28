---
title: Kubernetes and Cloud Native Security Associate (KCSA)
markmap:
  colorFreezeLevel: 2
---

# KCSA

## Kubernetes Cluster Component Security (22%)
### API Server
- Securing kube-apiserver
  - TLS for all API traffic
    - --tls-cert-file / --tls-private-key-file
    - --client-ca-file (verify client certs)
  - Authentication modes
    - X.509 client cert, bearer token, OIDC
  - Authorization chain
    - --authorization-mode=Node,RBAC
    - Reject default AlwaysAllow
  - Admission controllers chain
    - NodeRestriction, PodSecurity
    - --enable-admission-plugins
- Anonymous & insecure access
  - --anonymous-auth=false
  - Insecure port 8080 removed (post 1.20)
  - --profiling=false
### Controller Manager
- kube-controller-manager hardening
  - Dedicated ServiceAccount per controller
  - --use-service-account-credentials=true
  - --bind-address=127.0.0.1 (secure bind)
  - --profiling=false
  - RootCA in --root-ca-file
### Scheduler
- kube-scheduler hardening
  - --bind-address=127.0.0.1 (localhost only)
  - --profiling=false
  - Restrict scheduler ServiceAccount RBAC
### Kubelet
- Kubelet security
  - --anonymous-auth=false
  - --authorization-mode=Webhook
  - --read-only-port=0 (disable unauth port 10255)
  - --rotate-certificates=true (cert rotation)
  - Protect kubelet API port 10250
  - --protect-kernel-defaults=true
### Container Runtime
- Runtime isolation
  - containerd, CRI-O (CRI runtimes)
  - Sandboxed runtimes (gVisor runsc, Kata Containers VM)
  - RuntimeClass object (select runtime handler)
  - Security profiles
    - seccomp (syscall filtering, RuntimeDefault)
    - AppArmor (path-based MAC)
    - SELinux (label-based MAC)
### KubeProxy
- kube-proxy security
  - iptables / IPVS / nftables backend
  - Minimal ClusterRole (system:node-proxier)
  - kubeconfig file permissions 0600
  - --bind-address scoping
### Pod
- Pod-level controls
  - SecurityContext
    - runAsNonRoot: true
    - readOnlyRootFilesystem: true
    - allowPrivilegeEscalation: false
  - capabilities.drop: ["ALL"]
  - hostNetwork / hostPID / hostIPC: false
  - privileged: false
  - automountServiceAccountToken: false
### Etcd
- etcd security
  - Encryption at rest (EncryptionConfiguration, KMS provider)
  - Mutual TLS (--peer-cert-file, --client-cert-auth)
  - Restrict to control-plane network only
  - etcdctl snapshot save (backup + integrity)
### Container Networking
- Network hardening
  - CNI policy enforcement (Calico, Cilium)
  - Network segmentation (NetworkPolicy)
  - Encryption in transit (Cilium WireGuard/IPsec)
  - Drop default pod-to-pod allow-all
### Client Security
- kubeconfig & credentials
  - Protect ~/.kube/config (perms 0600)
  - Short-lived credentials (TokenRequest API)
  - kubectl auth can-i (verify access)
  - Avoid long-lived ServiceAccount tokens
### Storage
- Storage security
  - PV / PVC access modes (RWO/ROX/RWX)
  - Secret encryption at rest
  - CSI driver least-privilege RBAC
  - readOnly volume mounts

## Kubernetes Security Fundamentals (22%)
### Pod Security Standards
- Profiles
  - Privileged (unrestricted)
  - Baseline (block known escalations)
  - Restricted (hardened best practice)
    - runAsNonRoot, drop ALL caps, seccomp RuntimeDefault
### Pod Security Admissions
- PSA controller (built-in admission)
  - Modes: enforce / audit / warn
  - Namespace labels
    - pod-security.kubernetes.io/enforce=restricted
    - pod-security.kubernetes.io/enforce-version
  - Replaces PodSecurityPolicy (removed 1.25)
### Authentication
- Identity mechanisms
  - X.509 client certs (CN=user, O=group)
  - Bearer tokens
  - ServiceAccount tokens (bound, projected volume, audience+expiry)
  - OIDC integration (--oidc-issuer-url)
  - Authenticating proxy (X-Remote-User header)
### Authorization
- Authorization modes
  - RBAC (Role, ClusterRole, RoleBinding, ClusterRoleBinding)
  - Node authorization (kubelet scope)
  - ABAC (policy file, legacy)
  - Webhook (external authz service)
- Least privilege principle
  - Avoid wildcard verbs/resources (*)
  - Avoid cluster-admin binding
  - kubectl auth can-i --list
### Secrets
- Secret handling
  - etcd encryption at rest (aescbc / KMS)
  - External secret managers (HashiCorp Vault, External Secrets Operator)
  - Mount as file not env var
  - RBAC restricting get/list on secrets
### Isolation and Segmentation
- Boundaries
  - Namespaces (logical isolation)
  - NetworkPolicy (network isolation)
  - Node isolation (taints, dedicated nodes)
  - Multi-tenancy (soft vs hard, vCluster)
  - ResourceQuota / LimitRange
### Audit Logging
- Kubernetes audit
  - Audit policy levels (None, Metadata, Request, RequestResponse)
  - Backends (log file, webhook)
  - Stages (RequestReceived, ResponseComplete, Panic)
  - --audit-policy-file / --audit-log-path
### Network Policy
- NetworkPolicy resource
  - policyTypes: Ingress / Egress
  - Default deny (empty podSelector, no rules)
  - podSelector / namespaceSelector / ipBlock
  - Requires CNI enforcement (Calico/Cilium)

## Kubernetes Threat Model (16%)
### Kubernetes Trust Boundaries and Data Flow
- Trust zones
  - Control plane vs worker nodes
  - Container vs host (kernel boundary)
  - Internal cluster vs external traffic
- Data flow analysis
  - apiserver <-> etcd (mTLS)
  - kubelet <-> apiserver
  - Pod <-> Pod (CNI)
### Persistence
- Attacker persistence
  - Malicious DaemonSet (runs on all nodes)
  - Malicious CronJob (recurring execution)
  - Backdoored container images
  - Modified ClusterRoleBinding (privilege grant)
  - Static pod injection (manifests dir)
### Denial of Service
- DoS vectors
  - Resource exhaustion (missing limits/ResourceQuota)
  - API server request flooding (no rate limit)
  - etcd overload (large objects, watch storms)
  - APF (API Priority and Fairness) bypass
### Malicious Code Execution and Compromised Applications in Containers
- Compromise scenarios
  - Vulnerable application code (RCE, injection)
  - Untrusted / unscanned images
  - Container escape (privileged, host mounts)
  - Cryptomining / reverse shell payloads
### Attacker on the Network
- Network threats
  - Man-in-the-middle (unencrypted traffic)
  - Traffic interception (no mTLS)
  - Lateral movement (flat pod network)
  - ARP/DNS spoofing within cluster
### Access to Sensitive Data
- Data exposure
  - Exposed Secrets (env vars, base64 not encrypted)
  - Unprotected etcd (no encryption at rest)
  - Logs leaking credentials/tokens
  - Mounted ServiceAccount token theft
### Privilege Escalation
- Escalation paths
  - Privileged containers (privileged: true)
  - hostPath mounts (host filesystem access)
  - Overly permissive RBAC (escalate verb)
  - ServiceAccount token theft
  - Linux capabilities (CAP_SYS_ADMIN)

## Platform Security (16%)
### Supply Chain Security
- Securing the pipeline
  - Image signing (Sigstore cosign)
  - SBOM (Software Bill of Materials, SPDX/CycloneDX)
  - Provenance / attestations (SLSA, in-toto)
  - Dependency scanning (CVE detection)
### Image Repository
- Registry security
  - Private registries (Harbor)
  - Image scanning (Trivy, Clair, Grype)
  - imagePullSecrets (registry auth)
  - Trusted/minimal base images (distroless)
  - Pin by digest (sha256), avoid :latest
### Observability
- Security observability
  - Falco (runtime threat detection, syscall rules)
  - Audit log monitoring / SIEM
  - Metrics & alerting (Prometheus + Alertmanager)
  - Anomaly detection (eBPF, Tetragon)
### Service Mesh
- Mesh security
  - mTLS between services (automatic)
  - Istio (Envoy sidecar, PeerAuthentication)
  - Linkerd (lightweight proxy)
  - AuthorizationPolicy (L7 access control)
### PKI
- Certificate management
  - Cluster CA (/etc/kubernetes/pki/ca.crt)
  - cert-manager (Certificate / Issuer CRDs)
  - Certificate rotation (kubelet, CertificateSigningRequest)
  - TLS everywhere (control plane mTLS)
### Connectivity
- Secure connectivity
  - Encryption in transit (TLS, mTLS)
  - VPN / private cluster networking
  - Ingress / egress controls (NetworkPolicy, egress gateway)
  - Restrict node metadata endpoint (169.254.169.254)
### Admission Control
- Policy enforcement
  - Validating / mutating admission webhooks
  - OPA Gatekeeper (ConstraintTemplate, Rego)
  - Kyverno (policy-as-YAML, ClusterPolicy)
  - ValidatingAdmissionPolicy (CEL, built-in)
  - ImagePolicyWebhook

## Overview of Cloud Native Security (14%)
### The 4Cs of Cloud Native Security
- Cloud (provider infra, IAM)
- Cluster (apiserver, RBAC, network policy)
- Container (image, runtime, isolation)
- Code (app code, dependencies, secrets)
### Cloud Provider and Infrastructure Security
- IaaS hardening
  - IAM least privilege (roles, IRSA/Workload Identity)
  - Network security groups / firewall rules
  - Metadata service protection (IMDSv2, block pod access)
  - Node OS hardening (CIS OS benchmark)
### Controls and Frameworks
- Security controls
  - Defense in depth (layered controls)
  - Preventive (RBAC, admission)
  - Detective (audit, Falco)
  - Corrective (auto-remediation, policy)
### Isolation Techniques
- Workload isolation
  - Linux namespaces, cgroups
  - Sandboxing (gVisor, Kata Containers)
  - MAC (SELinux, AppArmor)
  - seccomp (syscall filter, RuntimeDefault)
  - User namespaces (rootless)
### Artifact Repository and Image Security
- Artifact protection
  - Image scanning (Trivy) and signing (cosign)
  - Registry RBAC / access control (Harbor)
  - Minimal / distroless / scratch images
  - Immutable tags + digest pinning
### Workload and Application Code Security
- Secure development
  - Secure coding (OWASP Top 10)
  - SAST (static analysis) / DAST (dynamic analysis)
  - Secret management in code (no hardcoded secrets, Vault)
  - Dependency / SCA vulnerability scanning (Dependabot, Snyk)

## Compliance and Security Frameworks (10%)
### Compliance Frameworks
- Standards & benchmarks
  - CIS Kubernetes Benchmark
  - NIST (SP 800-190 container security)
  - PCI-DSS, HIPAA, SOC 2
  - GDPR (data protection)
### Threat Modelling Frameworks
- Methodologies
  - STRIDE (Spoofing, Tampering, Repudiation, Info disclosure, DoS, Elevation)
  - MITRE ATT&CK for Containers
  - Attack trees
### Supply Chain Compliance
- Chain assurance
  - SLSA framework (levels 1-4, provenance)
  - Provenance verification (cosign verify-attestation)
  - Policy as code (Kyverno, OPA)
  - in-toto attestations
### Automation and Tooling
- Compliance automation
  - kube-bench (CIS benchmark checks)
  - kube-hunter (penetration testing)
  - Policy engines (OPA Gatekeeper, Kyverno)
  - Continuous scanning (Trivy operator, Falco)
