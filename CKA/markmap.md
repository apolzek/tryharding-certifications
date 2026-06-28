---
title: CKA - Certified Kubernetes Administrator
markmap:
  colorFreezeLevel: 2
---

# CKA

## Cluster Architecture, Installation and Configuration (25%)

### Control Plane Components
- kube-apiserver
  - REST API frontend
    - Static pod manifest `/etc/kubernetes/manifests/kube-apiserver.yaml`
    - Serves on `--secure-port=6443`
    - `--bind-address=0.0.0.0`
  - Authentication / Authorization / Admission
    - `--authorization-mode=Node,RBAC`
    - `--enable-admission-plugins=NodeRestriction`
    - `--client-ca-file=/etc/kubernetes/pki/ca.crt`
  - etcd connection
    - `--etcd-servers=https://127.0.0.1:2379`
    - `--etcd-cafile=/etc/kubernetes/pki/etcd/ca.crt`
    - `--etcd-certfile`, `--etcd-keyfile`
  - Service network
    - `--service-cluster-ip-range=10.96.0.0/12`
- etcd
  - Key-value store (cluster state)
    - Listens on `--listen-client-urls=https://127.0.0.1:2379`
    - Data at `--data-dir=/var/lib/etcd`
  - Backup
    - `etcdctl --endpoints=https://127.0.0.1:2379 snapshot save /opt/snap.db`
    - `--cacert=/etc/kubernetes/pki/etcd/ca.crt`
    - `--cert=/etc/kubernetes/pki/etcd/server.crt --key=...server.key`
    - `ETCDCTL_API=3`
  - Restore
    - `etcdctl snapshot restore /opt/snap.db --data-dir=/var/lib/etcd-restore`
    - Update `etcd.yaml` hostPath to new data-dir
  - Verify
    - `etcdctl member list`
    - `etcdctl endpoint health`
  - Certs `/etc/kubernetes/pki/etcd`
- kube-scheduler
  - Pod-to-Node binding
    - Static pod `/etc/kubernetes/manifests/kube-scheduler.yaml`
    - Sets `pod.spec.nodeName` via Binding API
  - Filtering (Predicates)
    - PodFitsResources, NodeAffinity, TaintToleration
  - Scoring (Priorities)
    - LeastAllocated, NodeAffinity weight
- kube-controller-manager
  - Reconcile loops
    - `/etc/kubernetes/manifests/kube-controller-manager.yaml`
  - Controllers
    - Node controller (`--node-monitor-grace-period`)
    - ReplicaSet controller
    - Endpoint / EndpointSlice controller
  - Cert signing
    - `--cluster-signing-cert-file=/etc/kubernetes/pki/ca.crt`
- cloud-controller-manager
  - `--cloud-provider=external`
  - Node, Route, Service LB controllers

### Node Components
- kubelet
  - Node agent
    - Config `/var/lib/kubelet/config.yaml`
    - Service `/etc/systemd/system/kubelet.service.d/10-kubeadm.conf`
    - Kubeconfig `/etc/kubernetes/kubelet.conf`
  - Static pods
    - Watches `staticPodPath: /etc/kubernetes/manifests`
  - PLEG (Pod Lifecycle Event Generator)
  - `--container-runtime-endpoint=unix:///run/containerd/containerd.sock`
- kube-proxy
  - Service networking
    - DaemonSet in `kube-system`
    - Mode `--proxy-mode=iptables` or `ipvs`
    - ConfigMap `kube-proxy` in `kube-system`
  - Programs `iptables -t nat -L KUBE-SERVICES`
- Container runtime (CRI)
  - containerd
    - Config `/etc/containerd/config.toml`
    - `SystemdCgroup = true`
    - Socket `/run/containerd/containerd.sock`
  - CRI-O socket `/var/run/crio/crio.sock`
  - Inspect via `crictl --runtime-endpoint ... ps`

### kubeadm Cluster Lifecycle
- `kubeadm init`
  - `--pod-network-cidr=10.244.0.0/16`
  - `--control-plane-endpoint=lb.example.com:6443`
  - `--apiserver-advertise-address=<ip>`
  - `--upload-certs`
  - Config file `kubeadm init --config=kubeadm-config.yaml`
