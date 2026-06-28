---
title: Linux Foundation Certified IT Associate (LFCA)
markmap:
  colorFreezeLevel: 2
---

# LFCA

## System Administration Fundamentals (30%)
### System Administration
- User and permission basics
  - Account types
    - root (UID 0) vs standard user
    - service/system accounts (UID < 1000)
  - Permission model
    - rwx for owner/group/other (e.g. `-rwxr-xr--`)
    - octal notation `chmod 644 file`, `chmod 755 dir`
    - ownership `chown bob:devs file`
  - Privilege escalation
    - `sudo command`
    - "/etc/sudoers" via `visudo`
- Filesystem and processes
  - FHS directories
    - "/etc" config, "/var" variable data, "/home" users, "/usr" programs
    - "/proc" kernel/process pseudo-fs
  - Process management
    - `ps aux`, `top`
    - `kill -9 <pid>`, `pgrep`
- Package and service management
  - Packages
    - apt: `apt install nginx`
    - dnf/yum: `dnf install httpd`
  - Services
    - `systemctl start|status nginx`
    - `systemctl enable nginx` at boot
  - Logging
    - "/var/log/syslog", "/var/log/messages"
    - `journalctl -u sshd`
### Best Practices
- Documentation
  - Runbooks and standard operating procedures (SOPs)
  - Inline comments and "README.md"
- Automation
  - Shell scripts (`#!/bin/bash`)
  - Scheduled jobs `crontab -e`
- Change/config management
  - Version control of configs (Git)
  - Idempotent config tools (Ansible playbooks)
- Reliability
  - Backups (`rsync`, `tar`), redundancy (RAID)
  - Monitoring agents (Prometheus node_exporter)
### Networking
- Core concepts
  - Addressing
    - IPv4 `192.168.1.10`, IPv6 `2001:db8::1`
    - subnet mask / CIDR `/24` = 255.255.255.0
    - private ranges 10.0.0.0/8, 192.168.0.0/16
  - Services
    - DNS (port 53), DHCP (ports 67/68)
    - default gateway, well-known ports (HTTP 80, HTTPS 443, SSH 22)
  - Models
    - OSI 7 layers, TCP/IP 4 layers
- Tools and protocols
  - Diagnostics
    - `ping host`, `traceroute host`
    - `ip addr`, `ss -tulpn`
  - Remote access
    - `ssh user@host`
    - HTTP/HTTPS, FTP/SFTP, TLS handshake
### Troubleshooting
- Methodology
  - Identify, hypothesize, test, resolve, document
  - Reproduce and isolate scope
- Log analysis
  - `journalctl -xe`, `tail -f /var/log/syslog`
  - `grep -i error /var/log/...`
- Resource diagnosis
  - CPU/mem `top`, `free -h`
  - disk `df -h`, network `ss`, `ping`
### Disaster Recovery
- Backup types
  - full, incremental, differential
  - tools `tar -czvf backup.tgz /data`, `rsync -avz`
- Objectives
  - RTO (recovery time objective)
  - RPO (recovery point objective)
- High availability
  - failover, clustering, redundancy (N+1)
- Validation
  - periodic restore drills
  - integrity checks (checksums `sha256sum`)

## Cloud Computing Fundamentals (18%)
### Cloud Computing
- Service models
  - IaaS (EC2, VMs)
  - PaaS (App Engine, managed runtimes)
  - SaaS (Gmail, Microsoft 365)
- Deployment models
  - public, private, hybrid, community
- Workload units
  - virtual machines (hypervisor-based)
  - containers (`docker run`, shared kernel)
- Providers
  - AWS, Azure, GCP core concepts
### Performance/Availability
- Scaling
  - vertical (bigger instance)
  - horizontal (more instances)
- Elasticity
  - auto-scaling groups, scaling policies
- Distribution
  - load balancers (round-robin, least-conn)
- Reliability metrics
  - SLA uptime (99.9% = "three nines")
  - regions and availability zones (AZs)
### Budgeting
- Pricing
  - pay-as-you-go / consumption-based
  - reserved/spot instances
- Accounting
  - CapEx vs OpEx
- Optimization
  - right-sizing instances
  - cost dashboards and budget alerts
### Best Practices
- Shared responsibility model (provider vs customer)
- Resource organization
  - tagging (`Environment=prod`)
  - naming conventions
- Infrastructure as Code
  - declarative templates (Terraform `.tf`, CloudFormation)
### Networking
- Virtual networks
  - VPC, subnets, route tables
  - security groups, NACLs
- Connectivity
  - site-to-site VPN, peering
- Edge
  - CDN caching, edge locations

