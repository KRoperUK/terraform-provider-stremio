#!/usr/bin/env python3
"""Filter govulncheck JSON output.

Fails (exit 1) only on *called* vulnerabilities that live in our dependencies.
Standard-library findings are reported as advisories but do not fail the build:
their fix ships with a Go toolchain release that may not be available yet, so
gating on them produces false CI failures. Dependency vulnerabilities, by
contrast, are actionable here (bump the module) and are watched by Dependabot.

Usage: govulncheck-filter.py <govulncheck-json-file>
"""
import json
import sys


def load_findings(path):
    with open(path) as handle:
        text = handle.read()
    decoder = json.JSONDecoder()
    findings = []
    index, length = 0, len(text)
    while index < length:
        while index < length and text[index] in " \t\r\n":
            index += 1
        if index >= length:
            break
        obj, index = decoder.raw_decode(text, index)
        if "finding" in obj:
            findings.append(obj["finding"])
    return findings


def is_called(finding):
    trace = finding.get("trace") or []
    return bool(trace and trace[0].get("function"))


def main():
    findings = load_findings(sys.argv[1])
    called = [f for f in findings if is_called(f)]

    stdlib, deps = set(), set()
    for finding in called:
        module = (finding.get("trace") or [{}])[0].get("module", "")
        osv = finding.get("osv", "unknown")
        (stdlib if module == "stdlib" else deps).add(osv)

    if stdlib:
        print(
            "::warning title=govulncheck::Advisory standard-library "
            "vulnerabilities (fixed in a future Go release): "
            + ", ".join(sorted(stdlib))
        )

    if deps:
        print(
            "::error title=govulncheck::Dependency vulnerabilities found: "
            + ", ".join(sorted(deps))
        )
        return 1

    print(
        "govulncheck: no called dependency vulnerabilities. "
        "Stdlib advisories: " + (", ".join(sorted(stdlib)) or "none")
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
