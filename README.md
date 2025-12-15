# cks


## Repositories

https://github.com/zealvora/certified-kubernetes-security-specialist
https://github.com/ahmetb/kubernetes-network-policy-recipes
https://github.com/ViktorUJ/cks
https://github.com/walidshaari/Certified-Kubernetes-Security-Specialist
https://github.com/techiescamp/cks-certification-guide
https://github.com/kodekloudhub/certified-kubernetes-security-specialist-cks-course


## Youtube courses

https://www.youtube.com/playlist?list=PLFkEchqXDZx6Bw3B2NRVc499j1TavjOvm
https://www.youtube.com/playlist?list=PLyJzBek6WsDpeFd8BUHx4OCtuv_A5zQW-
https://www.youtube.com/playlist?list=PLpbwBK0ptssx38770vYNwZEuCeGNw54CH
https://www.youtube.com/playlist?list=PLdyzeqlOPPmDSamNwyzWqY1ELgNVhCQXu
https://www.youtube.com/watch?v=d9xfB5qaOfg&t=39920s

---

## Cluster Hardening (15%)

- Use Role Based Access Controls to minimize exposure
- Exercise caution in using service accounts (e.g. disable defaults, minimize permissions on newly created ones)
- Restrict access to Kubernetes API
- Upgrade Kubernetes to avoid vulnerabilities

## System Hardening (10%)

- Minimize host OS footprint (reduce attack surface)
- Use least-privilege identity and access management
- Minimize external access to the network
- Appropriately use kernel hardening tools such as AppArmor and seccomp

## Minimize Microservice Vulnerabilities (20%)

- Use appropriate pod security standards
- Manage Kubernetes secrets
- Understand and implement isolation techniques (multi-tenancy, sandboxed containers, etc.)
- Implement Pod-to-Pod encryption (e.g. Cilium, Istio)

## Supply Chain Security (20%)

- Minimize base image footprint
- Understand your supply chain (e.g. SBOM, CI/CD, artifact repositories)
- Secure your supply chain (permitted registries, sign and validate artifacts, etc.)
- Perform static analysis of user workloads and container images (e.g. Kubesec, KubeLinter)

## Monitoring, Logging, and Runtime Security (20%)

- Perform behavioral analytics to detect malicious activities
- Detect threats within physical infrastructure, applications, networks, data, users, and workloads
- Investigate and identify phases of attack and bad actors within the environment
- Ensure immutability of containers at runtime
- Use Kubernetes audit logs to monitor access
