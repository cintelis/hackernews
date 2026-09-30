#!/bin/sh
# Sign and publish a release. The release workflow builds each tag into a
# DRAFT; nothing is installable until this signs it, with a key that never
# leaves the maintainer's machine:
#
#   scripts/sign-release.sh v1.2.3 [path/to/private/key]
#
# It checks the draft first — every archive was built by this repo's release
# workflow (build attestation) and matches checksums.txt — then signs
# checksums.txt, verifies the signature against the key committed in
# internal/update/release_key.pub, uploads it and publishes the release.
# Needs: gh (logged in), ssh-keygen (OpenSSH 8.1+), sha256sum or shasum.
set -eu

REPO="cintelis/hackernews"
NS="cintelis-release"
tag="${1:?usage: scripts/sign-release.sh vX.Y.Z [private key]}"
key="${2:-$HOME/.ssh/cintelis-release}"
root=$(cd "$(dirname "$0")/.." && pwd)
pub="$root/internal/update/release_key.pub"

[ -f "$key" ] || { echo "no private key at $key" >&2; exit 1; }
if [ "$(gh release view "$tag" -R "$REPO" --json isDraft --jq .isDraft)" != "true" ]; then
  echo "$tag is not a draft release (already published, or not built yet)" >&2
  exit 1
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
gh release download "$tag" -R "$REPO" -D "$tmp"

echo "checking build provenance ..."
for f in "$tmp"/*.tar.gz "$tmp"/*.zip; do
  gh attestation verify "$f" -R "$REPO" >/dev/null
done

echo "checking checksums ..."
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$tmp" && sha256sum -c --quiet checksums.txt)
else
  (cd "$tmp" && shasum -a 256 -c --quiet checksums.txt)
fi

echo "signing checksums.txt ..."
rm -f "$tmp/checksums.txt.sig"
ssh-keygen -Y sign -f "$key" -n "$NS" "$tmp/checksums.txt"

# the signature must verify with the key the app and installers carry
printf '%s namespaces="%s" %s\n' "$NS" "$NS" "$(cut -d' ' -f1,2 "$pub")" > "$tmp/allowed_signers"
ssh-keygen -Y verify -f "$tmp/allowed_signers" -I "$NS" -n "$NS" \
  -s "$tmp/checksums.txt.sig" < "$tmp/checksums.txt"

gh release upload "$tag" "$tmp/checksums.txt.sig" -R "$REPO"
gh release edit "$tag" -R "$REPO" --draft=false --latest
echo "published $tag"
