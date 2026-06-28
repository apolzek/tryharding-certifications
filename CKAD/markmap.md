---
title: CKAD - Certified Kubernetes Application Developer
markmap:
  colorFreezeLevel: 2
---

# CKAD

## Application Design and Build (20%)
### Container Images
- Define, build and modify container images
  - Dockerfile / Containerfile authoring
    - `FROM`, `RUN`, `COPY`, `WORKDIR`
    - `ENTRYPOINT` vs `CMD`
    - `EXPOSE 8080`, `USER 1000`
  - Build
    - `docker build -t app:1.0 .`
    - `podman build -t app:1.0 .`
  - Distribute
    - `docker tag app:1.0 reg/app:1.0`
    - `docker push reg/app:1.0`
    - `docker pull reg/app:1.0`
  - Multi-stage builds
    - second `FROM` + `COPY --from=builder /bin/app .`
  - Versioning
    - semantic tags `app:1.2.3`, avoid `:latest`
- OCI runtimes and tools
  - Docker, Podman, Buildah, containerd, CRI-O
  - CRI inspection
    - `crictl ps`, `crictl images`, `crictl logs <id>`
    - `--runtime-endpoint unix:///run/containerd/containerd.sock`
### Choose the Right Workload Resource
- Pod
  - Smallest deployable unit
    - `spec.containers[].image`
  - `kubectl run nginx --image=nginx`
- Deployment
  - Stateless, manages ReplicaSet, rolling updates
  - `kubectl create deployment web --image=nginx --replicas=3`
  - `spec.template.spec.containers[]`
- ReplicaSet
  - `spec.replicas`, `spec.selector.matchLabels`
- DaemonSet
  - One pod per node
  - `spec.template.spec.tolerations[]` for control-plane
- StatefulSet
  - `spec.serviceName`, `spec.volumeClaimTemplates[]`
  - Ordinal names `<sts>-0`
- Job
  - `spec.completions`, `spec.parallelism`, `spec.backoffLimit`
  - `spec.activeDeadlineSeconds`
  - `kubectl create job pi --image=perl -- perl -Mbignum -e 'print 1'`
- CronJob
  - `spec.schedule: "*/5 * * * *"`
  - `spec.concurrencyPolicy: Forbid`
  - `spec.successfulJobsHistoryLimit`
  - `kubectl create cronjob hi --image=busybox --schedule="*/1 * * * *" -- echo hi`
### Multi-Container Pod Design Patterns
- Sidecar
  - Helper container in same pod
  - Native sidecar: `spec.initContainers[]` with `restartPolicy: Always`
- Init containers
  - `spec.initContainers[]` run sequentially before app
  - wait-for-dependency / schema migration `command`
- Ambassador
  - Proxy container reached via `localhost:<port>`
- Adapter
  - Transforms app output (e.g. metrics) for consumers
- Shared context
  - Shared network namespace (`localhost`)
  - Shared volume via `spec.volumes[]` + `volumeMounts`
### Persistent and Ephemeral Volumes
- Ephemeral volumes
  - `spec.volumes[].emptyDir: {}`
  - `spec.volumes[].emptyDir.medium: Memory` (tmpfs)
  - `configMap`, `secret`, `downwardAPI`, `projected`
- PersistentVolume (PV)
  - `spec.accessModes` (RWO, ROX, RWX, RWOP)
  - `spec.capacity.storage`
  - `spec.persistentVolumeReclaimPolicy`
- PersistentVolumeClaim (PVC)
  - `spec.resources.requests.storage`
  - `spec.storageClassName`
  - `kubectl get pvc`
- StorageClass
  - `provisioner`, `volumeBindingMode: WaitForFirstConsumer`
  - default class annotation `storageclass.kubernetes.io/is-default-class`
- Mounting
  - `spec.containers[].volumeMounts[].mountPath`
  - `spec.volumes[].persistentVolumeClaim.claimName`

## Application Deployment (20%)
### Deployment Strategies
- Rolling update (default)
  - `spec.strategy.rollingUpdate.maxSurge`
  - `spec.strategy.rollingUpdate.maxUnavailable`
- Recreate strategy
  - `spec.strategy.type: Recreate`
- Blue/Green
  - Two Deployments, switch `service.spec.selector.version`
- Canary
  - Subset of new-version pods sharing Service label
### Deployments and Rolling Updates
- Manage rollouts
  - `kubectl set image deploy/web nginx=nginx:1.26`
  - `kubectl rollout status deploy/web`
  - `kubectl rollout history deploy/web`
  - `kubectl rollout history deploy/web --revision=2`
  - `kubectl rollout undo deploy/web --to-revision=2`
  - `kubectl rollout pause deploy/web` / `kubectl rollout resume deploy/web`
  - `kubectl rollout restart deploy/web`
- Scaling
  - `kubectl scale deploy/web --replicas=5`
  - `kubectl autoscale deploy/web --min=2 --max=10 --cpu-percent=80`
- Revision tracking
  - `spec.revisionHistoryLimit`
  - annotation `kubernetes.io/change-cause`
  - `kubectl apply -f d.yaml --record` (or set annotation)
