---
title: AWS Certified DevOps Engineer – Professional (DOP-C02)
markmap:
  colorFreezeLevel: 2
---

# DOP-C02

## SDLC Automation (22%)
### Task 1.1 Implement CI/CD pipelines
- Pipeline foundations
  - SDLC phases (plan, code, build, test, release, deploy, operate)
  - Single vs multi-account (cross-account roles, KMS key sharing)
  - Repos: CodeCommit (Git), GitHub/Bitbucket via CodeStar Connections
- AWS build/deploy services
  - CodePipeline: stages, actions, transitions, source→build→deploy
    - Manual approval action; artifact store (S3 bucket)
  - CodeBuild: `buildspec.yml` phases (install/pre_build/build/post_build), artifacts
  - CodeDeploy: `appspec.yml`, deployment groups, deployment configs
  - Secrets: Secrets Manager (rotation) + SSM Parameter Store SecureString
### Task 1.2 Integrate automated testing into pipelines
- Test types and stages
  - Unit/integration/acceptance/UI tests in CodeBuild phases
  - Trigger on PR/merge via CodeCommit + EventBridge or CodeStar webhook
  - Load/stress tests at scale before prod stage
  - `reports` in buildspec (JUnit/test reports), exit-code gates, code coverage
### Task 1.3 Build and manage artifacts
- Artifact management
  - CodeArtifact (npm/PyPI/Maven), S3, Amazon ECR (container images)
  - CodeBuild produces artifacts; Lambda layers/zip
  - EC2 Image Builder pipelines (golden AMI), recipes + components
  - ECR lifecycle policies; S3 versioning + lifecycle for artifacts
### Task 1.4 Implement deployment strategies
- Deployment patterns
  - Blue/green (CodeDeploy + ALB target group swap)
  - Canary (`CodeDeployDefault.LambdaCanary10Percent5Minutes`)
  - Linear (`...Linear10PercentEvery1Minute`); all-at-once
  - Immutable (new ASG) vs mutable (in-place) — Elastic Beanstalk policies
  - Targets: EC2 (CodeDeploy agent), ECS, EKS, Lambda alias traffic shift
  - Auto rollback on CloudWatch alarm; EFS/EBS for stateful workloads

## Configuration Management and IaC (17%)
### Task 2.1 Define cloud infrastructure and reusable components
- IaC tooling
  - CloudFormation: templates, parameters, mappings, `!Ref`/`!GetAtt`, change sets, drift detection
  - AWS SAM (`sam build`, `sam deploy --guided`) for serverless
  - AWS CDK (`cdk synth`, `cdk deploy`), constructs in TS/Python
  - StackSets across accounts/Regions (self-managed vs service-managed perms)
  - SSM Parameter Store + State Manager; Service Catalog products; AWS Proton templates
### Task 2.2 Automate account creation, onboarding, and security
- Account factory
  - Organizations + Control Tower Account Factory (or AFT/Terraform)
  - Standardized baselines via CloudFormation StackSets on new accounts
  - IAM Identity Center permission sets for multi-account access
  - Config rules + SCPs as preventive/detective guardrails
### Task 2.3 Build automated solutions for complex/large-scale tasks
- Operational automation
  - SSM Inventory, Patch Manager baselines, State Manager associations
  - Lambda automations (boto3/SDK custom logic, EventBridge triggers)
  - SSM Automation runbooks for desired-state; Config compliance packs

## Resilient Cloud Solutions (15%)
### Task 3.1 Implement highly available solutions
- High availability
  - Multi-AZ subnets behind ALB/NLB; RDS Multi-AZ (sync standby)
  - Replication/failover: Aurora replicas, ElastiCache replication groups
  - Remove SPOF: ≥2 AZs, ASG min across AZs
  - Cross-Region: DynamoDB global tables, RDS cross-Region read replica, S3 CRR
  - ELB health checks deregister unhealthy targets
### Task 3.2 Implement scalable solutions
- Scalability
  - Auto Scaling target tracking / step; ALB request-count scaling
  - ElastiCache / DAX caching; SQS to decouple producers/consumers
  - Containers: ECS (Fargate/EC2), EKS, Fargate capacity providers
  - Serverless: API Gateway + Lambda + Step Functions
  - Multi-Region via Route 53 + Global Accelerator
### Task 3.3 Implement automated recovery (RTO/RPO)
- Disaster recovery
  - RTO (downtime) / RPO (data loss) targets
  - AWS Backup plans; pilot light, warm standby
  - Failover tests: RDS/Aurora `failover-db-cluster`, Route 53 health-check failover
  - Cross-Region backup copy; ASG replaces failed instances behind ELB

