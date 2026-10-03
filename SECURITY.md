# Security policy

## Supported versions
Only the latest release of rabbitrun gets fixes, including security fixes. If you hit an issue on an older version, please reproduce it on the latest release first.

## Reporting a vulnerability
Report security issues privately, not through public issues or pull requests.

- Email: n.chika156@gmail.com
- Or use the "Report a vulnerability" button on the repository's Security tab.

rabbitrun is an offline game. It reads only its embedded assets and writes only its save file (under the user config directory) and, when asked with `--capture` / `--bgm-wav`, files into the directory you pass. Reports about file handling, the save file parser, or anything that makes the game touch files it should not are especially useful. Please include:

- rabbitrun version (the release tag or commit)
- OS and architecture
- What you did and what happened
- A minimal reproduction, if you have one

## What to expect
rabbitrun is maintained by one developer in spare time, so there is no guaranteed response time. I will acknowledge the report, confirm the issue, and fix it in a new release. You will be credited in the release notes unless you prefer to stay anonymous.

## Verifying releases
Release artifacts are signed with cosign and ship with an SBOM and build provenance. See [Verifying release integrity](./README.md#verifying-release-integrity).
