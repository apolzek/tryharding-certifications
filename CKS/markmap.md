---
title: CKS - Certified Kubernetes Security Specialist
markmap:
  colorFreezeLevel: 2
---

# CKS

## Cluster Setup (15%)

### Use Network security policies to restrict cluster level access
- NetworkPolicy resource (networking.k8s.io/v1)
  - Default-deny baseline
    - Deny all ingress
      - `podSelector: {}` (selects all pods in ns)
      - `policyTypes: [Ingress]`
      - `spec.ingress: []` (no rules = nothing allowed)
    - Deny all egress
      - `policyTypes: [Egress]`
      - `spec.egress: []`
    - Deny all both
      - `policyTypes: [Ingress, Egress]` with empty rule arrays
  - Selectors
    - `podSelector.matchLabels` (e.g. `role: db`)
    - `podSelector.matchExpressions` (In/NotIn/Exists)
    - `namespaceSelector.matchLabels` (e.g. `kubernetes.io/metadata.name: prod`)
    - `ipBlock.cidr` + `ipBlock.except`
      - e.g. `cidr: 172.17.0.0/16`, `except: [172.17.1.0/24]`
  - Rules
    - `ingress[].from[]` (podSelector / namespaceSelector / ipBlock)
    - `egress[].to[]`
    - `ingress[].ports[]`
      - `protocol: TCP|UDP|SCTP`
      - `port: 6379` or named port
      - `endPort: 8080` (port range)
    - Allow DNS egress
      - `to: namespaceSelector kube-system` + `podSelector k8s-app: kube-dns`
      - `ports: [{protocol: UDP, port: 53}, {protocol: TCP, port: 53}]`
  - CNI must support policies
    - Supported: Calico, Cilium, Weave Net, Antrea
    - NOT supported: flannel (no policy enforcement)
  - Combination semantics
    - Additive allow-list (union of all matching policies)
    - No explicit deny rules in core API
  - Scope
    - Namespaced object (`metadata.namespace`)
    - Cluster-wide: `CiliumClusterwideNetworkPolicy` CRD
  - Verify / debug
    - `kubectl get netpol -A`
    - `kubectl describe netpol <name> -n <ns>`
    - Test: `kubectl exec <pod> -- nc -zv <svc> <port>`

### Use CIS benchmark to review the security configuration of Kubernetes components (etcd, kubelet, kubedns, kubeapi)
- kube-bench (Aqua) runs CIS Kubernetes Benchmark
  - `kube-bench run --targets master,node,etcd,policies`
  - `kube-bench run --benchmark cis-1.8`
  - `kube-bench --config-dir /cfg --config /cfg/config.yaml`
  - Run as Job: `kubectl apply -f job-master.yaml`
  - Output states: PASS / FAIL / WARN / INFO with remediation text
- kube-apiserver hardening (/etc/kubernetes/manifests/kube-apiserver.yaml)
  - `--anonymous-auth=false`
  - `--authorization-mode=Node,RBAC`
  - `--insecure-port=0` (removed in 1.20+)
  - `--profiling=false`
  - `--audit-log-path=/var/log/kubernetes/audit.log`
  - `--tls-cert-file` + `--tls-private-key-file`
  - `--kubelet-certificate-authority=<ca>`
  - `--service-account-lookup=true`
  - `--enable-admission-plugins=NodeRestriction`
- kubelet hardening
  - Config file: /var/lib/kubelet/config.yaml
    - `authentication.anonymous.enabled: false`
    - `authorization.mode: Webhook`
    - `readOnlyPort: 0`
    - `protectKernelDefaults: true`
    - `rotateCertificates: true`
    - `tlsCipherSuites: [TLS_AES_256_GCM_SHA384, ...]`
  - Flags: `--read-only-port=0`, `--anonymous-auth=false`
  - Reload: `systemctl restart kubelet`
- etcd hardening (/etc/kubernetes/manifests/etcd.yaml)
  - `--client-cert-auth=true`
  - `--peer-client-cert-auth=true`
  - `--cert-file` / `--key-file` / `--trusted-ca-file`
  - `--auto-tls=false`, `--peer-auto-tls=false`
  - Encryption at rest via apiserver EncryptionConfiguration
- File permissions / ownership (CIS checks)
  - `chmod 600 /etc/kubernetes/manifests/*.yaml`
  - `chown root:root /etc/kubernetes/manifests/*.yaml`
  - PKI keys: `chmod 600 /etc/kubernetes/pki/*.key`
  - kubelet config: `chmod 600 /var/lib/kubelet/config.yaml`
  - etcd data dir: `chmod 700 /var/lib/etcd`