## Linux Fundamentals (16%)
### Linux Operating System
- Open source / distros
  - GPL-licensed kernel
  - families: Debian/Ubuntu, RHEL/Fedora, SUSE
- Architecture
  - kernel, shell (bash), userspace
- Filesystem hierarchy (FHS)
  - "/bin", "/etc", "/var", "/home", "/usr", "/tmp"
- File types and links
  - regular, directory, device, socket
  - hard link `ln a b`, symlink `ln -s target link`
### Command Line
- Navigation
  - `cd /path`, `ls -la`, `pwd`
- File operations
  - `cp src dst`, `mv old new`, `rm -i file`
  - `mkdir -p dir/sub`, `touch file`
- Viewing files
  - `cat file`, `less file`
  - `head -n 20 file`, `tail -f log`
- Text processing
  - `grep -r "text" .`
  - `sed 's/old/new/g' file`
  - `awk '{print $1}' file`
  - `cut -d: -f1 /etc/passwd`, `sort | uniq -c`
- Pipes and redirection
  - pipe `cmd1 | cmd2`
  - `> file` overwrite, `>> file` append
  - `2> err.log`, `< input`
- Help
  - `man ls`, `ls --help`
  - `apropos network`

## Security Fundamentals (14%)
### Security
- AAA
  - authentication (who), authorization (what), accounting (logging)
- Least privilege
  - minimal rights, `sudo` per-command grants
- Encryption
  - symmetric (AES) vs asymmetric (RSA, ECC)
  - at rest (LUKS, disk encryption) vs in transit (TLS)
- PKI / certificates
  - CA, X.509 certs, public/private key pairs
  - `openssl x509 -text -noout`
- Network defenses
  - firewalls (`firewall-cmd`, `ufw`)
  - VPN tunnels, MFA / TOTP
### Sensitive Data
- Classification
  - public, internal, confidential, restricted
- PII handling
  - names, IDs, financial data
- Secrets management
  - vaults (HashiCorp Vault), avoid plaintext in code
  - file perms `chmod 600 secret.key`
- Access controls
  - RBAC, ACLs (`setfacl -m u:bob:r file`)
### Compliance
- Regulations
  - GDPR (EU privacy), HIPAA (health), PCI-DSS (payment cards)
- Audit / logging
  - `auditd`, "/var/log/audit/audit.log"
  - log retention policies
- Governance
  - security policies, acceptable use, periodic reviews

## DevOps Fundamentals (12%)
### DevOps Basics
- Culture
  - Dev + Ops collaboration, blameless postmortems
- CI/CD
  - CI (build/test on push), CD (automated deploy)
  - pipelines (GitHub Actions `.github/workflows/`, GitLab CI `.gitlab-ci.yml`)
- IaC
  - declarative provisioning (Terraform, Ansible)
- Feedback
  - monitoring (Prometheus), dashboards (Grafana), alerting
### Git Concepts
- Objects
  - repository, commit, branch, tag
- Core commands
  - `git clone`, `git add`, `git commit -m`
  - `git push`, `git pull`
- Collaboration
  - `git merge`, pull/merge requests
  - conflict resolution
- Model
  - distributed VCS, every clone is full history
### Containers
- Container vs VM
  - shared kernel vs full guest OS
- Building blocks
  - images, containers, registries (Docker Hub)
  - "Dockerfile" build instructions
- Tooling
  - `docker build -t app .`, `docker run app`
  - orchestration: Kubernetes (Pods, Deployments)
- Properties
  - portability, immutability, ephemeral storage

## IT Project Management Fundamentals (10%)
### Project Management
- Methodologies
  - Agile, Scrum, Kanban, Waterfall
- Scrum artifacts
  - sprints, product/sprint backlog, daily stand-up
  - roles: Product Owner, Scrum Master
- Planning
  - scope, timeline (Gantt), resource allocation
### Software Application Architecture
- Styles
  - monolithic vs microservices
  - client-server, three-tier (presentation/logic/data)
- Integration
  - REST APIs, JSON payloads
  - message queues
### Functional Analysis
- Requirements gathering
  - interviews, user stories
- Requirement types
  - functional (what it does)
  - non-functional (performance, security, scalability)
- Communication
  - stakeholder alignment, sign-off
### Open Source Software and Licensing
- Permissive licenses
  - MIT, Apache 2.0, BSD
- Copyleft licenses
  - GPL, LGPL, AGPL (share-alike)
- Compliance
  - license compatibility matrix
  - attribution and notice files ("LICENSE", "NOTICE")
- Community model
  - upstream contribution, pull requests, maintainers
