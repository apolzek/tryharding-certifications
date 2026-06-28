---
title: Certified Open Source Developer for Enterprise (CODE)
markmap:
  colorFreezeLevel: 2
---

<!--
NOTE ON CERT IDENTIFICATION:
The folder hint suggested "CODE" might be a CNCF observability cert. After research it is
NOT a CNCF certification. "CODE" is a Linux Foundation certification:
"Certified Open Source Developer for Enterprise (CODE)", developed with the TODO Group and OSI.
Source (official, domains + weights): https://training.linuxfoundation.org/certification/certified-open-source-developer-for-enterprise-code/
Confidence: HIGH (official LF exam page confirms acronym, full name, and exact domain weights).
-->

# CODE

## Fundamentals of Open Source Software Development (24%)
### Open Source Principles
- Principles of open source software
  - OSI Open Source Definition (10 criteria)
  - Four freedoms (FSF: use, study, share, improve)
  - Source availability vs OSI-approved license
- Open source communities
  - Roles: maintainers, contributors, users
    - Maintainer = merge/release rights
    - Triager / reviewer / committer ladder
  - Community norms & governance
    - CODE_OF_CONDUCT.md (Contributor Covenant)
    - GOVERNANCE.md, BDFL vs meritocracy
    - CONTRIBUTING.md guidelines
### Collaboration Workflows
- Issues & pull requests
  - fork → branch → pull request
  - `git rebase -i` to clean history
  - Issue templates / labels / milestones
- Code reviews
  - Review etiquette and feedback
    - Conventional Comments, request changes vs approve
  - CODEOWNERS auto-review assignment
  - CI checks required before merge (status checks)
### Release Practices
- Release management
  - Git tags + annotated release notes
  - CHANGELOG.md (Keep a Changelog)
  - Release branches / LTS vs rolling
- Semantic versioning (SemVer)
  - MAJOR.MINOR.PATCH
  - Breaking change → MAJOR bump
  - Pre-release tags (`1.2.0-rc.1`)

## Open Source Licensing and Usage Guidelines (14%)
### Licensing & Legal
- Intellectual property basics
  - Copyright vs patent vs trademark
- Software licensing & open source legalities
  - SPDX license identifier (e.g. `Apache-2.0`)
  - LICENSE file in repo root
- Open source & copyleft license compliance
  - Permissive vs copyleft
    - Permissive: MIT, BSD-3-Clause, Apache-2.0
    - Weak copyleft: LGPL, MPL-2.0
    - Strong copyleft: GPL-3.0, AGPL-3.0
  - License compatibility matrix
  - Attribution / NOTICE file requirements
### Compliance & Risk
- Risk assessments
  - License scanning (FOSSA / ScanCode / OSS Review Toolkit)
- Export control regulations & compliance
  - US EAR / ECCN classification
  - Encryption export notices
- Patent grants
  - Apache-2.0 §3 patent grant / termination
### Contributor Agreements
- Contributor License Agreements (CLA)
  - Individual vs Corporate CLA
  - CLA bot enforcement
- Developer Certificate of Origin (DCO)
  - `Signed-off-by:` trailer (`git commit -s`)
  - CLA vs DCO trade-offs

## Consuming Open Source Software (28%)
### Supply Chain & Dependencies
- The software supply chain
  - Upstream → package registry → build → deploy
  - SLSA provenance levels
- Code dependencies
  - Transitive dependency risk
    - Dependency tree (`npm ls`, `go mod graph`)
    - Lockfiles (`package-lock.json`, `go.sum`)
  - Pinning vs version ranges
- Software Bill of Materials (SBOM)
  - SPDX / CycloneDX formats
  - Generate with Syft / `cdxgen`
### Risk & Maintenance
- Codebase risk
  - CVE / OSV vulnerability IDs
  - CVSS severity score
  - OpenSSF Scorecard, project health/bus factor
- Software maintenance plans
  - EOL / deprecation policy
  - Dependabot / Renovate auto-updates
### Enterprise Process
- Open source software approval process
  - Intake & vetting workflows
    - License + security review gate
    - Approved-components allowlist
  - OSPO intake ticket / SPDX manifest

## Contributing to Open Source (22%)
### Contribution Strategy
- Contribution strategy
  - Upstream-first policy
  - "Good first issue" onboarding
- Project types: business, personal, open source
  - Inner source vs public OSS
### Best Practices
- Code and documentation best practices
  - Conventional Commits messages
  - Tests + CI green before PR
  - README / API docs / docstrings
- Contribution approval processes
  - Maintainer review + required approvals
  - DCO sign-off / CLA check passing
### Legal & Risk
- Copyright ownership and intellectual property
  - Employer IP assignment / work-for-hire
  - SPDX file headers, copyright notice
- Contribution risks
  - Leaking proprietary code / secrets
  - Incompatible license contamination

## Open Source Management Operations (12%)
### Upstream Engagement
- Contributing to upstream projects
  - Submit patch upstream vs maintain fork
  - Backport fixes downstream
- Developer support
  - Mailing lists / Discourse / Slack
  - Issue triage SLAs
### Organizational Roles
- Open source management roles
  - OSPO (Open Source Program Office)
    - TODO Group OSPO practices
    - Policy, compliance, advocacy functions
- Escalation paths
  - Maintainer → OSPO → legal review
  - Security disclosure (SECURITY.md, embargo)
