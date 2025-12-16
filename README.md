# CKS

## CKS Curriculum

### Cluster Setup (15%)

- [ ] **01** Use Network security policies to restrict cluster level access  
- [ ] **02** Use CIS benchmark to review the security configuration of Kubernetes components (etcd, kubelet, kubedns, kubeapi)  
- [ ] **03** Properly set up Ingress objects with TLS  
- [ ] **04** Protect node metadata and endpoints  
- [ ] **05** Verify platform binaries before deploying  

### Cluster Hardening (15%)

- [ ] **06** Use Role Based Access Controls to minimize exposure  
- [ ] **07** Exercise caution in using service accounts (e.g. disable defaults, minimize permissions on newly created ones)  
- [ ] **08** Restrict access to Kubernetes API  
- [ ] **09** Upgrade Kubernetes to avoid vulnerabilities  

### System Hardening (10%)

- [ ] **10** Minimize host OS footprint (reduce attack surface)  
- [ ] **11** Use least-privilege identity and access management  

### Minimize Microservice Vulnerabilities (20%)

- [ ] **12** Use appropriate pod security standards  
- [ ] **13** Manage Kubernetes secrets  
- [ ] **14** Understand and implement isolation techniques (multi-tenancy, sandboxed containers, etc.)  
- [ ] **15** Implement Pod-to-Pod encryption (e.g. Cilium, Istio)  

### Supply Chain Security (20%)

- [ ] **16** Minimize base image footprint  
- [ ] **17** Understand your supply chain (e.g. SBOM, CI/CD, artifact repositories)  
- [ ] **18** Secure your supply chain (permitted registries, sign and validate artifacts, etc.)  
- [ ] **19** Perform static analysis of user workloads and container images (e.g. Kubesec, KubeLinter)  

### Monitoring, Logging, and Runtime Security (20%)

- [ ] **20** Perform behavioral analytics to detect malicious activities  
- [ ] **21** Minimize external access to the network  
- [ ] **22** Detect threats within physical infrastructure, applications, networks, data, users, and workloads  
- [ ] **23** Appropriately use kernel hardening tools such as AppArmor and seccomp  
- [ ] **24** Investigate and identify phases of attack and bad actors within the environment  
- [ ] **25** Ensure immutability of containers at runtime  
- [ ] **26** Use Kubernetes audit logs to monitor access  

## Courses

- [KodeKloud](https://learn.kodekloud.com/courses/certified-kubernetes-security-specialist-cks)
- [Killercoda](https://killercoda.com/)
- [Killer Shell](https://killer.sh/)
- [Linux Foundation](https://training.linuxfoundation.org/certification/certified-kubernetes-security-specialist/)
- https://www.youtube.com/playlist?list=PLFkEchqXDZx6Bw3B2NRVc499j1TavjOvm
- https://www.youtube.com/playlist?list=PLyJzBek6WsDpeFd8BUHx4OCtuv_A5zQW-
- https://www.youtube.com/playlist?list=PLpbwBK0ptssx38770vYNwZEuCeGNw54CH
- https://www.youtube.com/playlist?list=PLdyzeqlOPPmDSamNwyzWqY1ELgNVhCQXu
- https://www.youtube.com/watch?v=d9xfB5qaOfg&t=39920s

## Repositories

- https://github.com/zealvora/certified-kubernetes-security-specialist
- https://github.com/ahmetb/kubernetes-network-policy-recipes
- https://github.com/ViktorUJ/cks
- https://github.com/walidshaari/Certified-Kubernetes-Security-Specialist
- https://github.com/techiescamp/cks-certification-guide
- https://github.com/kodekloudhub/certified-kubernetes-security-specialist-cks-course

## Articles

https://dev.to/mageshwaransekar/installing-cilium-as-cni-for-kubernetes-cluster-5d28
https://techwithmohamed.com/blog/cks-exam-study-guide/
https://aws.plainenglish.io/istio-must-know-for-the-certified-kubernetes-security-specialist-cks-exam-c5f1f86f24d8
https://afonsorodrigues.com/cks/
https://dev.to/ptuladhar3/falco-must-know-for-cks-exam-7en
https://www.groundcover.com/blog/kubernetes-network-policy
https://medium.com/@muppedaanvesh/a-hands-on-guide-to-kubernetes-network-policy-%EF%B8%8F-041bebe19a23
https://kubernetes.io/docs/concepts/services-networking/network-policies/

