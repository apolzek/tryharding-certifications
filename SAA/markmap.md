---
title: AWS Certified Solutions Architect – Associate (SAA-C03)
markmap:
  colorFreezeLevel: 2
---

# SAA-C03

## Design Secure Architectures (30%)
### Design secure access to AWS resources
- Identity and federation
  - IAM principals
    - Users, groups, roles
    - Identity-based policies (attached to principal)
    - Resource-based policies (attached to S3 bucket, KMS key, SQS, Lambda)
      - `Principal` element + cross-account `arn:aws:iam::ACCOUNT:root`
    - Policy evaluation: explicit `Deny` > `Allow` > implicit deny
    - Condition keys: `aws:SourceIp`, `aws:PrincipalOrgID`, `aws:MultiFactorAuthPresent`
  - AWS IAM Identity Center (SSO)
    - Permission sets mapped to accounts/groups
    - SCIM provisioning from external IdP (Okta, Entra ID)
  - AWS STS temporary credentials
    - `aws sts assume-role --role-arn ARN --role-session-name NAME`
    - `aws sts assume-role-with-web-identity` (OIDC)
    - `aws sts get-session-token` with `--serial-number` MFA
    - Max session duration: `DurationSeconds` up to 43200s (12h)
  - Federation
    - SAML 2.0 → `sts:AssumeRoleWithSAML`
    - OIDC (Cognito, GitHub Actions) → `sts:AssumeRoleWithWebIdentity`
    - IAM roles trust policy `sts:AssumeRoleWithWebIdentity`
- Multi-account security
  - AWS Organizations
    - Service Control Policies (SCPs) set permission ceiling
    - Organizational Units (OUs) hierarchy
    - `aws organizations attach-policy --policy-id p-xxxx`
  - AWS Control Tower
    - Account Factory provisions accounts
    - Guardrails (preventive = SCP, detective = AWS Config rule)
    - Landing zone with Log Archive + Audit accounts
  - Cross-account access
    - IAM role with trust policy + `ExternalId` for third parties
    - AWS RAM to share subnets, Transit Gateway
- Foundational controls
  - Least privilege via IAM Access Analyzer findings
  - MFA: virtual (TOTP) or FIDO2 hardware key on root
  - Root: delete root access keys, enable MFA, lock away
  - Shared Responsibility: AWS = "of" the cloud, customer = "in" the cloud
  - Global infra: Regions, AZs (≥3 per Region), Local Zones, Wavelength
### Design secure workloads and applications
- Network security
  - Security groups (stateful, allow-only, evaluated all rules)
  - Network ACLs (stateless, ordered rules, allow + deny, ephemeral ports 1024-65535)
  - Route tables: `0.0.0.0/0` → IGW (public) vs NAT (private)
  - NAT Gateway (managed, AZ-bound) vs NAT instance
  - VPC endpoints
    - Gateway endpoint (S3, DynamoDB) → route table entry, free
    - Interface endpoint (PrivateLink, ENI) → DNS name, hourly + data cost
- Edge and app protection
  - AWS WAF: managed rule groups, rate-based rules, SQLi/XSS match
  - AWS Shield Standard (free) vs Shield Advanced (cost protection, DRT)
  - AWS Firewall Manager: org-wide WAF/Shield policy
  - Amazon Cognito: user pools (auth) + identity pools (AWS creds)
  - GuardDuty: analyzes CloudTrail, VPC Flow Logs, DNS logs
  - Amazon Macie: classifies PII/PHI in S3 buckets
  - Secrets Manager: automatic rotation via Lambda
- Hybrid connectivity
  - Site-to-Site VPN: IPsec tunnels over internet (~1.25 Gbps)
  - AWS Direct Connect: dedicated 1/10/100 Gbps + VPN backup
  - Direct Connect + VPN = IPsec over private link
### Determine appropriate data security controls
- Encryption at rest
  - AWS KMS
    - Symmetric AES-256 keys; key policy + grants
    - Automatic rotation `aws kms enable-key-rotation` (yearly)
    - AWS managed vs customer managed keys (CMK)
    - Envelope encryption (data key wrapped by CMK)
  - AWS CloudHSM: FIPS 140-2 Level 3 dedicated HSM
  - S3 encryption: SSE-S3 (AES256), SSE-KMS, SSE-C, DSSE-KMS
    - `aws s3 cp file s3://b --sse aws:kms --sse-kms-key-id ID`
  - EBS: encrypt-by-default account setting; RDS encryption at creation only
