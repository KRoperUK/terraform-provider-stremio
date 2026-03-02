# Security Policy

## Supported Versions

This project is currently in an early release stage.

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |
| older   | :x:                |

## Reporting a Vulnerability

Please do **not** open public GitHub issues for security vulnerabilities.

Report vulnerabilities by using GitHub's private vulnerability reporting feature:

- Go to the **Security** tab of this repository
- Click **Report a vulnerability**
- Include steps to reproduce, impact, and any proof-of-concept details

If private reporting is not available, contact the maintainer directly through GitHub and mark the message as security-sensitive.

## What to Include in a Report

- A clear description of the issue and impact
- Affected version/commit
- Reproduction steps
- Suggested remediation (if known)

## Response Expectations

- Initial triage target: within 7 days
- Follow-up updates provided as triage progresses
- Fixes are released as soon as reasonably possible based on severity

## Security Considerations for Users

- Never commit `.env`, `*.tfvars`, or `*.tfstate` files containing credentials.
- Rotate credentials immediately if they are exposed.
- Use least-privilege API credentials for third-party add-ons.
- Review provider and module diffs before every `terraform apply`.
