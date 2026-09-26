#!/usr/bin/env bash
# Watchdog Deterministic Build Reproducibility Verification Script
# Verifies that independent clean builds with -trimpath and CGO_ENABLED=0 produce identical byte-for-byte binaries.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

BUILD_DIR_1="$(mktemp -d /tmp/watchdog-repro-1.XXXXXX)"
BUILD_DIR_2="$(mktemp -d /tmp/watchdog-repro-2.XXXXXX)"

cleanup() {
    rm -rf "${BUILD_DIR_1}" "${BUILD_DIR_2}"
}
trap cleanup EXIT

echo "========================================================"
echo " Watchdog Build Reproducibility Verification"
echo "========================================================"
echo "Project Root: ${ROOT_DIR}"
echo "Build Dir 1:  ${BUILD_DIR_1}"
echo "Build Dir 2:  ${BUILD_DIR_2}"

# Fixed metadata for reproducibility verification
FIXED_VERSION="1.0.0-repro"
FIXED_COMMIT="0000000000000000000000000000000000000000"
FIXED_DATE="2026-09-26T00:00:00Z"
FIXED_BUILT_BY="repro-verifier"

LDFLAGS="-s -w -X github.com/DocHoax/watchdog/cmd.Version=${FIXED_VERSION} -X github.com/DocHoax/watchdog/cmd.GitCommit=${FIXED_COMMIT} -X github.com/DocHoax/watchdog/cmd.BuildDate=${FIXED_DATE} -X github.com/DocHoax/watchdog/cmd.BuiltBy=${FIXED_BUILT_BY}"

echo "Building Run 1 (CGO_ENABLED=0, -trimpath)..."
(
    cd "${ROOT_DIR}"
    CGO_ENABLED=0 go build -trimpath -ldflags="${LDFLAGS}" -o "${BUILD_DIR_1}/watchdog" .
)

echo "Building Run 2 (CGO_ENABLED=0, -trimpath)..."
(
    cd "${ROOT_DIR}"
    CGO_ENABLED=0 go build -trimpath -ldflags="${LDFLAGS}" -o "${BUILD_DIR_2}/watchdog" .
)

echo "Calculating SHA-256 Checksums..."
if command -v sha256sum >/dev/null 2>&1; then
    HASH_1=$(sha256sum "${BUILD_DIR_1}/watchdog" | awk '{print $1}')
    HASH_2=$(sha256sum "${BUILD_DIR_2}/watchdog" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
    HASH_1=$(shasum -a 256 "${BUILD_DIR_1}/watchdog" | awk '{print $1}')
    HASH_2=$(shasum -a 256 "${BUILD_DIR_2}/watchdog" | awk '{print $1}')
else
    echo "Error: Neither sha256sum nor shasum is available"
    exit 1
fi

echo "Build 1 Hash: ${HASH_1}"
echo "Build 2 Hash: ${HASH_2}"

if [ "${HASH_1}" = "${HASH_2}" ]; then
    echo "SUCCESS: Builds are byte-for-byte identical!"
    echo "Deterministic build verification passed."
    exit 0
else
    echo "FAILURE: Builds produced mismatched binary outputs."
    exit 1
fi