- `kubeadm join`
  - `kubeadm join <ip>:6443 --token <token> --discovery-token-ca-cert-hash sha256:<hash>`
  - Control plane: `--control-plane --certificate-key <key>`
- Add / remove nodes
  - `kubeadm token create --print-join-command`
  - Compute hash `openssl x509 -pubkey -in /etc/kubernetes/pki/ca.crt | openssl ...`
  - `kubectl drain <node> --ignore-daemonsets --delete-emptydir-data`
  - `kubectl delete node <node>`
  - On node: `kubeadm reset`
- Cluster upgrade
  - `kubeadm upgrade plan`
  - `kubeadm upgrade apply v1.33.1` (first control plane)
  - `kubeadm upgrade node` (other nodes)
  - `apt-mark unhold kubeadm && apt install kubeadm=1.33.1-*`
  - kubelet + kubectl: `apt install kubelet=1.33.1-* kubectl=1.33.1-*`
  - `systemctl daemon-reload && systemctl restart kubelet`
  - Order: control plane → workers, one at a time
- Certificate renewal
  - `kubeadm certs check-expiration`
  - `kubeadm certs renew all`
  - Certs in `/etc/kubernetes/pki/*.crt`

### High Availability Control Plane
- Stacked etcd vs external etcd
  - Stacked: etcd as static pod per control plane node
  - External: `--external etcd endpoints` in kubeadm config
- Multiple control plane nodes
  - Join with `--control-plane --certificate-key`
- Load balancer in front of API servers
  - L4 LB to all `:6443`, set `controlPlaneEndpoint`

### RBAC (Role Based Access Control)
- Role / ClusterRole
  - `rules[].apiGroups`, `rules[].resources`, `rules[].verbs`
  - `kubectl create role pod-reader --verb=get,list,watch --resource=pods`
  - `kubectl create clusterrole node-reader --verb=get --resource=nodes`
- RoleBinding / ClusterRoleBinding
  - `kubectl create rolebinding rb --role=pod-reader --user=jane`
  - `kubectl create clusterrolebinding crb --clusterrole=cluster-admin --serviceaccount=ns:sa`
  - `roleRef.kind`, `subjects[].kind`
- ServiceAccounts
  - `kubectl create serviceaccount ci`
  - Token `kubectl create token ci`
- Verify
  - `kubectl auth can-i create deployments --as=jane`
  - `kubectl auth can-i list pods --as=system:serviceaccount:ns:ci`
- Aggregated ClusterRoles
  - `aggregationRule.clusterRoleSelectors.matchLabels`

### Underlying Infrastructure
- OS prerequisites
  - `swapoff -a` and remove from `/etc/fstab`
  - `modprobe br_netfilter overlay`
  - `sysctl net.ipv4.ip_forward=1`
  - `net.bridge.bridge-nf-call-iptables=1` in `/etc/sysctl.d/k8s.conf`
- Install container runtime
  - `apt install containerd.io`
  - `containerd config default > /etc/containerd/config.toml`
- Install kubeadm / kubelet / kubectl
  - Add repo `pkgs.k8s.io/core:/stable:/v1.33/deb/`
  - `apt install -y kubelet kubeadm kubectl`
  - `apt-mark hold kubelet kubeadm kubectl`

### Package & Extension Tooling
- Helm
  - Charts (templates + values.yaml)
  - Releases tracked in `secret type=helm.sh/release.v1`
  - `helm repo add bitnami https://charts.bitnami.com/bitnami`
  - `helm install web bitnami/nginx -f values.yaml --set replicaCount=3`
  - `helm upgrade web bitnami/nginx --version 1.2.3`
  - `helm rollback web 1`
  - `helm list -A`
- Kustomize
  - `kustomization.yaml` (resources, patches)
  - bases / overlays directories
  - `patchesStrategicMerge`, `images[].newTag`
  - `kubectl apply -k overlays/prod`
  - Preview `kubectl kustomize overlays/prod`

