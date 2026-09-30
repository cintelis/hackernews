# Security

## Reporting a vulnerability

Please report privately through GitHub:
**[Report a vulnerability](https://github.com/cintelis/hackernews/security/advisories/new)**.
Don't open a public issue for security problems.

## How releases are protected

- **Signed.** Every release's `checksums.txt` is signed with the cintelis
  release key, an Ed25519 key kept offline by the maintainer and never
  stored on GitHub. `cintelis update`, `install.sh` and `install.ps1` refuse
  any release whose signature doesn't verify, then check the archive against
  the signed checksums.
- **Built by CI, with provenance.** Release archives are built by this
  repository's release workflow and carry a signed build-provenance
  attestation. The workflow produces a *draft*; the maintainer checks the
  attestation and checksums before signing and publishing it
  (`scripts/sign-release.sh`).
- **Immutable once published.** Published releases and their tags can't be
  changed or replaced.
- **Pinned dependencies.** Go modules are pinned by hash (`go.sum`, checked
  against the Go checksum database); GitHub Actions are pinned to commits and
  restricted to GitHub's own plus GoReleaser's. CI runs `govulncheck` on
  every change and weekly, and Dependabot tracks updates.

## Verifying a release yourself

The release public key:

```
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOCB3IaMc3Lc4JvPv6rWCVLpTjmvvPrhFFPST0NSsypP cintelis-release
```

Fingerprint: `SHA256:RY9yd61LBZCa5WrzSkEet+1Dnt2zVu9zpMRbZMWIV+I`

It is also published on our domain, so you can cross-check it through a
second channel: <https://cintelis.ai/.well-known/cintelis-release.pub>

```sh
# 1. the checksums are signed by the release key
echo 'cintelis-release namespaces="cintelis-release" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOCB3IaMc3Lc4JvPv6rWCVLpTjmvvPrhFFPST0NSsypP' > allowed_signers
ssh-keygen -Y verify -f allowed_signers -I cintelis-release -n cintelis-release \
  -s checksums.txt.sig < checksums.txt

# 2. your archive matches the signed checksums
sha256sum --ignore-missing -c checksums.txt

# 3. optionally, it was built by this repository's release workflow
gh attestation verify cintelis_<version>_<os>_<arch>.tar.gz --repo cintelis/hackernews
```
