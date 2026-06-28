---
title: OpenTelemetry Certified Associate (OTCA)
markmap:
  colorFreezeLevel: 2
---

# OTCA

## Fundamentals of Observability (18%)
### Observability Concepts
- monitoring (known dashboards) vs observability (unknown unknowns)
- three signals
  - traces (request flow across services)
  - metrics (numeric aggregates over time)
  - logs (timestamped records)
- telemetry lifecycle: generation -> collection -> processing -> export -> analysis
### Telemetry Signals
- Traces
  - Span (operation with start/end timestamps)
  - SpanContext propagated to children
  - parent spanId links child to parent
  - trace = DAG of spans sharing traceId
- Metrics
  - Counter (monotonic), UpDownCounter
  - Gauge / ObservableGauge
  - Histogram (explicit bucket boundaries)
  - aggregation temporality: delta vs cumulative
- Logs
  - LogRecord with severityNumber, body, attributes
  - correlation via traceId + spanId fields in log record
### Semantic Conventions
- attribute keys: service.name, service.version, service.namespace
- resource attributes vs span attributes
- HTTP: http.request.method, http.response.status_code, url.path
- DB: db.system, db.namespace, db.query.text
- messaging: messaging.system, messaging.operation
- RPC: rpc.system, rpc.method
### Instrumentation Concepts
- manual instrumentation (tracer.start_span in code)
- automatic / zero-code (Java -javaagent, Python opentelemetry-instrument)
- instrumentation libraries per framework (Flask, Express, gRPC)
### Analysis & Outcomes
- root cause analysis via trace waterfall
- SLI (e.g. p99 latency) / SLO (target) / SLA (contract)
- error budget from SLO
- cross-signal correlation via exemplars (metric -> trace)

## The OpenTelemetry API and SDK (46%)
### Data Model
- Resource (immutable key-value describing producer)
  - merge precedence, resource detectors (env, host, process)
- InstrumentationScope (name, version of instrumenting library)
- signal data models: Span, Metric (data point), LogRecord
### Signals - Tracing
- TracerProvider -> Tracer (name + version)
- Span fields
  - name
  - SpanKind: SERVER, CLIENT, PRODUCER, CONSUMER, INTERNAL
  - attributes (key-value)
  - events (timestamped, addEvent)
  - links (to other SpanContexts)
  - status: UNSET, OK, ERROR
- SpanContext: traceId (16 bytes), spanId (8 bytes), traceFlags (sampled bit), traceState
- Sampling
  - AlwaysOn / AlwaysOff
  - TraceIdRatioBased(ratio)
  - ParentBased(root=...)
- SpanProcessor
  - SimpleSpanProcessor (synchronous export)
  - BatchSpanProcessor (maxQueueSize, scheduledDelayMillis, maxExportBatchSize)
### Signals - Metrics
- MeterProvider -> Meter
- instruments
  - Counter.add() (synchronous, monotonic)
  - UpDownCounter.add()
  - Histogram.record()
  - ObservableCounter / ObservableGauge / ObservableUpDownCounter (callbacks)
- synchronous (recorded inline) vs asynchronous (observed via callback)
- View (rename, drop attributes, set aggregation/bucket boundaries)
- aggregation temporality: CUMULATIVE, DELTA
- MetricReader
  - PeriodicExportingMetricReader (exportIntervalMillis)
### Signals - Logs
- LoggerProvider -> Logger
- LogRecord: timestamp, severityNumber, severityText, body, attributes, traceId, spanId
- log appender bridge (Logback, log4j, Python logging)
- LogRecordProcessor (SimpleLogRecordProcessor, BatchLogRecordProcessor)
### Context Propagation
- Context API (attach/detach, current context)
- Propagators
  - W3C TraceContext: traceparent header (00-traceid-spanid-flags), tracestate
  - W3C Baggage: baggage header (key=value pairs)
  - B3 (Zipkin): b3 / X-B3-TraceId / X-B3-SpanId
