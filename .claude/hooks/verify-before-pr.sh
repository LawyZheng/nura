#!/usr/bin/env bash
set -euo pipefail

# Lightweight pre-PR verification for Nura.
# Does NOT assume Go files exist yet. Checks governance files and obvious issues.

REPO_ROOT="$(git rev-parse --show-toplevel)"
ERRORS=()

# 1. Governance files must exist
for f in AGENTS.md CLAUDE.md; do
  if [ ! -f "$REPO_ROOT/$f" ]; then
    ERRORS+=("Missing required file: $f")
  fi
done

# 2. Scan staged/changed files for suspicious patient-data markers.
# Keep patterns narrow to avoid false positives in governance docs that say
# "do not use real patient data".
SUSPECT_PATTERNS="REAL[ _-]?PATIENT[ _-]?DATA|DOB:.*19[0-9][0-9]|SSN.*[0-9]{3}-[0-9]{2}-[0-9]{4}|MRN[: ]*[0-9]"
STAGED_FILES="$(git diff --cached --name-only 2>/dev/null || true)"
if [ -n "$STAGED_FILES" ]; then
  CHANGED_FILES="$STAGED_FILES"
elif git rev-parse --verify origin/main >/dev/null 2>&1; then
  CHANGED_FILES="$(git diff --name-only origin/main...HEAD 2>/dev/null || true)"
else
  CHANGED_FILES="$(git ls-files --modified --others --exclude-standard 2>/dev/null || true)"
fi

if [ -n "$CHANGED_FILES" ]; then
  while IFS= read -r file; do
    [ -f "$REPO_ROOT/$file" ] || continue
    if grep -qEi "$SUSPECT_PATTERNS" "$REPO_ROOT/$file" 2>/dev/null; then
      ERRORS+=("Possible real patient data in: $file")
    fi
  done <<< "$CHANGED_FILES"
fi

# 3. Check for secrets patterns in changed files
SECRET_PATTERNS="PRIVATE.KEY|BEGIN RSA|sk-[a-zA-Z0-9]{20,}|ghp_[a-zA-Z0-9]{36}|AKIA[0-9A-Z]{16}"
if [ -n "$CHANGED_FILES" ]; then
  while IFS= read -r file; do
    [ -f "$REPO_ROOT/$file" ] || continue
    # Skip .env files (they are gitignored)
    [[ "$file" == *.env* ]] && continue
    if grep -qE "$SECRET_PATTERNS" "$REPO_ROOT/$file" 2>/dev/null; then
      ERRORS+=("Possible secret/key in: $file")
    fi
  done <<< "$CHANGED_FILES"
fi

# Report
if [ ${#ERRORS[@]} -gt 0 ]; then
  echo "Pre-PR verification failed:" >&2
  for err in "${ERRORS[@]}"; do
    echo "  - $err" >&2
  done
  exit 2
fi

exit 0
