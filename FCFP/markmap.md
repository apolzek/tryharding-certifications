---
title: FinOps Certified Professional (FinOps Foundation)
markmap:
  colorFreezeLevel: 2
---

<!--
  ACRONYM NOTE / UNCERTAINTY:
  The folder is named "FCFP". The FinOps Foundation's professional-tier
  certification is officially the "FinOps Certified Professional" (commonly
  abbreviated FCP). "FCFP" is most plausibly an expansion of
  "FinOps Certified FinOps Professional" referring to this same exam.
  IMPORTANT: the FinOps Foundation does NOT publish numeric percentage
  weights for the Professional exam (unlike Linux Foundation exams).
  This map is therefore organized around the official FinOps Framework
  2025 (Domains + Capabilities), which is the published basis of the exam.
  Domains are shown WITHOUT invented percentages because none are official.
  Sources: learn.finops.org (FinOps Certified Professional path),
  training.linuxfoundation.org/certification/certified-finops/,
  finops.org/framework (mirrored via learn.microsoft.com FinOps Framework).
-->

# FCFP

## FinOps Principles
### Core principles (2025)
- Teams need to collaborate
  - Engineering + Finance shared cadence
  - Common vocabulary (FinOps lexicon)
- Business value drives technology decisions
  - Unit economics over absolute cost
  - Trade-off: cost vs speed vs quality
- Everyone takes ownership for their cloud usage
  - Decentralized accountability to teams
  - Cost visibility at team/product level
- FinOps data should be accessible, timely, and accurate
  - Near-real-time billing data feeds
  - Granular cost + usage records (CUR / billing export)
- FinOps should be enabled centrally
  - Central FinOps team / Cloud Cost Center of Excellence
  - Shared tooling and rate negotiation
- Take advantage of the variable cost model of the cloud
  - Elastic scaling / pay-as-you-go
  - Just-in-time provisioning

## FinOps Lifecycle (Inform / Optimize / Operate)
### Inform
- Cost visibility and transparency
  - Cost allocation (tags / labels)
  - Account / subscription / project hierarchy
- Allocation, showback, and chargeback
  - Showback vs chargeback models
  - Amortization of commitments
  - Shared cost split (proportional / even)
- Benchmarking, budgeting, and forecasting
  - Trend-based vs driver-based forecast
  - Budget vs actual variance
- Shared accountability across teams
  - Tag governance / mandatory tag keys
  - Unallocated/untagged spend reduction
### Optimize
- Reduce cloud waste / improve efficiency
  - Idle resource detection
  - Storage tiering (S3 lifecycle / Intelligent-Tiering)
- Rate optimization (commitments, discounts)
  - Reserved Instances (RI)
  - Savings Plans (Compute / EC2)
  - Committed Use Discounts (GCP CUD)
  - Spot / preemptible instances
  - Commitment coverage % and utilization %
- Workload optimization (rightsizing, scheduling)
  - Rightsizing (instance family/size match)
  - Autoscaling (HPA / cluster autoscaler)
  - Off-hours scheduling (dev/test shutdown)
- Eliminate idle and unused resources
  - Orphaned EBS volumes / unattached IPs
  - Zombie / stopped-but-billed resources
### Operate
- Define, track, and monitor KPIs
  - KPI: effective savings rate
  - KPI: commitment coverage / utilization
  - KPI: cost per unit ($/transaction)
  - KPI: forecast accuracy / variance
- Governance and policy enforcement
  - Tagging policy enforcement (AWS SCP / Azure Policy)
  - Budget alerts / spend guardrails
- Establish cadences and continuous improvement
  - Weekly cost review / monthly true-up
  - Anomaly detection and remediation loop
- Align cloud spend with business objectives
  - Map spend to product / revenue line
  - Margin / gross-margin analysis

## FinOps Scopes
### Areas of responsibility (2025 core element)
- Public, private, and hybrid cloud (Cloud+)
  - On-prem / private cloud cost normalization
- SaaS
  - License utilization (seats vs active users)
- Data / AI
  - GPU instance cost (training/inference)
  - Token-based pricing (LLM API spend)
- Data centers
  - Power, cooling, depreciation allocation