### Helm Package Manager
- Core concepts
  - Chart (templates + `values.yaml` + `Chart.yaml`)
  - Release tracked as `secret type=helm.sh/release.v1`
  - Repository index
- Commands
  - `helm repo add bitnami https://charts.bitnami.com/bitnami`
  - `helm repo update`
  - `helm search repo nginx`
  - `helm install web bitnami/nginx`
  - `helm upgrade web bitnami/nginx --version 1.2.3`
  - `helm rollback web 1`
  - `helm list -A` / `helm uninstall web`
  - `helm install web ./chart -f values.yaml --set replicaCount=3`
  - `helm template ./chart` (render locally)
### Kustomize
- Concepts
  - Overlays without templating
  - `kustomization.yaml`, bases, overlays, patches
- Features
  - `commonLabels`, `namePrefix`, `namespace`
  - `images[].newName/newTag`
  - `configMapGenerator[].literals`
  - `secretGenerator[].files`
  - `patchesStrategicMerge`
- Commands
  - `kubectl kustomize overlays/prod`
  - `kubectl apply -k overlays/prod`

## Application Observability and Maintenance (15%)
### API Deprecations
- Deprecation policy
  - maturity: alpha → beta → stable (GA)
  - `kubectl api-versions`
  - `kubectl api-resources`
  - `kubectl explain deploy` (shows current apiVersion)
  - Migrate manifests to current apiVersion (e.g. `apps/v1`)
  - `kubectl convert -f old.yaml` (plugin)
### Probes and Health Checks
- livenessProbe
  - restart on failure
  - `livenessProbe.httpGet.path: /healthz`
- readinessProbe
  - gate Service endpoints
  - `readinessProbe.tcpSocket.port`
- startupProbe
  - protect slow start
  - `startupProbe.failureThreshold`
- Probe handlers
  - `httpGet.path/port`, `tcpSocket.port`
  - `exec.command`, `grpc.port`
- Probe tuning
  - `initialDelaySeconds`, `periodSeconds`
  - `timeoutSeconds`, `failureThreshold`, `successThreshold`
### Monitoring with CLI Tools
- Resource metrics
  - `kubectl top pod` / `kubectl top node` (Metrics Server)
  - `kubectl top pod --containers`
- Inspection
  - `kubectl get pods -o wide`
  - `kubectl describe pod <name>`
  - `kubectl get events --sort-by=.metadata.creationTimestamp`
### Container Logs
- View logs
  - `kubectl logs <pod>`
  - `kubectl logs <pod> -c <container>`
  - `kubectl logs -f <pod>`
  - `kubectl logs --previous <pod>`
  - `kubectl logs --since=1h <pod>` / `--tail=50`
### Debugging in Kubernetes
- Interactive debugging
  - `kubectl exec -it <pod> -- sh`
  - `kubectl debug <pod> -it --image=busybox --target=<container>`
  - `kubectl port-forward pod/<pod> 8080:80`
- Troubleshooting flow
  - status: `Pending`, `CrashLoopBackOff`, `ImagePullBackOff`, `OOMKilled`
  - `kubectl describe` (Events) → `kubectl logs` (app errors)

## Application Environment, Configuration and Security (25%)
### Extending Kubernetes
- CustomResourceDefinition (CRD)
  - `spec.group`, `spec.names.kind`, `spec.scope`
  - `kubectl get crd`
- Operators
  - Controller + CRD reconcile loop
- Discovery
  - `kubectl api-resources`
  - `kubectl get <crd-plural> -A`
### Authentication, Authorization, Admission Control
- Authentication
  - client certs, bearer tokens
  - ServiceAccount token via `kubectl create token <sa>`
- Authorization (RBAC)
  - Role / ClusterRole (`rules[].apiGroups/resources/verbs`)
  - RoleBinding / ClusterRoleBinding (`roleRef`, `subjects[]`)
  - `kubectl create role pod-reader --verb=get,list --resource=pods`
  - `kubectl create rolebinding rb --role=pod-reader --serviceaccount=ns:sa`
  - `kubectl auth can-i get pods --as=system:serviceaccount:ns:sa`
- Admission control
  - MutatingWebhookConfiguration / ValidatingWebhookConfiguration
### Requests, Limits, Quotas
- Resource requests/limits
  - `resources.requests.cpu: 250m`
  - `resources.requests.memory: 128Mi`
  - `resources.limits.cpu: 500m`
  - `resources.limits.memory: 256Mi`
- ResourceQuota
  - `spec.hard["requests.cpu"]`, `spec.hard["limits.memory"]`
  - `spec.hard["pods"]`
- LimitRange
  - `spec.limits[].default` / `defaultRequest` / `max` / `min`
### ConfigMaps
- Create
  - `kubectl create configmap app --from-literal=KEY=val`
  - `kubectl create configmap app --from-file=config.properties`
  - `kubectl create configmap app --from-env-file=app.env`
- Consume
  - `envFrom[].configMapRef.name`
  - `valueFrom.configMapKeyRef.name/key`
  - volume `spec.volumes[].configMap.name`
