---
title: AWS Certified Solutions Architect – Professional (SAP-C02)
markmap:
  colorFreezeLevel: 2
---

# SAP-C02

## Design Solutions for Organizational Complexity (26%)
### Task 1.1 Architect network connectivity strategies
- Multi-VPC connectivity
  - VPC peering (non-transitive, no overlapping CIDRs, 1:1)
  - AWS Transit Gateway (hub-and-spoke, route tables, inter-Region peering)
    - TGW Connect (GRE/BGP for SD-WAN), multicast support
  - AWS PrivateLink: interface endpoint + endpoint service (NLB front)
- Hybrid connectivity
  - Direct Connect: dedicated/hosted, private VIF (VPC) + public VIF (S3) + transit VIF (TGW)
    - DX Gateway to reach VPCs in multiple Regions
    - LAG (link aggregation) + MACsec encryption
  - Site-to-Site VPN over DX public VIF for encryption
  - Transitive routing via Transit Gateway, not VPC peering
- Hybrid DNS
  - Route 53 Resolver inbound endpoint (on-prem → AWS)
  - Route 53 Resolver outbound endpoint + forwarding rules (AWS → on-prem)
  - Route 53 Private Hosted Zones associated to VPCs
- Network design & monitoring
  - Non-overlapping CIDR planning, IPAM (IP Address Manager)
  - VPC Flow Logs → CloudWatch Logs / S3 (ACCEPT/REJECT records)
  - Traffic Mirroring to monitoring appliances
  - Region/AZ choice by latency; Global Accelerator anycast entry
### Task 1.2 Prescribe security controls
- Identity and access
  - IAM Identity Center permission sets across Organizations
  - Cross-account IAM roles + `sts:AssumeRole` with `ExternalId`
  - ABAC: `aws:PrincipalTag` / `aws:ResourceTag` condition keys
  - Third-party federation: SAML 2.0, OIDC
- Data and detection
  - KMS multi-Region keys; key policy + grants; `aws:ViaService` condition
  - ACM public certs (auto-renew) + ACM Private CA
  - CloudTrail organization trail → central S3 bucket
  - AWS Config aggregator (multi-account/Region)
  - GuardDuty delegated admin; Security Hub central findings (ASFF, CSPM standards)
### Task 1.3 Design reliable and resilient architectures
- DR and recovery
  - RTO/RPO mapped to strategy (backup&restore → active-active)
  - AWS Elastic Disaster Recovery (DRS) continuous block replication
  - Aurora Global Database (<1s replication, RPO ~1s, RTO <1min)
  - Auto Scaling self-healing + Route 53 ARC (Application Recovery Controller) routing controls
  - AWS Backup: backup plans, vault lock, cross-account/Region copy
### Task 1.4 Design a multi-account AWS environment
- Governance at scale
  - AWS Organizations: OUs, SCPs (allow-list vs deny-list), tag policies
  - Control Tower: Account Factory, mandatory/elective guardrails, AFT (Account Factory for Terraform)
  - AWS RAM: share TGW, subnets, License Manager, Resolver rules
  - Centralized logging: org CloudTrail + Config to Log Archive account
  - Org-wide EventBridge bus for cross-account notifications
### Task 1.5 Determine cost optimization and visibility strategies
- Cost governance
  - Cost Explorer + CUR 2.0 → Athena; Trusted Advisor limit/cost checks
  - Purchasing: Convertible RIs, Compute Savings Plans, Spot fleets
  - Cost allocation tags + AWS Budgets actions (auto-apply SCP/IAM)
  - Compute Optimizer + Trusted Advisor rightsizing recommendations

## Design for New Solutions (29%)
### Task 2.1 Design a deployment strategy
- Delivery automation
  - CloudFormation: nested stacks, StackSets, change sets, drift detection
  - CI/CD: CodePipeline → CodeBuild → CodeDeploy; or third-party
  - Systems Manager: State Manager, Automation runbooks, Patch Manager
  - Managed services (Fargate, Aurora Serverless) to cut undifferentiated heavy lifting
### Task 2.2 Design a solution to ensure business continuity
- Resilient DR
  - Backup&restore, pilot light, warm standby, active-active selection by RTO/RPO
  - Aurora Global DB, DynamoDB global tables, S3 CRR for data replication
  - Route 53 ARC + readiness checks; regular DR game-day testing
  - AWS Backup centralized + cross-Region/account copy with vault lock
### Task 2.3 Determine security controls based on requirements
- Defense in depth
  - IAM least privilege + permissions boundaries + Access Analyzer
  - Security groups (stateful) + NACLs (stateless) layered controls
  - KMS encryption at rest, TLS/ACM in transit; Secrets Manager rotation
  - AWS Shield Advanced + WAF (rate-based, managed rules) for L7/DDoS
  - Patch Manager baselines for compliance; PrivateLink for private endpoints