- propagator.inject(carrier) / propagator.extract(carrier) at process boundaries
### SDK Pipelines
- TracerProvider -> SpanProcessor -> SpanExporter
- Exporters
  - OTLP/gRPC (:4317)
  - OTLP/HTTP (:4318, /v1/traces /v1/metrics /v1/logs)
  - ConsoleSpanExporter / logging
  - Prometheus exporter (metrics scrape)
- BatchSpanProcessor: maxQueueSize, maxExportBatchSize, exportTimeoutMillis
### Configuration
- env vars
  - OTEL_SERVICE_NAME
  - OTEL_EXPORTER_OTLP_ENDPOINT
  - OTEL_EXPORTER_OTLP_PROTOCOL (grpc, http/protobuf)
  - OTEL_TRACES_SAMPLER (parentbased_traceidratio)
  - OTEL_RESOURCE_ATTRIBUTES
  - OTEL_PROPAGATORS (tracecontext,baggage)
- programmatic SDK builder configuration
- declarative file config (OTEL_EXPERIMENTAL_CONFIG_FILE, config.yaml)
- resource detection (OTEL_RESOURCE_ATTRIBUTES + detectors)
### Composability & Agents
- no-op API default when no SDK registered
- auto-instrumentation agents (opentelemetry-javaagent.jar)
- distributions (e.g. opentelemetry-java vs vendor distro)

## The OpenTelemetry Collector (26%)
### Architecture & Pipelines
- pipeline = receivers -> processors -> exporters
- connectors (e.g. spanmetrics, forward, count) bridge two pipelines
- extensions: health_check, pprof, zpages, basicauth
- service.pipelines.traces / metrics / logs in config.yaml
- run: `otelcol --config config.yaml`
### Receivers
- otlp (protocols.grpc :4317, protocols.http :4318)
- prometheus (scrape_configs)
- jaeger, zipkin
- filelog (include globs, operators)
- hostmetrics (cpu, memory, disk scrapers)
- push (otlp) vs pull (prometheus, hostmetrics)
### Processors
- batch (send_batch_size, timeout)
- memory_limiter (limit_mib, spike_limit_mib, check_interval)
- attributes (insert/update/delete keys)
- resource (set resource attrs)
- filter (drop spans/metrics by condition)
- transform (OTTL statements, e.g. set(attributes["x"], "y"))
- tail_sampling (policies: latency, status_code, probabilistic)
### Exporters
- otlp / otlphttp
- prometheus (expose /metrics) / prometheusremotewrite
- backend-specific (e.g. datadog, otlp to vendor)
- debug (verbosity: detailed)
### Deployment Patterns
- Agent: sidecar or DaemonSet on each node
- Gateway: standalone collector Deployment behind LB
- no-collector: SDK exports OTLP directly to backend
### Scaling
- horizontal scaling of gateway collectors
- loadbalancing exporter (routing_key: traceID) for tail_sampling consistency
- OpenTelemetry Operator (OpenTelemetryCollector CRD, auto-instrumentation injection)
### Distributions
- core (otelcol) vs contrib (otelcol-contrib)
- ocb (OpenTelemetry Collector Builder) with builder-config.yaml (dist, receivers, exporters modules)

## Maintaining and Debugging Observability Pipelines (10%)
### Pipeline Debugging
- debug exporter (verbosity: detailed) to print telemetry
- zpages extension (/debug/tracez, /debug/pipelinez on :55679)
- collector self-metrics (otelcol_receiver_accepted_spans, otelcol_exporter_sent_spans)
- health_check extension (:13133 /)
### Context Propagation Issues
- broken trace: missing traceparent injection at HTTP client
- mismatched propagators between services (B3 vs W3C)
- verify with OTEL_PROPAGATORS alignment
### Error Handling
- exporter sending_queue (enabled, num_consumers, queue_size)
- retry_on_failure (initial_interval, max_interval, max_elapsed_time)
- memory_limiter processor prevents OOM (drops on spike)
- backpressure: queue full -> data dropped (otelcol_exporter_send_failed_spans)
### Schema & Versioning
- schemaUrl on Resource/InstrumentationScope
- semantic convention version (e.g. 1.27.0)
- schema transformation (schema_processor / telemetry schema files)