### Secrets
- Create
  - `kubectl create secret generic db --from-literal=password=pass`
  - `kubectl create secret tls tls-cert --cert=t.crt --key=t.key`
  - `kubectl create secret docker-registry regcred --docker-server=...`
  - Types: `Opaque`, `kubernetes.io/dockerconfigjson`, `kubernetes.io/tls`
- Consume
  - env `valueFrom.secretKeyRef.name/key`
  - volume `spec.volumes[].secret.secretName`
  - base64 `data:` (not encrypted by default)
### ServiceAccounts
- Concepts
  - `spec.serviceAccountName`
  - `spec.automountServiceAccountToken: false`
  - `kubectl create serviceaccount ci`
- Tokens
  - projected `spec.volumes[].projected.sources[].serviceAccountToken`
  - `kubectl create token ci --duration=1h`
### Application Security
- SecurityContext
  - `securityContext.runAsUser: 1000`
  - `securityContext.runAsGroup`, `securityContext.fsGroup`
  - `securityContext.runAsNonRoot: true`
  - `securityContext.readOnlyRootFilesystem: true`
  - `securityContext.allowPrivilegeEscalation: false`
  - `securityContext.privileged: false`
- Linux capabilities
  - `securityContext.capabilities.add: ["NET_ADMIN"]`
  - `securityContext.capabilities.drop: ["ALL"]`
- seccomp / AppArmor
  - `securityContext.seccompProfile.type: RuntimeDefault`
  - annotation/`appArmorProfile.type: Localhost`

## Services and Networking (20%)
### NetworkPolicies
- Concepts
  - default allow-all without policy
  - requires CNI plugin (Calico, Cilium)
- Policy structure
  - `spec.podSelector.matchLabels`
  - `spec.policyTypes: [Ingress, Egress]`
  - `spec.ingress[].from[]` / `spec.egress[].to[]`
  - `spec.ingress[].ports[].port`
  - selectors: `podSelector`, `namespaceSelector`, `ipBlock.cidr`
- Common patterns
  - default deny ingress: `spec.podSelector: {}` + `policyTypes: [Ingress]`, no rules
### Services
- Service types
  - `spec.type: ClusterIP` (default)
  - `spec.type: NodePort` (`spec.ports[].nodePort` 30000-32767)
  - `spec.type: LoadBalancer`
  - `spec.type: ExternalName` (`spec.externalName`)
- Expose workloads
  - `kubectl expose deploy web --port=80 --target-port=8080`
  - `kubectl create service clusterip web --tcp=80:8080`
  - `spec.selector` maps to pod labels
- Endpoints / EndpointSlices
  - `kubectl get endpoints <svc>`
  - `kubectl get endpointslices`
- Troubleshooting access
  - verify `spec.selector`, `spec.ports[].targetPort`, readiness
  - `kubectl describe svc <svc>`
  - DNS `nslookup <svc>.<ns>.svc.cluster.local`
### Ingress
- Concepts
  - L7 HTTP/HTTPS routing to Services
  - requires Ingress controller (nginx, traefik)
- Ingress rules
  - `spec.rules[].host`
  - `spec.rules[].http.paths[].pathType: Prefix|Exact`
  - `spec.rules[].http.paths[].backend.service.name/port.number`
  - `kubectl create ingress web --rule="host/path=svc:80"`
  - TLS `spec.tls[].secretName`
- IngressClass
  - `spec.ingressClassName: nginx`

## Exam Essentials
### kubectl Productivity
- Aliases and autocompletion
  - `alias k=kubectl`
  - `source <(kubectl completion bash)`
  - `complete -F __start_kubectl k`
  - `export do='--dry-run=client -o yaml'`
- Set default namespace
  - `kubectl config set-context --current --namespace=<ns>`
- Useful flags
  - `--all-namespaces` / `-A`
  - `-o wide`, `-o yaml`, `-o json`
  - `-o jsonpath='{.items[*].metadata.name}'`
  - `--show-labels`, `-l key=value`
  - `--field-selector=status.phase=Running`
### Dry-Run and Manifest Generation
- Generate YAML fast
  - `kubectl run nginx --image=nginx --dry-run=client -o yaml > pod.yaml`
  - `kubectl create deploy web --image=nginx --dry-run=client -o yaml`
  - `kubectl create -f file.yaml --dry-run=server`
- Apply and edit
  - `kubectl apply -f f.yaml`
  - `kubectl replace -f f.yaml --force`
  - `kubectl edit deploy/web`
  - `kubectl delete -f f.yaml`
### kubectl explain
- Discover field schemas
  - `kubectl explain pod.spec`
  - `kubectl explain deploy.spec.template.spec.containers --recursive`
### Exam Logistics
- Performance-based, hands-on tasks (~2 hours)
- Allowed docs: kubernetes.io, helm.sh, kubectl docs
- Time management: flag hard questions, prefer imperative commands
- vim `:set paste`, `:set et sw=2` to avoid YAML indent issues
- Verify with `kubectl get` / `kubectl describe` after each task
