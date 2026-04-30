#!/usr/bin/env bash
set -euo pipefail

TAG="${1:-v1.37.2}"
REPO="envoyproxy/envoy"
ARCHIVE_URL="https://github.com/${REPO}/archive/refs/tags/${TAG}.tar.gz"
DEST="$(cd "$(dirname "$0")" && pwd)"
TMP=$(mktemp -d)

cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT

echo "Downloading envoy proto @ ${TAG} ..."
curl -fsSL "$ARCHIVE_URL" | tar -xz -C "$TMP"

EXTRACTED="$TMP/envoy-${TAG#v}/api"
if [[ ! -d "$EXTRACTED" ]]; then
  echo "error: expected directory not found: $EXTRACTED" >&2
  exit 1
fi

# Remove previously downloaded protos, keep this script
find "$DEST" -mindepth 1 -not -name "$(basename "$0")" -delete

cp -r "$EXTRACTED"/envoy        "$DEST/envoy"
cp -r "$EXTRACTED"/contrib      "$DEST/contrib"

# Remove Bazel BUILD files and any directories left empty by their removal
find "$DEST" -name "BUILD" -delete
find "$DEST" -mindepth 1 -type d -empty -delete

echo "Done. Proto files written to $DEST"
echo "  envoy/    — core config types (Listener, Cluster, Route, ...)"
echo "  contrib/  — contrib/extension types"