- Licensing and professional services
  - BYOL (Bring Your Own License)
  - Marketplace / private offers

## FinOps Personas / Stakeholders
### Core stakeholders
- FinOps practitioners
  - FinOps analyst / FinOps lead
- Engineering / product owners
  - SRE / platform / DevOps teams
- Finance and procurement
  - FP&A, accounting, vendor negotiation
- Leadership and business owners
  - CFO / CTO / business unit owners
### Allied stakeholders
- Sustainability practitioners
  - Carbon footprint / cloud carbon reporting
- ITFM / TBM teams
  - Technology Business Management taxonomy
- ITSM / ITIL teams
  - CMDB / change management integration
- ITAM teams
  - Software asset / license inventory
- Security teams
  - Cost of security tooling / compliance spend

## Domain: Understand Usage and Cost
### Capabilities
- Data ingestion
  - Normalize and aggregate billing data
    - AWS CUR (Cost and Usage Report)
    - Azure Cost Management exports
    - GCP Billing BigQuery export
  - FOCUS schema adoption
    - FOCUS spec (FinOps Open Cost & Usage Spec)
    - Common columns: BilledCost, EffectiveCost, ServiceName
- Allocation
  - Tagging / labeling and account hierarchies
    - Cost allocation tags (AWS) / labels (GCP)
    - Account / management-group hierarchy
  - Showback and chargeback models
    - Showback vs chargeback
    - Amortized vs unblended cost
- Reporting and analytics
  - Dashboards, KPIs, trend analysis
    - Cost Explorer / Azure Cost Analysis
    - Grafana / QuickSight cost dashboards
- Anomaly management
  - Detect, alert, and remediate cost spikes
    - AWS Cost Anomaly Detection
    - Statistical / ML-based anomaly detection
    - Alert routing (Slack / PagerDuty)

## Domain: Quantify Business Value
### Capabilities
- Planning and estimating
  - Pricing calculators (AWS Pricing Calculator)
  - Pre-deployment cost estimates (Infracost)
- Forecasting
  - Trend-based and driver-based forecasts
  - Forecast accuracy KPI (MAPE)
- Budgeting
  - Budget vs. actual variance tracking
  - AWS Budgets / Azure Budgets with thresholds
- Benchmarking
  - Internal and industry comparisons
  - Effective savings rate benchmark
- Unit economics
  - Cost per business metric (e.g. cost per transaction)
  - $/customer, $/order, cost-to-serve

## Domain: Optimize Usage and Cost
### Capabilities
- Architecting for the cloud
  - Cost-aware design and trade-offs
    - Serverless vs provisioned trade-off
    - Managed service vs self-hosted cost
- Workload optimization
  - Rightsizing, autoscaling, scheduling
    - AWS Compute Optimizer recommendations
    - Karpenter / cluster autoscaler
- Rate optimization
  - Reserved instances / savings plans / committed use
    - RI / Savings Plans / GCP CUD
  - Discount coverage and utilization
    - Coverage % and utilization % targets
    - Enterprise Discount Program (EDP/PPA)
- Licensing and SaaS
  - License optimization and BYOL
    - BYOL (Bring Your Own License)
    - SaaS seat reclamation
- Cloud sustainability
  - Carbon-aware optimization
    - Cloud Carbon Footprint tool
    - Region/scheduling for lower carbon intensity

## Domain: Manage the FinOps Practice
### Capabilities
- FinOps education and enablement
  - FinOps lexicon / internal training
- FinOps practice operations
  - RACI for cost ownership
  - Cadence meetings and runbooks
- Onboarding workloads
  - Tagging standards at provisioning (IaC)
- Policy and governance
  - Guardrails: AWS SCP / Azure Policy
  - Budget enforcement / quota limits
- Invoicing and chargeback
  - Internal cross-charge / cost center mapping
- FinOps assessment
  - Maturity model (Crawl-Walk-Run)
  - FinOps Foundation maturity assessment
- FinOps tools and services
  - Native: Cost Explorer, Azure Cost Management
  - Third-party: CloudHealth, Cloudability, Apptio
- Intersecting frameworks
  - ITFM/TBM, ITSM, ITAM, sustainability
    - TBM taxonomy / ITIL / ISO 19770 (ITAM)
