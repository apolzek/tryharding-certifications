---
title: Istio Certified Associate (ICA)
markmap:
  colorFreezeLevel: 2
---

# ICA

## Traffic Management (35%)
### Ingress Traffic
- Gateway (networking.istio.io/v1)
  - spec.selector
    - istio: ingressgateway (matches istio-ingressgateway pod labels)
    - custom gateway via `istioctl install --set` injection template
  - spec.servers[]
    - port.number / port.name / port.protocol (HTTP, HTTPS, GRPC, HTTP2, MONGO, TCP, TLS)
    - hosts: "namespace/host" (e.g. "*/*.example.com", "./reviews.svc")
    - tls.mode
      - SIMPLE (server TLS, credentialName secret)
      - MUTUAL (client cert required, caCertificates)
      - PASSTHROUGH (SNI routing, no decryption)
      - ISTIO_MUTUAL (mesh certs)
      - OPTIONAL_MUTUAL
    - tls.minProtocolVersion / maxProtocolVersion (TLSV1_2, TLSV1_3)
    - tls.credentialName -> Kubernetes secret in istio-system
- Kubernetes Gateway API (gateway.networking.k8s.io/v1)
  - GatewayClass.spec.controllerName: istio.io/gateway-controller
  - Gateway.spec.listeners[].protocol / .port / .tls.certificateRefs
  - HTTPRoute.spec.rules[].matches[].path.type (PathPrefix, Exact, RegularExpression)
  - HTTPRoute.spec.rules[].backendRefs[].weight
  - HTTPRoute.spec.parentRefs[] -> Gateway
  - ReferenceGrant for cross-namespace backendRefs
  - migration: `istioctl x analyze` + Gateway API conversion
### Egress Traffic
- ServiceEntry (networking.istio.io/v1)
  - spec.hosts[] + spec.ports[]
  - spec.resolution: DNS, STATIC, NONE, DNS_ROUND_ROBIN
  - spec.location: MESH_EXTERNAL, MESH_INTERNAL
  - spec.endpoints[].address (for STATIC)
  - spec.exportTo: ".", "*", "namespace"
- Egress Gateway
  - dedicated istio-egressgateway pod
  - Gateway + VirtualService + DestinationRule chaining to route egress
  - meshConfig.outboundTrafficPolicy.mode: REGISTRY_ONLY vs ALLOW_ANY
  - SNI-based routing on egress (tls.match.sniHosts)
### Routing
- VirtualService (networking.istio.io/v1)
  - spec.hosts[] (short name, FQDN, or "*")
  - spec.gateways[] (bind to Gateway, or "mesh" for sidecars)
  - spec.http[].match[]
    - uri.prefix / uri.exact / uri.regex
    - headers["x-user"].exact / .prefix / .regex
    - method.exact (GET/POST)
    - queryParams / port / scheme / authority
    - sourceLabels
  - spec.http[].route[].destination.host / .subset / .port.number
  - spec.http[].route[].weight (sums to 100)
  - spec.http[].rewrite.uri / .authority
  - spec.http[].redirect.uri / .authority / .redirectCode
  - spec.http[].fault.delay / .fault.abort
  - spec.http[].mirror + mirrorPercentage.value
  - spec.http[].corsPolicy.allowOrigins / allowMethods / allowHeaders
  - spec.http[].timeout / .retries
- DestinationRule (networking.istio.io/v1)
  - spec.host (FQDN of service)
  - spec.subsets[].name + .labels (e.g. version: v1)
  - spec.subsets[].trafficPolicy (per-subset override)
  - applied after VirtualService routing resolves destination
### Traffic Shifting & Release Strategies
- canary: two route[] entries with weight 90/10 -> shift to 0/100
- A/B testing: match.headers + route.subset
- blue-green: switch full weight between subset blue and green
- mirroring: spec.http[].mirror to shadow subset, mirrorPercentage
### Load Balancing
- DestinationRule.spec.trafficPolicy.loadBalancer
  - simple: ROUND_ROBIN, LEAST_REQUEST, RANDOM, PASSTHROUGH, LEAST_CONN
  - consistentHash.httpHeaderName
  - consistentHash.httpCookie (name, ttl)
  - consistentHash.useSourceIp: true
  - consistentHash.minimumRingSize
- locality load balancing
  - trafficPolicy.loadBalancer.localityLbSetting.distribute[].from / .to
  - localityLbSetting.failover[].from / .to (region/zone)
  - requires outlierDetection enabled
### Resilience
- timeouts: spec.http[].timeout (e.g. "10s")
- retries
  - spec.http[].retries.attempts
  - retries.perTryTimeout
  - retries.retryOn ("5xx,gateway-error,connect-failure,refused-stream")
- circuit breaking via DestinationRule.trafficPolicy
  - connectionPool.tcp.maxConnections
  - connectionPool.http.http1MaxPendingRequests
  - connectionPool.http.maxRequestsPerConnection
  - connectionPool.http.http2MaxRequests
  - outlierDetection.consecutive5xxErrors
  - outlierDetection.interval
  - outlierDetection.baseEjectionTime
  - outlierDetection.maxEjectionPercent
- failover via localityLbSetting + outlierDetection
### Fault Injection
- spec.http[].fault.delay.fixedDelay + .percentage.value
- spec.http[].fault.abort.httpStatus + .percentage.value
- spec.http[].fault.abort.grpcStatus
- chaos testing combined with `istioctl proxy-config route`

