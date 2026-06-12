# Security Policy

## Supported Versions

Only the latest **stable release** receives security updates. Pre-release versions (`-alpha`, `-beta`, `-rc`) are not covered.

| Version         | Supported          |
|-----------------|--------------------|
| Latest release  | ✅ Supported       |
| Older releases  | ❌ Not supported   |
| Pre-release     | ❌ Not supported   |

## Reporting a Vulnerability

We take security vulnerabilities seriously. Please do **not** report them via public GitHub issues.

### How to Report

1. **Open a draft security advisory** on GitHub:
   - Go to the [Security Advisories](https://github.com/manudiv16/scheduled-db/security/advisories) page.
   - Click **"New draft security advisory"**.
   - Fill in the details — provide as much context as possible (affected versions, reproduction steps, impact).

2. If you cannot use GitHub Advisories, email the project maintainer directly. You can find the contact information in the commit history or through the GitHub profile.

### What to Include

- **Type** of vulnerability (e.g., RCE, denial of service, information disclosure)
- **Affected version(s)** and commit hash if known
- **Steps to reproduce** — minimal, self-contained example preferred
- **Impact** — what an attacker could achieve
- **Suggested fix** (optional but appreciated)

### Response Timeline

| Timeframe       | Action                                     |
|-----------------|--------------------------------------------|
| 48 hours        | Acknowledgment of receipt                  |
| 5 business days | Preliminary assessment and confirmation    |
| 14 days         | Fix development in private fork            |
| TBD             | Coordinated release and public disclosure  |

We strive to release a fix within **14 days** of confirmation, depending on severity and complexity.

## Disclosure Policy

We follow **coordinated disclosure**:

1. The reporter and maintainers communicate privately during the fix process.
2. A fix is prepared and a new release is cut.
3. The vulnerability is publicly disclosed in the release notes once the fix is available.

## Recognition

We credit reporters who follow this policy in our release notes, unless they prefer to remain anonymous.

## Security-Relevant Configuration

### Production Deployments

- Always run in a **clustered configuration** (minimum 3 nodes) with **TLS between nodes**.
- Set `--health-failure-threshold` appropriately (default 0.1).
- Enable **gossip encryption** in multi-tenant networks.
- Use the **split-brain detection** (`--raft-advertise-host` + `CLUSTER_SIZE` in K8s) to prevent data inconsistencies.
- Configure **execution timeouts** (`--execution-timeout`, `--inprogress-timeout`) to prevent runaway jobs.

---

Thank you for helping keep Scheduled-DB and its users safe.