### Properly set up Ingress with TLS
- Ingress resource (networking.k8s.io/v1) + IngressController
  - Controllers: ingress-nginx, traefik, HAProxy, Contour
  - `spec.ingressClassName: nginx`
- TLS termination
  - `spec.tls[].hosts: [app.example.com]`
  - `spec.tls[].secretName: <tls-secret>`
  - Secret type `kubernetes.io/tls` with keys `tls.crt` + `tls.key`
  - `kubectl create secret tls web-tls --cert=tls.crt --key=tls.key`
- Generate certificate
  - `openssl req -x509 -newkey rsa:4096 -keyout tls.key -out tls.crt -days 365 -nodes -subj "/CN=app.example.com"`
  - cert-manager: `ClusterIssuer` + `Certificate` CRD (ACME/Let's Encrypt)
- nginx annotations
  - `nginx.ingress.kubernetes.io/ssl-redirect: "true"`
  - `nginx.ingress.kubernetes.io/force-ssl-redirect: "true"`
  - `nginx.ingress.kubernetes.io/backend-protocol: "HTTPS"`
- Verify: `curl -v https://app.example.com` / `openssl s_client -connect app:443`

### Protect node metadata and endpoints
- Cloud metadata endpoint 169.254.169.254 (IMDS)
  - Block via NetworkPolicy egress
    - `egress[].to[].ipBlock.cidr: 0.0.0.0/0`
    - `ipBlock.except: [169.254.169.254/32]`
  - AWS IMDSv2: `HttpTokens=required`, `HttpPutResponseHopLimit=1`
    - `aws ec2 modify-instance-metadata-options --http-tokens required`
- Protect kubelet API
  - Kubelet port 10250: enable Webhook authz, anonymous-auth=false
  - Read-only port 10255: set `readOnlyPort: 0`
  - Test exposure: `curl -sk https://<node>:10250/pods`
- Restrict API server reachability
  - Private endpoint / firewall allow-list
  - `--bind-address` and load balancer source ranges
- Cloud identity scoping
  - AWS IRSA (`eks.amazonaws.com/role-arn` SA annotation)
  - GKE Workload Identity / Azure AAD Workload Identity
  - Avoid attaching broad node instance roles

### Verify platform binaries before deploying
- Verify checksums
  - `sha256sum kubectl` compared to dl.k8s.io `.sha256`
  - `curl -L https://dl.k8s.io/release/v1.31.0/bin/linux/amd64/kubectl.sha256`
  - `echo "<hash>  kubectl" | sha256sum --check`
- Verify signatures
  - cosign on release artifacts: `cosign verify-blob --signature ...`
  - GPG verify: `gpg --verify kubernetes.tar.gz.asc`
- Validate then install
  - Confirm `kubectl version --client` matches expected
  - Move to PATH only after verification (`install -m 0755 kubectl /usr/local/bin/`)

## Cluster Hardening (15%)

### Use Role Based Access Controls to minimize exposure
- Objects (rbac.authorization.k8s.io/v1)
  - Role / ClusterRole
    - `rules[].apiGroups: [""]` (core) or `["apps"]`
    - `rules[].resources: ["pods", "secrets"]`
    - `rules[].verbs: ["get", "list", "watch", "create", "update", "delete"]`
    - `rules[].resourceNames: ["my-config"]`
  - RoleBinding / ClusterRoleBinding
    - `subjects[].kind: User|Group|ServiceAccount`
    - `roleRef.kind: Role|ClusterRole`
    - `roleRef.name: <role>`
- Least privilege
  - Avoid `verbs: ["*"]` and `resources: ["*"]`
  - Never bind built-in `cluster-admin` to users/SAs
  - Prefer namespaced Role + RoleBinding over ClusterRole
  - Restrict with `resourceNames` to specific objects
- Verify access
  - `kubectl auth can-i create pods --as=jane -n dev`
  - `kubectl auth can-i --list --as=system:serviceaccount:dev:app`
  - `kubectl auth can-i get secrets --as-group=devs`
- Advanced
  - Aggregated ClusterRoles via `aggregationRule.clusterRoleSelectors`
  - Built-in roles: view / edit / admin / cluster-admin
  - Audit bindings: `kubectl get clusterrolebindings -o wide`

### Exercise caution in using service accounts e.g. disable defaults, minimize permissions on newly created ones
- Default ServiceAccount per namespace
  - Disable token auto-mount
    - On SA: `automountServiceAccountToken: false`
    - On Pod: `spec.automountServiceAccountToken: false`
  - Patch default SA: `kubectl patch sa default -p '{"automountServiceAccountToken":false}'`
- Token handling (K8s 1.24+)
  - No auto-generated Secret token by default
  - TokenRequest API: `kubectl create token <sa> --duration=1h`
  - Projected token volume: `serviceAccountToken.expirationSeconds`
- Dedicated SA per workload
  - `kubectl create serviceaccount app-sa -n dev`
  - `spec.serviceAccountName: app-sa` in Pod
  - Scoped RoleBinding to that SA only
- Never grant SA cluster-wide perms; audit `system:serviceaccount:*` bindings

### Restrict access to Kubernetes API
- Authentication
  - Disable anonymous: `--anonymous-auth=false`
  - X509 client certs: `--client-ca-file`
  - OIDC: `--oidc-issuer-url`, `--oidc-client-id`, `--oidc-username-claim`
  - Static token / bearer tokens (avoid in prod)
- Authorization
  - `--authorization-mode=Node,RBAC`
  - Node authorizer limits kubelet to its own node objects
- Network reachability
  - NetworkPolicy / firewall / private API endpoint
  - `--bind-address` not 0.0.0.0 where avoidable
- Admission control
  - `--enable-admission-plugins=NodeRestriction,PodSecurity,...`
  - NodeRestriction limits kubelet self-modification
  - `--disable-admission-plugins=...` for unsafe ones
- Disable insecure ports + `--service-account-lookup=true`
- Rotate certs: `kubeadm certs renew all`

### Upgrade Kubernetes to avoid vulnerabilities
- kubeadm upgrade flow (control plane)
  - `kubeadm upgrade plan`
  - `apt-mark unhold kubeadm && apt-get install -y kubeadm=1.31.0-*`
  - `kubeadm upgrade apply v1.31.0`
- Node upgrade flow
  - `kubectl drain <node> --ignore-daemonsets --delete-emptydir-data`
  - `apt-get install -y kubelet=1.31.0-* kubectl=1.31.0-*`
  - `systemctl daemon-reload && systemctl restart kubelet`
  - `kubectl uncordon <node>`
- Constraints
  - Upgrade one minor version at a time (skew policy)
  - Verify: `kubectl get nodes` shows new VERSION
- Track CVEs
  - kubernetes.io/docs/reference/issues-security/official-cve-feed
  - Subscribe to kubernetes-security-announce

## System Hardening (10%)

### Minimize host OS footprint (reduce attack surface)
- Remove packages
  - `apt-get purge <pkg>` / `dnf remove <pkg>`
  - List installed: `dpkg -l` / `rpm -qa`
- Disable/mask services
  - `systemctl disable --now <service>`
  - `systemctl mask <service>`
  - List: `systemctl list-unit-files --type=service --state=enabled`
- Close unused ports
  - Audit: `ss -tulpn`, `netstat -tulpn`
- Minimal/immutable distros
  - Flatcar Container Linux, Bottlerocket, Talos, minimal Ubuntu
- Patch: `unattended-upgrades` / `dnf-automatic`

### Using least-privilege identity and access management
- OS user accounts
  - Remove unused: `userdel <user>`; list: `cat /etc/passwd`
  - Find UID 0 dupes: `awk -F: '$3==0' /etc/passwd`
  - No shared accounts; lock: `passwd -l <user>`
- sudo restrictions
  - `/etc/sudoers.d/` entries; validate with `visudo -c`
  - Avoid `NOPASSWD: ALL`
- SSH hardening (/etc/ssh/sshd_config)
  - `PermitRootLogin no`
  - `PasswordAuthentication no`
  - `PubkeyAuthentication yes`
  - Reload: `systemctl reload sshd`
- File permission auditing
  - Find SUID/SGID: `find / -perm -4000 -o -perm -2000 2>/dev/null`
  - World-writable: `find / -perm -002 -type f`

### Minimize external access to the network
- Host firewall
  - ufw: `ufw default deny incoming`, `ufw allow 6443/tcp`, `ufw enable`
  - iptables: `iptables -A INPUT -p tcp --dport 22 -j ACCEPT`
  - firewalld: `firewall-cmd --add-port=6443/tcp --permanent`
- Limit SSH exposure
  - Restrict source: `ufw allow from 10.0.0.0/8 to any port 22`
- Identify open sockets
  - `ss -tulpn`, `lsof -i`, `nmap -sT localhost`
- Disable unneeded network services
  - `systemctl disable --now avahi-daemon cups`

### Appropriately use kernel hardening tools such as AppArmor, seccomp
- AppArmor
  - Profile dir: /etc/apparmor.d/
  - `aa-status` (list loaded profiles)
  - `apparmor_parser -r /etc/apparmor.d/<profile>` (load)
  - Modes: enforce (`aa-enforce`) vs complain (`aa-complain`)
  - Pod field (K8s 1.30+): `securityContext.appArmorProfile.type: Localhost`
    - `securityContext.appArmorProfile.localhostProfile: k8s-deny-write`
  - Legacy annotation: `container.apparmor.security.beta.kubernetes.io/<ctr>: localhost/<profile>`
  - Types: RuntimeDefault, Localhost, Unconfined
- seccomp
  - Profile JSON dir: /var/lib/kubelet/seccomp/profiles/
  - Pod: `securityContext.seccompProfile.type: RuntimeDefault`
    - `Localhost` + `localhostProfile: profiles/audit.json`
    - `Unconfined` (disables filtering)
  - Profile JSON: `defaultAction: SCMP_ACT_ERRNO`, `syscalls[].action: SCMP_ACT_ALLOW`
  - Inspect violations: `dmesg | grep seccomp`
- SELinux (alternative MAC)
  - `securityContext.seLinuxOptions.level: "s0:c123,c456"`
  - `getenforce` / `setenforce 1`
- Linux capabilities
  - `securityContext.capabilities.drop: ["ALL"]`
  - `securityContext.capabilities.add: ["NET_BIND_SERVICE"]`
  - Inspect: `capsh --print`, `getcap`

## Minimize Microservice Vulnerabilities (20%)

### Use appropriate pod security standards
- Pod Security Standards (PSS) profiles
  - Privileged (unrestricted)
  - Baseline (minimally restrictive)
  - Restricted (hardened best-practice)
- Pod Security Admission (PSA, GA 1.25, replaces PodSecurityPolicy)
  - Namespace labels
    - `pod-security.kubernetes.io/enforce: restricted`
    - `pod-security.kubernetes.io/audit: baseline`
    - `pod-security.kubernetes.io/warn: restricted`
    - `pod-security.kubernetes.io/enforce-version: v1.31`
  - Apply: `kubectl label ns prod pod-security.kubernetes.io/enforce=restricted`
  - Cluster default via AdmissionConfiguration `PodSecurity` plugin
- SecurityContext controls (restricted profile)
  - `runAsNonRoot: true`
  - `runAsUser: 1000`, `runAsGroup: 3000`, `fsGroup: 2000`
  - `allowPrivilegeEscalation: false`
  - `privileged: false`
  - `readOnlyRootFilesystem: true`
  - `capabilities.drop: ["ALL"]`
  - `seccompProfile.type: RuntimeDefault`
- Policy engines (PSP alternatives)
  - OPA Gatekeeper
    - `ConstraintTemplate` (Rego in `spec.targets.rego`)
    - `Constraint` CRD (e.g. K8sRequiredLabels)
    - `kubectl get constraints`
  - Kyverno
    - `ClusterPolicy` with `validate` / `mutate` / `generate`
    - `validationFailureAction: Enforce`
    - `kubectl get cpol`

### Manage Kubernetes secrets
- Secret object (base64 only, NOT encrypted by default)
  - `kubectl create secret generic db --from-literal=pass=s3cr3t`
  - Inspect: `kubectl get secret db -o jsonpath='{.data.pass}' | base64 -d`
- Encryption at rest
  - EncryptionConfiguration (apiserver-config.k8s.io/v1)
    - `resources[].providers`: aescbc, aesgcm, secretbox, kms (v2)
    - First provider = write key
  - apiserver flag: `--encryption-provider-config=/etc/kubernetes/enc/enc.yaml`
  - Rotate + re-encrypt: `kubectl get secrets -A -o json | kubectl replace -f -`
- Consumption
  - Env: `env[].valueFrom.secretKeyRef`
  - Volume (preferred): `volumes[].secret.secretName`
- Restrict access
  - RBAC: limit `verbs: ["get","list"]` on `resources: ["secrets"]`
- External managers
  - HashiCorp Vault (Agent Injector / CSI)
  - External Secrets Operator (`ExternalSecret` CRD)
  - Secrets Store CSI Driver (`SecretProviderClass`)

### Understand and implement isolation techniques (multi-tenancy, sandboxed containers, etc.)
- Namespace soft multi-tenancy
  - `ResourceQuota` (`spec.hard.requests.cpu`, `limits.memory`, `pods`)
  - `LimitRange` (default/defaultRequest per container)
- Per-tenant controls
  - RBAC Role/RoleBinding scoped to namespace
  - NetworkPolicy isolating namespace traffic
- Sandboxed runtimes
  - gVisor (runsc) - user-space kernel
  - Kata Containers - lightweight VM per pod
  - RuntimeClass (node.k8s.io/v1)
    - `handler: runsc`
    - Pod: `spec.runtimeClassName: gvisor`
  - containerd config: `/etc/containerd/config.toml` `[plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes.runsc]`
- Node isolation
  - Taints: `kubectl taint nodes n1 tenant=a:NoSchedule`
  - Pod `tolerations` + `nodeSelector` / `affinity.nodeAffinity`

### Implement Pod-to-Pod encryption (Cilium, Istio)
- Cilium
  - Transparent encryption: WireGuard (`encryption.type: wireguard`)
  - IPsec mode (`encryption.enabled=true`, `encryption.type=ipsec`)
  - Enable via Helm: `--set encryption.enabled=true`
  - eBPF dataplane, identity-based policy
- Istio service mesh
  - Sidecar Envoy proxy injection (`istio-injection=enabled` ns label)
  - `PeerAuthentication.spec.mtls.mode: STRICT`
  - `DestinationRule.spec.trafficPolicy.tls.mode: ISTIO_MUTUAL`
  - Verify: `istioctl authn tls-check <pod>`
- Linkerd (alternative)
  - Automatic mTLS by default (`linkerd.io/inject: enabled`)

## Supply Chain Security (20%)

### Minimize base image footprint
- Minimal base images
  - `gcr.io/distroless/static`, `gcr.io/distroless/base`
  - `alpine:3.20`, `scratch`
- Multi-stage builds
  - `FROM golang AS build` then `COPY --from=build /app /app`
- Remove shells / package managers
  - No `bash`, `apk`, `apt` in final layer
- Pin versions
  - No `:latest`; pin digest `image@sha256:<digest>`
- Non-root in Dockerfile
  - `USER 1000` / `USER nonroot`
- Verify layers: `docker history <img>`, `dive <img>`

### Understand your supply chain (e.g. SBOM, CI/CD, artifact repositories)
- SBOM (Software Bill of Materials)
  - Formats: SPDX, CycloneDX
  - Generate: `syft <img> -o spdx-json`
  - `trivy image --format cyclonedx --output sbom.json <img>`
  - Attest: `cosign attest --predicate sbom.json --type cyclonedx <img>`
- CI/CD pipeline security
  - Least-privilege pipeline tokens / OIDC federation
  - Scan gates that fail on HIGH/CRITICAL
  - Signed commits / protected branches
- Artifact repositories
  - Harbor (built-in Trivy scanning, RBAC, replication)
  - AWS ECR (`scanOnPush`), Google Artifact Registry
  - Immutable tags, vulnerability quarantine

### Secure your supply chain (permitted registries, sign and validate artifacts, etc.)
- Restrict to permitted registries
  - ImagePolicyWebhook admission controller (`--admission-control-config-file`)
  - Kyverno policy: validate `image` startsWith `registry.corp.io/`
  - OPA Gatekeeper constraint on allowed registries
  - Disallow `:latest`; require digest references
- Image signing & verification
  - Cosign / Sigstore
    - Sign: `cosign sign <registry>/app:1.0`
    - Verify: `cosign verify --key cosign.pub <img>`
    - Keyless: `cosign sign --identity-token` (Fulcio + Rekor)
  - Kyverno `verifyImages` rule with public key / keyless
  - Connaisseur admission webhook
- Registry auth
  - `imagePullSecrets[].name`
  - `kubectl create secret docker-registry regcred --docker-server=...`

### Perform static analysis of user workloads and container images (e.g. Kubesec, KubeLinter)
- Image vulnerability scanning
  - Trivy
    - `trivy image nginx:1.27`
    - `trivy image --severity HIGH,CRITICAL nginx`
    - `trivy image --exit-code 1 --severity CRITICAL nginx`
    - `trivy k8s --report summary cluster`
  - Grype: `grype <img>`
  - Clair (Harbor backend)
- Manifest / workload static analysis
  - Kubesec: `kubesec scan deploy.yaml` (risk score JSON)
  - KubeLinter: `kube-linter lint deploy.yaml`
  - kube-score: `kube-score score deploy.yaml`
  - Checkov: `checkov -f deploy.yaml`
  - Conftest (OPA Rego): `conftest test deploy.yaml`
- CI integration
  - Fail build on HIGH/CRITICAL via exit codes
  - Cache vuln DB; schedule rescans

## Monitoring, Logging and Runtime Security (20%)

### Perform behavioral analytics to detect malicious activities
- Falco (CNCF runtime security)
  - Drivers: modern eBPF (`--modern-bpf`), eBPF probe, kernel module
  - Run: `falco -r /etc/falco/falco_rules.yaml`
  - Config: /etc/falco/falco.yaml
  - Rule files
    - /etc/falco/falco_rules.yaml (defaults)
    - /etc/falco/falco_rules.local.yaml (overrides)
  - Rule fields
    - `rule:`, `condition:`, `output:`, `priority:`, `tags:`
    - Reusable `macro:` and `list:`
  - Example conditions
    - Shell in container: `spawned_process and container and shell_procs`
    - Write below binary dir: `open_write and bin_dir`
    - Unexpected outbound: `outbound and not allowed`
  - Outputs: stdout, file, gRPC, falcosidekick, program
  - Validate rules: `falco -V /etc/falco/falco_rules.yaml`
- Tetragon (Cilium eBPF) - `TracingPolicy` CRD alternative

### Detect threats within physical infrastructure, apps, networks, data, users and workloads
- Event sources
  - Falco syscall driver (host + container)
  - Falco k8s audit plugin (`k8saudit`)
- Network anomaly detection
  - Cilium Hubble (`hubble observe --verdict DROPPED`)
  - NetworkPolicy drop logging
- Layered detection
  - Host: auditd rules (/etc/audit/rules.d/)
  - Container: Falco
  - Network: IDS (Suricata/Zeek)
  - API: kube-apiserver audit log
- File integrity monitoring
  - AIDE (`aide --check`), Falco `open_write` on sensitive paths

### Investigate and identify phases of attack and bad actors within the environment
- Map to MITRE ATT&CK / kill-chain phases
  - Recon, Initial Access, Execution, Persistence, Privilege Escalation, Exfiltration
- Triage with Falco priorities
  - Emergency, Alert, Critical, Error, Warning, Notice, Informational, Debug
- Correlation
  - Join apiserver audit.log + Falco alerts by `user`/`pod`/timestamp
- Response actions
  - Isolate pod: apply deny-all NetworkPolicy + cordon node
  - Revoke SA token: delete SA / rotate; `kubectl delete secret <token>`
  - Capture evidence: `kubectl logs`, `kubectl cp`, node `crictl ps`

### Ensure immutability of containers at runtime
- Enforce read-only filesystem
  - `securityContext.readOnlyRootFilesystem: true`
  - Writable paths via `volumes[].emptyDir` + `volumeMounts`
- Block escalation / privilege
  - `allowPrivilegeEscalation: false`
  - `privileged: false`
  - `capabilities.drop: ["ALL"]`
  - `runAsNonRoot: true`
- Enforcement
  - Pod Security Admission `restricted`
  - Kyverno / OPA Gatekeeper mutate+validate
- Drift detection (Falco)
  - Rule "Write below binary dir" (`bin_dir` + `open_write`)
  - Rule "Package management launched" (apt/apk/yum in container)

### Use Kubernetes audit logs to monitor access
- Enable on kube-apiserver
  - `--audit-policy-file=/etc/kubernetes/audit/policy.yaml`
  - `--audit-log-path=/var/log/kubernetes/audit.log`
  - `--audit-log-maxage=30`
  - `--audit-log-maxbackup=10`
  - `--audit-log-maxsize=100`
  - Mount policy + log dir as hostPath volumes in static pod
- Audit Policy (audit.k8s.io/v1)
  - Levels: None, Metadata, Request, RequestResponse
  - `rules[].resources[].group` + `resources`
  - `rules[].verbs: ["delete", "create"]`
  - `rules[].users` / `userGroups` / `namespaces`
  - `omitStages: ["RequestReceived"]`
- Stages
  - RequestReceived, ResponseStarted, ResponseComplete, Panic
- Backends
  - Log backend (file) + Webhook backend (`--audit-webhook-config-file`)
  - Ship to SIEM (Elasticsearch / Loki / Splunk)
- Inspect: `cat /var/log/kubernetes/audit.log | jq 'select(.verb=="delete")'`