### Extension Interfaces
- CNI (Container Network Interface)
  - Config `/etc/cni/net.d/`
  - Binaries `/opt/cni/bin/`
  - Calico / Cilium / Flannel DaemonSets
- CSI (Container Storage Interface)
  - CSIDriver, CSINode objects
  - External provisioner / attacher sidecars
- CRI (Container Runtime Interface)
  - gRPC over `containerd.sock`

### CRDs & Operators
- CustomResourceDefinition
  - `spec.group`, `spec.versions[].schema.openAPIV3Schema`
  - `spec.scope: Namespaced`, `spec.names.kind`
  - `kubectl get crd`
- Custom Resources
  - `kubectl get <crd-plural> -A`
- Operator pattern
  - Controller watches CR + reconciles to desired state

## Troubleshooting (30%)

### Cluster & Node Troubleshooting
- Node state
  - `kubectl get nodes -o wide`
  - `kubectl describe node <n>` (Conditions, Events)
- Node conditions
  - `Ready`, `MemoryPressure`, `DiskPressure`, `PIDPressure`
- Scheduling control
  - `kubectl cordon <n>` / `kubectl uncordon <n>`
  - `kubectl drain <n> --ignore-daemonsets --force`
- kubelet diagnosis
  - `systemctl status kubelet`
  - `journalctl -u kubelet -f`
  - Check `/var/lib/kubelet/config.yaml`

### Control Plane Component Troubleshooting
- Static pod manifests
  - `/etc/kubernetes/manifests/{kube-apiserver,etcd,...}.yaml`
  - Move file out → kubelet stops pod
- Container inspection
  - `crictl ps -a`
  - `crictl logs <container-id>`
  - `crictl --runtime-endpoint unix:///run/containerd/containerd.sock ps`
- Component logs
  - `/var/log/pods/<ns>_<pod>_<uid>/`
  - `/var/log/containers/*.log`
- etcd health
  - `etcdctl endpoint health --endpoints=https://127.0.0.1:2379 --cacert ... --cert ... --key ...`

### Application / Pod Troubleshooting
- Inspect
  - `kubectl describe pod <p>` (Events section)
  - `kubectl get pod <p> -o yaml`
- Pod states
  - `Pending` → check `kubectl describe` for scheduling/resources
  - `CrashLoopBackOff` → `kubectl logs --previous`
  - `ImagePullBackOff` → image name / pull secret
  - `OOMKilled` → raise `resources.limits.memory`
- Logs
  - `kubectl logs <p> --previous`
  - `kubectl logs <p> -c <container>`
- Exec / debug
  - `kubectl exec -it <p> -- sh`
  - `kubectl debug <p> -it --image=busybox --target=<container>`
  - `kubectl debug node/<n> -it --image=ubuntu`

### Resource Usage Monitoring
- Metrics Server
  - Deployed in `kube-system`, exposes `metrics.k8s.io`
  - `--kubelet-insecure-tls` (lab)
- `kubectl top nodes`
- `kubectl top pods --containers -A`

### Container Output Streams
- stdout / stderr captured by runtime
- Logging architecture
  - Symlinks `/var/log/containers/*.log → /var/log/pods/...`
- `journalctl` for systemd-managed kubelet

### Services & Networking Troubleshooting
- Endpoints
  - `kubectl get endpoints <svc>`
  - `kubectl get endpointslices -l kubernetes.io/service-name=<svc>`
  - Empty endpoints → selector/readiness mismatch
- DNS resolution
  - `kubectl exec -it <p> -- nslookup <svc>.<ns>.svc.cluster.local`
  - Check CoreDNS pods `kubectl -n kube-system get pods -l k8s-app=kube-dns`
- Service check
  - `kubectl get svc -o wide`
  - Compare `svc.spec.selector` to pod labels
- kube-proxy / iptables
  - `iptables -t nat -L KUBE-SERVICES -n`
  - `kubectl -n kube-system logs ds/kube-proxy`

## Services and Networking (20%)