### Task 2.4 Design a strategy to meet reliability requirements
- High availability
  - Multi-AZ (RDS sync standby) / multi-Region (Aurora Global DB)
  - S3 replication: CRR/SRR, Replication Time Control (RTC 15-min SLA)
  - Auto Scaling target tracking; SNS/SQS decoupling; Service Quotas monitoring
  - Route 53 latency/failover/weighted routing + health checks
### Task 2.5 Design a solution to meet performance objectives
- Performance design
  - Storage tier match (gp3/io2, EFS, FSx); instance family by workload
  - Purpose-built DBs: DynamoDB, Aurora, ElastiCache, Neptune, Timestream, DocumentDB
  - Caching: CloudFront, ElastiCache, DAX; RDS/Aurora read replicas
  - Elasticity via Auto Scaling + Compute Optimizer rightsizing
### Task 2.6 Determine a cost optimization strategy
- Cost-aware design
  - Savings Plans/RIs for steady, Spot for interruptible
  - S3 Intelligent-Tiering, lifecycle to Glacier Deep Archive
  - CloudFront + VPC Gateway endpoints to cut data-transfer cost
  - Replace EC2 with Lambda/Fargate managed services where fit

## Continuous Improvement for Existing Solutions (25%)
### Task 3.1 Improve overall operational excellence
- Operations automation
  - CloudWatch metrics/alarms/dashboards; Logs Insights queries
  - Deployment: CodeDeploy blue/green, canary (Lambda alias traffic shift)
  - Systems Manager Automation runbooks + State Manager desired state
  - AWS FIS experiments (CPU/network fault injection) for game days
### Task 3.2 Improve security
- Security hardening
  - Secrets Manager (rotation) vs SSM Parameter Store (SecureString, free)
  - IAM Access Analyzer unused-access + external-access findings
  - Inspector continuous CVE scanning (EC2/ECR/Lambda)
  - Patch Manager + automated remediation via Config + SSM
  - CloudTrail full traceability + CloudTrail Lake queries
### Task 3.3 Improve performance
- Performance tuning
  - Auto Scaling + Compute Optimizer; Global Accelerator + CloudFront edge
  - CloudWatch + X-Ray service map to find bottlenecks; map SLA/KPI → metrics
  - Add read replicas/DAX/ElastiCache; switch to gp3/io2 storage
### Task 3.4 Improve reliability
- Reliability remediation
  - DynamoDB global tables, Aurora replicas, S3 CRR replication
  - Auto Scaling self-healing + ELB health checks remove SPOF
  - Service Quotas + Trusted Advisor limit monitoring
  - Resilience Hub assessments against RTO/RPO targets
### Task 3.5 Identify opportunities for cost optimizations
- Cost reduction
  - Trusted Advisor idle/low-util + Cost Explorer rightsizing
  - Spot, Convertible RIs, Compute Savings Plans adoption
  - AWS Budgets alarms; CUR + Athena granular analysis; cost-allocation tags

## Accelerate Workload Migration and Modernization (20%)
### Task 4.1 Select workloads and processes for migration
- Migration assessment
  - Migration Hub central tracking + Strategy Recommendations
  - Application Discovery Service (Agentless connector / agent)
  - Portfolio + wave planning; TCO via Migration Evaluator
### Task 4.2 Determine the optimal migration approach
- Migration tooling
  - 7 Rs: rehost, replatform, repurchase, refactor, relocate, retain, retire
  - Data: DataSync (online), Snowball Edge / Snowmobile (offline)
  - Databases: AWS DMS + Schema Conversion Tool (SCT) for heterogeneous
  - Application Migration Service (MGN) for lift-and-shift rehost
  - Networking via Direct Connect/VPN; governance via Control Tower
### Task 4.3 Determine a new architecture for existing workloads
- Platform selection
  - Compute: EC2, ECS, EKS, Fargate
  - Storage: EBS, EFS, FSx, S3
  - Databases: DynamoDB, Aurora, OpenSearch Service, ElastiCache
### Task 4.4 Determine opportunities for modernization
- Modernization
  - Serverless: Lambda + API Gateway; Fargate for containers
  - Purpose-built DBs: DynamoDB, Aurora Serverless v2
  - Decouple with SQS/SNS/EventBridge; orchestrate with Step Functions
  - Strangler-fig refactor; App2Container / Porting Assistant for .NET

## Exam Essentials
### Key facts
- 75 questions; scaled 100–1000; pass = 750
- Compensatory scoring; multiple choice + multiple response
- 180 minutes; scenario-heavy, multi-account + hybrid + migration focus