## Securing Workloads (25%)
### Authentication
- PeerAuthentication (security.istio.io/v1)
  - spec.mtls.mode: STRICT, PERMISSIVE, DISABLE, UNSET
  - scope: mesh-wide (istio-system, no selector), namespace, workload (spec.selector.matchLabels)
  - spec.portLevelMtls[port].mode
- RequestAuthentication (security.istio.io/v1)
  - spec.jwtRules[].issuer
  - spec.jwtRules[].jwksUri / .jwks
  - spec.jwtRules[].audiences[]
  - spec.jwtRules[].forwardOriginalToken: true
  - spec.jwtRules[].outputPayloadToHeader
  - spec.jwtRules[].fromHeaders / .fromParams
### Mutual TLS (mTLS)
- automatic sidecar-to-sidecar mTLS via Envoy
- SPIFFE identity: spiffe://cluster.local/ns/<ns>/sa/<serviceaccount>
- X.509 workload certs issued by istiod CA (istio-ca-secret)
- certificate rotation: default 24h workload cert TTL, --cert-ttl
- istioctl proxy-config secret <pod> (inspect cert)
- verify with `istioctl authn tls-check`
### Authorization
- AuthorizationPolicy (security.istio.io/v1)
  - spec.action: ALLOW, DENY, AUDIT, CUSTOM
  - spec.rules[].from[].source.principals (SPIFFE id)
  - spec.rules[].from[].source.namespaces / .ipBlocks / .requestPrincipals
  - spec.rules[].to[].operation.methods (GET/POST) / .paths / .ports / .hosts
  - spec.rules[].when[].key (request.headers, source.ip, request.auth.claims[iss])
  - spec.selector.matchLabels (workload scope)
  - deny-all baseline: empty spec rules with action ALLOW
  - allow-all: rules with {} matching everything
  - CUSTOM action -> extensionProvider (ext_authz)
### Securing Edge / Gateway Traffic
- TLS termination: Gateway server.tls.mode SIMPLE + credentialName
- mutual TLS at edge: tls.mode MUTUAL + caCertificates
- SDS delivers certs to ingress gateway dynamically (no pod restart)
- credentialName -> kubernetes.io/tls secret (tls.crt, tls.key)
- verify: `istioctl proxy-config secret istio-ingressgateway-<id>`

## Installation, Upgrade & Configuration (20%)
### Installation Methods
- `istioctl install --set profile=default`
- IstioOperator CRD (install.istio.io/v1alpha1)
  - spec.profile / spec.components / spec.values
  - `istioctl install -f operator.yaml`
- Helm charts: `helm install istio-base`, `istiod`, `istio-ingress`
- profiles: default, demo, minimal, ambient, empty, preview, openshift
  - inspect: `istioctl profile dump demo`
  - diff: `istioctl profile diff default demo`
### Data Plane Modes
- Sidecar mode (Envoy injected as istio-proxy container)
  - automatic injection: namespace label istio-injection=enabled OR istio.io/rev=<rev>
  - manual: `istioctl kube-inject -f deploy.yaml`
  - init container istio-init / CNI plugin sets iptables
- Ambient mode
  - ztunnel DaemonSet (L4 mTLS overlay, HBONE on :15008)
  - waypoint proxy (L7): `istioctl waypoint apply --namespace`
  - label namespace istio.io/dataplane-mode=ambient
### Configuration & Customization
- MeshConfig (meshConfig.* in IstioOperator)
  - meshConfig.accessLogFile: /dev/stdout
  - meshConfig.defaultConfig.holdApplicationUntilProxyStarts
  - meshConfig.enableTracing / meshConfig.trustDomain
- IstioOperator spec.components.pilot.k8s.resources overlays
- enable/disable: components.ingressGateways[].enabled, egressGateways[].enabled
- Sidecar resource (networking.istio.io/v1)
  - spec.egress[].hosts (limit config scope)
  - spec.workloadSelector
### Upgrades
- canary upgrade: install new revision `istioctl install --revision=1-22-0`
- relabel namespace istio.io/rev=1-22-0 + restart workloads
- in-place upgrade: `istioctl upgrade`
- revision tags: `istioctl tag set prod --revision 1-22-0`
- rollback: relabel to prior revision + `kubectl rollout restart`

## Troubleshooting (20%)
### Configuration Issues
- `istioctl analyze` (whole namespace/cluster)
- `istioctl analyze --all-namespaces`
- `istioctl validate -f config.yaml`
- detecting conflicting VirtualService host overlaps (IST0109)
- missing DestinationRule subset referenced by VirtualService (IST0101)
### Control Plane
- `kubectl logs -n istio-system deploy/istiod`
- `istioctl proxy-status` (CDS/LDS/RDS/EDS state: SYNCED, STALE, NOT SENT)
- istiod xDS push metrics (pilot_xds_pushes)
- `istioctl x internal-debug` for config dump
### Data Plane
- `istioctl proxy-config clusters <pod>`
- `istioctl proxy-config listeners <pod>`
- `istioctl proxy-config routes <pod>`
- `istioctl proxy-config endpoints <pod>`
- `istioctl proxy-config bootstrap <pod>`
- `istioctl proxy-config secret <pod>`
- Envoy access logs: `kubectl logs <pod> -c istio-proxy`
- injection check: `kubectl get pod -o jsonpath` for istio-proxy container
- `istioctl dashboard envoy <pod>` (admin :15000)
### Observability for Debugging
- Envoy/Prometheus metrics: istio_requests_total, istio_request_duration_milliseconds
- tracing: Jaeger / Zipkin via meshConfig.defaultConfig.tracing
- Kiali: `istioctl dashboard kiali` (service graph, validations)
- Grafana: `istioctl dashboard grafana` (Istio mesh/service/workload dashboards)