### Pod Connectivity
- Pod network model (flat, no NAT)
  - Each pod gets a routable IP
- Pod-to-Pod across nodes
  - Handled by CNI overlay/routing
  - Verify `kubectl get pods -o wide` (pod IPs)

### Service Types
- ClusterIP
  - `spec.type: ClusterIP`, virtual IP from service CIDR
  - `spec.ports[].port` / `spec.ports[].targetPort`
- NodePort
  - `spec.type: NodePort`, `spec.ports[].nodePort` 30000-32767
- LoadBalancer
  - `spec.type: LoadBalancer`, gets `status.loadBalancer.ingress[].ip`
- ExternalName
  - `spec.externalName: db.example.com` (CNAME)
- Endpoints & EndpointSlices
  - `kubectl get endpointslices`

### CoreDNS
- Cluster DNS service
  - Deployment `coredns` in `kube-system`
  - Service `kube-dns` ClusterIP
- Records
  - `<svc>.<ns>.svc.cluster.local`
  - `<pod-ip-dashed>.<ns>.pod.cluster.local`
- Pod DNS policy
  - `spec.dnsPolicy: ClusterFirst`
  - `spec.dnsConfig.nameservers`
- Configure
  - ConfigMap `coredns` Corefile (`forward`, `cache`, `hosts`)

### Ingress
- Ingress Controllers
  - nginx ingress controller Deployment + Service
- Ingress Resource
  - `spec.rules[].host`
  - `spec.rules[].http.paths[].path`
  - `spec.rules[].http.paths[].pathType: Prefix`
  - `spec.rules[].http.paths[].backend.service.name/port.number`
  - `kubectl create ingress web --rule="host/path=svc:80"`
- TLS termination
  - `spec.tls[].hosts` + `spec.tls[].secretName`
- IngressClass
  - `spec.ingressClassName: nginx`

### Gateway API
- GatewayClass
  - `spec.controllerName`
- Gateway
  - `spec.listeners[].protocol/port`
- HTTPRoute
  - `spec.rules[].matches[].path`
  - `spec.rules[].backendRefs[]`
- Successor to Ingress (`gateway.networking.k8s.io`)

### Network Policies
- Default deny
  - empty `spec.podSelector: {}` + `policyTypes: [Ingress]`, no rules
- Selectors
  - `spec.ingress[].from[].podSelector.matchLabels`
  - `spec.ingress[].from[].namespaceSelector`
  - `spec.egress[].to[].ipBlock.cidr`
- Ports
  - `spec.ingress[].ports[].protocol/port`
- Requires CNI support (Calico, Cilium)

## Workloads and Scheduling (15%)

### Workload Resources
- Pod
  - `spec.containers[].image`, `spec.containers[].command`
- ReplicaSet
  - `spec.replicas`, `spec.selector.matchLabels`
- Deployment
  - `spec.strategy.type: RollingUpdate`
  - `spec.strategy.rollingUpdate.maxSurge`
  - `spec.strategy.rollingUpdate.maxUnavailable`
  - `kubectl set image deploy/x c=img:2`
  - `kubectl rollout status deploy/x`
  - `kubectl rollout history deploy/x`
  - `kubectl rollout undo deploy/x --to-revision=2`
- StatefulSet
  - `spec.serviceName`, `spec.volumeClaimTemplates[]`
  - Stable name `<sts>-0`, `<sts>-1`
- DaemonSet
  - One pod/node, tolerates control-plane taints
- Job / CronJob
  - `spec.completions`, `spec.parallelism`, `spec.backoffLimit`
  - CronJob `spec.schedule: "*/5 * * * *"`

### Self-Healing Primitives
- ReplicaSet desired state (`spec.replicas`)
- Probes
  - `livenessProbe.httpGet.path`
  - `readinessProbe.tcpSocket.port`
  - `startupProbe.failureThreshold`
- restartPolicy
  - `spec.restartPolicy: Always|OnFailure|Never`
- PodDisruptionBudget
  - `spec.minAvailable` / `spec.maxUnavailable`
  - `spec.selector.matchLabels`