- Encryption in transit
  - TLS via ACM (free public certs, auto-renewal)
  - ACM cert on ALB/CloudFront; private CA via ACM PCA
  - Enforce HTTPS: ALB redirect 80→443; S3 bucket policy `aws:SecureTransport=false` deny
- Data governance
  - S3 lifecycle: transition + expiration rules, object versioning
  - S3 Object Lock (WORM): governance vs compliance mode
  - Cross-Region Replication (CRR), Same-Region Replication (SRR)
  - AWS Artifact (SOC/PCI reports), AWS Config conformance packs

## Design Resilient Architectures (26%)
### Design scalable and loosely coupled architectures
- Decoupling and messaging
  - Amazon SQS
    - Standard (at-least-once, best-effort order) vs FIFO (exactly-once, `.fifo` suffix)
    - Visibility timeout (default 30s, max 12h); long polling `WaitTimeSeconds=20`
    - Dead-letter queue after `maxReceiveCount`
  - Amazon SNS: fan-out to SQS/Lambda/HTTP; FIFO topics; message filtering
  - Amazon EventBridge: event bus, rules with event patterns, schema registry
  - AWS Step Functions: Standard vs Express workflows, ASL state machine
- Compute scaling
  - EC2 Auto Scaling: launch templates, min/max/desired
    - Target tracking (e.g. CPU 50%), step scaling, scheduled
    - Lifecycle hooks; cooldown/warm-up
  - AWS Lambda: `Timeout` max 900s, `MemorySize` 128-10240MB, reserved/provisioned concurrency
  - AWS Fargate (serverless containers for ECS/EKS)
  - Amazon ECS (task definitions) vs Amazon EKS (managed Kubernetes)
- API and edge
  - API Gateway: REST (usage plans, API keys) vs HTTP API (cheaper) vs WebSocket
  - Application Load Balancer: path/host routing, target groups, sticky sessions
  - CloudFront: cache behaviors, OAC for S3 origin, TTL settings
  - ElastiCache: Redis (replication, persistence) vs Memcached (multithreaded)
  - RDS read replicas (async) for read scaling
### Design highly available and/or fault-tolerant architectures
- HA across AZs/Regions
  - Multi-AZ: RDS standby (synchronous), failover via DNS CNAME swap
  - Route 53 routing
    - Failover (primary/secondary + health checks)
    - Latency-based, geolocation, weighted, multivalue
    - Health checks: endpoint, calculated, CloudWatch alarm
  - Eliminate SPOF: ≥2 AZs behind ELB
  - RDS Proxy: connection pooling, reduces failover time
- Disaster recovery strategies
  - Backup & restore (RPO/RTO hours, cheapest)
  - Pilot light (core DB replicated, minimal compute off)
  - Warm standby (scaled-down full stack running)
  - Multi-site active-active (RPO/RTO near zero, costliest)
  - RPO = data loss tolerance; RTO = downtime tolerance
- Reliability tooling
  - S3 11 9's durability; EBS snapshots to S3; AWS Backup plans
  - AWS X-Ray: service map, trace segments, sampling rules
  - Service Quotas console + `aws service-quotas request-service-quota-increase`

## Design High-Performing Architectures (24%)
### High-performing and scalable storage
- Storage types and services
  - Object: S3 (3500 PUT / 5500 GET per prefix per sec), S3 Transfer Acceleration
  - File: EFS (NFS, multi-AZ, Elastic/Provisioned throughput) vs FSx (Windows/Lustre/NetApp/OpenZFS)
  - Block: EBS (gp3 baseline 3000 IOPS, io2 Block Express up to 256k IOPS)
    - Instance store (ephemeral NVMe) for temp high-IOPS
  - AWS Storage Gateway: File/Volume/Tape gateway for hybrid
