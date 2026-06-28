---
title: FINOS Financial Services Certified Open Source Developer (FSOSD)
markmap:
  colorFreezeLevel: 2
---

<!--
  ACRONYM: FSOSD = "FINOS Financial Services Certified Open Source Developer",
  a Linux Foundation / FINOS (Fintech Open Source Foundation) certification.
  Domains and weights below are the official exam blueprint.
  Source: training.linuxfoundation.org/certification/finos-open-source-developer-fsosd/
-->

# FSOSD

## Ethics and Behavior (10%)
### Open source community engagement
- Understanding escalation paths
  - Project maintainers -> TSC (Technical Steering Committee)
  - FINOS Community Manager / mailing list
  - Code of Conduct committee reporting
- Engaging with open source communities
  - GitHub issues / pull requests (PRs)
  - Mailing lists, Slack, public meetings
  - Good first issue / triage labels
- Codes of conduct and professional behavior
  - Contributor Covenant Code of Conduct
  - CNCF / Linux Foundation CoC adoption

## Open Source Licensing (18%)
### License obligations and IP
- Comply with open source license obligations
  - Permissive: Apache-2.0, MIT, BSD-3-Clause
  - Copyleft: GPL-3.0, LGPL-3.0, AGPL-3.0
  - Apache-2.0 vs GPL incompatibility nuance
  - NOTICE file + attribution retention
- Understanding implications of unlicensed software
  - "No license" = all rights reserved (not OSS)
  - SPDX-License-Identifier header requirement
- Understanding copyrights and licenses
  - SPDX license list and identifiers
  - OSI-approved license definition
  - Patent grant clause (Apache-2.0 sect.3)
- Contributor License Agreements (CLA)
  - Individual CLA (ICLA) vs Corporate CLA (CCLA)
  - EasyCLA (Linux Foundation/FINOS tooling)
  - Developer Certificate of Origin (DCO)
    - git commit -s "Signed-off-by:" trailer
    - DCO bot check on PRs

## Consuming Open Source (26%)
### Using third-party / open source code safely
- Understanding the software supply chain
  - SBOM formats: SPDX, CycloneDX
  - Provenance / SLSA framework
  - Transitive dependency graph
- Evaluate and maintain code dependencies
  - Dependency manifests: package.json, pom.xml, requirements.txt, go.mod
  - Pinning / lockfiles (package-lock.json, poetry.lock)
  - Renovate / Dependabot updates
- Identify software vulnerabilities
  - CVE / CVSS scoring
  - SCA tools: Snyk, OWASP Dependency-Check, Trivy
  - GitHub Advisory Database / OSV
- Managing third party applications and code
  - Artifact repositories: Artifactory, Nexus
  - Internal mirror / approved registry
- Vulnerability monitoring and maintenance plans
  - Continuous scanning in CI (GitHub Actions)
  - Patch SLAs by CVSS severity
- Approval processes for using open source software
  - Inbound license review / allowlist
  - OSPO intake request
- Evaluate codebase risk
  - OpenSSF Scorecard
  - Maintainer activity / bus factor
  - License compliance scan (FOSSA, ScanCode)

## Contributing to Open Source (28%)
### Contributing from a financial institution
- Risks of contributing
  - Data leakage risk
    - Secrets in commits (API keys, credentials)
    - git-secrets / gitleaks pre-commit scanning
  - Dependency risk
    - Introducing vulnerable transitive deps
  - Operational risk
    - Fork maintenance burden / upstream divergence
- Benefits of contributing to open source projects
  - Upstreaming patches reduces maintenance fork
  - Talent attraction / community goodwill
- Ownership of copyright and IP implications
  - Work-for-hire / employer copyright assignment
  - DCO sign-off attesting right to contribute
- Importance of contribution approval processes
  - Internal contribution request workflow
  - Manager + legal + OSPO sign-off
- Publication review processes
  - Pre-publication legal/compliance review
  - Outbound license selection (Apache-2.0 default at FINOS)
- Firm projects vs. personal projects vs. open source projects
  - Use of corporate email/identity in commits
  - Separating personal GitHub from firm work
- Role of an OSPO (Open Source Program Office)
  - Policy ownership and license allowlists
  - FINOS as financial-services foundation
  - FINOS projects: Morphir, Legend, Waltz, Perspective

## Regulatory Impact on Open Source (18%)
### Financial-sector regulatory considerations
- Regulations around communication surveillance
  - SEC Rule 17a-4 / FINRA record retention
  - MiFID II communication recording (EU)
  - GitHub/Slack as monitored channels
- Social media policies
  - Public posting / endorsement restrictions
  - Personal vs firm-affiliated statements
- Compliance processes around open source contribution
  - Sarbanes-Oxley (SOX) controls on code changes
  - Audit trail via signed commits + PR approvals
- IP regulations around data within a bank
  - GDPR / data residency for shared data
  - No PII / confidential data in public repos
  - GLBA (Gramm-Leach-Bliley) data safeguards
