#!/bin/bash
# Validates that Containerfile FROM directives use pinned tags and trusted registries.
# Usage: check-containerfile-tags.sh <Containerfile> [<Containerfile2> ...]
# Set ENFORCE=0 to treat failures as warnings instead of errors.

set -euo pipefail

ENFORCE="${ENFORCE:-1}"
EXITCODE=0

TRUSTED_REGISTRIES=(
  "docker.io"
  "quay.io"
  "ghcr.io"
  "registry.fedoraproject.org"
  "registry.access.redhat.com"
  "registry.redhat.io"
  "gcr.io"
)

check_file() {
  local file="$1"
  if [ ! -f "$file" ]; then
    echo "ERROR: File not found: $file"
    return 1
  fi

  local line_num=0
  local errors=0

  while IFS= read -r line; do
    line_num=$((line_num + 1))

    # Skip comments and empty lines
    [[ "$line" =~ ^[[:space:]]*# ]] && continue
    [[ -z "${line// /}" ]] && continue

    # Match FROM directives (including multi-stage "AS" aliases)
    if [[ "$line" =~ ^[[:space:]]*FROM[[:space:]]+ ]]; then
      local image
      image=$(echo "$line" | awk '{print $2}')

      # Skip build args (e.g., FROM ${BASE_IMAGE})
      if [[ "$image" == \$* ]]; then
        continue
      fi

      # Check for :latest or no tag
      if [[ "$image" == *":latest" ]]; then
        echo "ERROR: $file:$line_num: ':latest' tag used: $image"
        errors=$((errors + 1))
      elif [[ "$image" != *":"* ]] && [[ "$image" != *"@sha256:"* ]]; then
        echo "ERROR: $file:$line_num: no tag specified: $image"
        errors=$((errors + 1))
      fi

      # Check trusted registry
      local registry_ok=0
      for registry in "${TRUSTED_REGISTRIES[@]}"; do
        if [[ "$image" == "${registry}/"* ]] || [[ "$image" == *"/"* && "$image" != *"."* ]]; then
          # docker.io images can be short-form (e.g., library/golang:1.26)
          registry_ok=1
          break
        fi
      done

      if [ "$registry_ok" -eq 0 ]; then
        # Check if it's a Docker Hub official image (no registry prefix, e.g., golang:1.26)
        if [[ "$image" != *"."* ]] || [[ "$image" == "docker.io/"* ]]; then
          registry_ok=1
        fi
      fi

      if [ "$registry_ok" -eq 0 ]; then
        echo "WARNING: $file:$line_num: unrecognized registry: $image"
      fi
    fi
  done < "$file"

  return "$errors"
}

if [ $# -eq 0 ]; then
  echo "Usage: $0 <Containerfile> [<Containerfile2> ...]"
  exit 1
fi

for file in "$@"; do
  echo "Checking: $file"
  if ! check_file "$file"; then
    EXITCODE=1
  fi
done

if [ "$EXITCODE" -ne 0 ] && [ "$ENFORCE" -eq 1 ]; then
  echo "FAIL: Containerfile tag validation failed"
  exit 1
elif [ "$EXITCODE" -ne 0 ]; then
  echo "WARN: Containerfile tag issues found (ENFORCE=0, not failing)"
fi

echo "Containerfile tag check complete."