## Monitoring and Logging (15%)
### Task 4.1 Configure collection, aggregation, storage of logs/metrics
- Telemetry pipeline
  - CloudWatch metrics: namespaces, dimensions, standard (1min) vs high-res (1s)
  - CloudWatch agent (`amazon-cloudwatch-agent.json`) for mem/disk/custom metrics
  - Metric filters from log groups → custom metrics
  - Metric streams → Kinesis Data Firehose → S3 / partner
  - Log group retention setting; subscription filters → Kinesis/Lambda; KMS encryption
### Task 4.2 Audit, monitor, analyze logs and metrics
- Detection and analysis
  - CloudWatch anomaly detection alarms (band model)
  - Amazon Inspector (CVE), AWS Config rules, CloudTrail event history/Lake
  - CloudWatch dashboards + QuickSight visualizations
  - X-Ray tracing (ECS sidecar, API Gateway, Lambda active tracing)
  - Analyze: Athena on S3 logs, CloudWatch Logs Insights queries, Kinesis Data Streams
### Task 4.3 Automate monitoring and event management
- Event-driven ops
  - Auto scaling: DynamoDB, EC2 ASG, RDS storage autoscaling
  - CloudWatch alarm → SNS; EventBridge rules with event patterns + targets
  - S3 event notifications → Lambda (`s3:ObjectCreated:*`)
  - SSM Agent install (`AmazonSSMManagedInstanceCore` role); Config auto-remediation
  - Health checks: Route 53, ALB target groups

## Incident and Event Response (14%)
### Task 5.1 Manage event sources to process, notify, act
- Event ingestion
  - AWS Health events, EventBridge, CloudTrail as sources
  - Patterns: SNS fan-out, Kinesis streaming, SQS queuing
  - Workflows: SQS + SNS + Step Functions + Lambda
### Task 5.2 Implement configuration changes in response to events
- Automated remediation
  - SSM Automation + AWS Auto Scaling fleet management
  - AWS Config rules + SSM remediation documents
  - EventBridge rule → Lambda/SSM to fix drift / undesired state
### Task 5.3 Troubleshoot system and application failures
- Root cause analysis
  - CloudWatch metrics/alarms + X-Ray traces/service map
  - AWS Health Dashboard; SSM service health, Session Manager for shell
  - Debug failed CodePipeline/CodeBuild/CodeDeploy (logs, `appspec` hooks)
  - ASG activity history, ECS stopped-task `stoppedReason`, container exit codes

## Security and Compliance (17%)
### Task 6.1 Identity and access management at scale
- IAM at scale
  - IAM roles (machine) vs users/Identity Center (human)
  - Federation: SAML/OIDC IdPs, IAM Identity Center
  - Permissions boundaries; Organizations SCPs ceiling
  - Least privilege, RBAC + ABAC (`aws:PrincipalTag`)
  - Secrets Manager rotation (Lambda); IAM Access Analyzer
### Task 6.2 Automate security controls and data protection
- Defense in depth automation
  - Security groups, NACLs, AWS Network Firewall (Suricata rules)
  - ACM certs (auto-renew); KMS keys + rotation + grants
  - Data classification; SSE-KMS at rest, TLS in transit
  - Amazon Macie sensitive-data discovery in S3
  - Firewall Manager org-wide WAF/SG policies
### Task 6.3 Security monitoring and auditing
- Audit and threat detection
  - CloudTrail (org trail), AWS Config, VPC Flow Logs
  - GuardDuty, Inspector, Security Hub (CIS/AWS FSBP standards), Detective
  - Threats: insecure traffic, exposed access keys, public S3 buckets
  - EventBridge on GuardDuty findings → SNS/Lambda auto-response

## Key Services
### Developer tools
- CodePipeline, CodeBuild, CodeDeploy, CodeArtifact, CodeCommit, CodeGuru
- CDK, SAM, CloudFormation, Proton, Service Catalog, EC2 Image Builder
### Operations and security
- Systems Manager (Automation/State Manager/Patch/Session Manager), Config, Control Tower, Organizations, FIS
- CloudWatch, CloudTrail, X-Ray, EventBridge, AWS Health, Resilience Hub
- IAM, IAM Identity Center, KMS, Secrets Manager, GuardDuty, Inspector, Security Hub, Macie
### Exam essentials
- 75 questions; scaled 100–1000; pass = 750; 180 minutes
- Compensatory scoring; multiple choice + multiple response