### Configuration
- ConfigMaps
  - `kubectl create cm app --from-file=./conf`
  - `kubectl create cm app --from-literal=KEY=val`
  - env via `envFrom[].configMapRef.name`
  - volume via `spec.volumes[].configMap.name`
- Secrets
  - Types `Opaque`, `kubernetes.io/dockerconfigjson`, `kubernetes.io/tls`
  - `kubectl create secret generic db --from-literal=pw=x`
  - base64 in `data:` (not encrypted at rest by default)
- Mounting
  - env: `valueFrom.secretKeyRef`
  - volume: `spec.volumes[].secret.secretName`

### Autoscaling
- HorizontalPodAutoscaler (HPA)
  - `kubectl autoscale deploy/x --min=2 --max=10 --cpu-percent=80`
  - `spec.metrics[].resource.target.averageUtilization`
  - requires Metrics Server
- VerticalPodAutoscaler
  - `spec.updatePolicy.updateMode` (concept)

### Pod Admission & Scheduling
- Resource requests & limits
  - `spec.containers[].resources.requests.cpu`
  - `spec.containers[].resources.limits.memory`
- nodeSelector
  - `spec.nodeSelector.disktype: ssd`
  - `kubectl label node n1 disktype=ssd`
- Node affinity
  - `spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution`
- Pod affinity / anti-affinity
  - `spec.affinity.podAntiAffinity.topologyKey: kubernetes.io/hostname`
- Taints & Tolerations
  - `kubectl taint node n1 key=val:NoSchedule`
  - `spec.tolerations[].key/operator/effect`
- Topology spread constraints
  - `spec.topologySpreadConstraints[].maxSkew`
- Static pods (`/etc/kubernetes/manifests`)
- Manual scheduling (`spec.nodeName: n1`)

## Storage (10%)

### Volumes
- emptyDir
  - `spec.volumes[].emptyDir.medium: Memory`
- hostPath
  - `spec.volumes[].hostPath.path` / `.type: DirectoryOrCreate`
- configMap / secret volumes
  - `spec.volumes[].configMap.items[].key/path`

### Persistent Volumes (PV)
- Capacity / access modes
  - `spec.capacity.storage: 5Gi`
  - `spec.accessModes`
- Reclaim policy
  - `spec.persistentVolumeReclaimPolicy: Retain|Delete|Recycle`
- Source
  - `spec.nfs.server/path` or `spec.csi.driver`

### Persistent Volume Claims (PVC)
- Binding to PV
  - `spec.volumeName` or matched by class/size
- Request storage
  - `spec.resources.requests.storage: 1Gi`
- Mount in pod
  - `spec.volumes[].persistentVolumeClaim.claimName`
  - `kubectl get pvc`

### Access Modes
- ReadWriteOnce (RWO)
- ReadOnlyMany (ROX)
- ReadWriteMany (RWX)
- ReadWriteOncePod (RWOP)

### Storage Classes
- Dynamic provisioning
  - `provisioner: ebs.csi.aws.com`
- Policy
  - `reclaimPolicy: Delete`
  - `volumeBindingMode: WaitForFirstConsumer`
- Default StorageClass
  - annotation `storageclass.kubernetes.io/is-default-class: "true"`

### Volume Types
- Cloud volumes (CSI `ebs.csi.aws.com`)
- NFS (`spec.nfs.server`)
- CSI-backed volumes (`spec.csi.driver`)

## Exam Essentials

### kubectl Productivity
- `alias k=kubectl`
- `kubectl run x --image=nginx --dry-run=client -o yaml`
- `kubectl explain pod.spec.containers --recursive`
- imperative `kubectl create deploy/cm/secret`
- `kubectl config use-context <ctx>`
- `kubectl config set-context --current --namespace=<ns>`

### Documentation
- kubernetes.io/docs allowed during exam
- Bookmark NetworkPolicy / PV / RBAC example pages

### Environment
- Remote desktop, PSI browser
- Multiple clusters via `kubectl config get-contexts`