### High-performing and elastic compute
- Compute selection
  - EC2 families: general (M/T), compute (C), memory (R/X), storage (I/D), accelerated (P/G)
  - Lambda, Fargate, AWS Batch (managed job queues), Amazon EMR (Spark/Hadoop)
  - Auto Scaling on CloudWatch metrics (CPU, ALBRequestCountPerTarget)
  - Placement groups: cluster (low latency), spread, partition
### High-performing database solutions
- Database choice
  - Relational: RDS (MySQL/PostgreSQL/etc.), Aurora (5x MySQL, 6 copies across 3 AZs)
  - Non-relational: DynamoDB (single-digit ms, partition key design)
    - Global tables (multi-Region active-active)
    - On-demand vs provisioned capacity + auto scaling
  - Caching: ElastiCache; DynamoDB DAX (microsecond reads)
  - Aurora read replicas (up to 15), RDS Proxy, Provisioned IOPS (io1/io2)
### High-performing and scalable networks
- Edge and connectivity
  - CloudFront (cacheable HTTP) vs Global Accelerator (anycast IP, TCP/UDP, non-cacheable)
  - ALB (L7 HTTP) / NLB (L4, static IP, millions req/s) / GWLB (3rd-party appliances)
  - Enhanced networking (ENA), Elastic Fabric Adapter (EFA) for HPC
  - VPC CIDR sizing (/16 to /28), subnet-per-AZ tiering
### High-performing data ingestion and transformation
- Data pipeline services
  - Kinesis Data Streams (shards, 1MB/s in per shard) vs Firehose (to S3/Redshift/OpenSearch)
  - Kinesis Data Analytics / Managed Service for Apache Flink
  - AWS Glue (serverless Spark ETL, Data Catalog crawlers)
  - Athena (serverless SQL on S3, Presto), Lake Formation (data lake permissions), QuickSight (BI)
  - AWS DataSync (online), Transfer Family (SFTP/FTPS), Storage Gateway
  - Convert `.csv` → columnar `.parquet`/ORC + partitioning for Athena cost cut

## Design Cost-Optimized Architectures (20%)
### Cost-optimized storage
- Storage cost levers
  - S3 classes: Standard, Standard-IA, One Zone-IA, Glacier Instant/Flexible/Deep Archive
  - S3 Intelligent-Tiering (auto move, no retrieval fee)
  - Lifecycle: transition to IA after 30d, Glacier after 90d, expire after Nd
  - Glacier retrieval: Expedited / Standard / Bulk; Deep Archive 12h
  - EBS: gp3 (cheaper than gp2), sc1/st1 HDD for throughput workloads
  - Snow Family (Snowcone/Snowball Edge) for petabyte offline transfer
### Cost-optimized compute
- Purchasing options
  - Spot (up to 90% off, 2-min interruption notice) for fault-tolerant
  - Reserved Instances (1/3yr, Standard vs Convertible)
  - Savings Plans (Compute / EC2 Instance / SageMaker)
  - On-Demand for spiky/unknown
  - Right-size via Compute Optimizer; EC2 hibernation; Outposts/Snowball Edge hybrid
### Cost-optimized database
- Database cost levers
  - Aurora Serverless v2 (ACU autoscaling), DynamoDB on-demand for spiky
  - Read replica only if read-heavy; reduce backup retention (max 35d automated)
  - Reserved capacity for DynamoDB/RDS steady workloads
### Cost-optimized network
- Network cost levers
  - CloudFront to cut origin egress; VPC Gateway endpoints (free S3/DynamoDB)
  - Same-AZ traffic free; cross-AZ + cross-Region charged
  - Single NAT Gateway tradeoff vs per-AZ HA; Direct Connect lowers per-GB egress

## Cost Management & Key Services
### Cost tooling
- Cost Explorer (trends, RI/SP recommendations), AWS Budgets (alerts/actions)
- Cost and Usage Report (CUR) → S3 → Athena/QuickSight
- Cost allocation tags (user + AWS generated), AWS Organizations consolidated billing
- Trusted Advisor (cost/security/perf/limits checks), Compute Optimizer rightsizing
### Exam essentials
- 65 questions (50 scored), scaled 100–1000, pass = 720
- Compensatory scoring; multiple choice + multiple response; 130 minutes
- Map requirements to Well-Architected pillars: security, reliability, performance, cost, operational excellence, sustainability
