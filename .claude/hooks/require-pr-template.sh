#!/usr/bin/env bash
# require-pr-template.sh - PreToolUse hook that blocks PR creation when the body
# is missing required section headers from .github/pull_request_template.md.
#
# Handles two surfaces:
#   Bash tool                          -> gh pr create ... --body "..."
#   mcp__github__create_pull_request   -> tool_input.body
#
# Exit codes:
#   0         - allow (passes, or not a PR-create call, or template missing)
#   non-zero  - block with explanation

set -uo pipefail

TEMPLATE="${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel 2>/dev/null)}/.github/pull_request_template.md"
[ -f "$TEMPLATE" ] || exit 0

HOOK_INPUT="$(cat)"
[ -n "$HOOK_INPUT" ] || exit 0

TOOL_NAME="$(printf '%s' "$HOOK_INPUT" | python3 -c "import sys,json; print(json.loads(sys.stdin.read()).get('tool_name',''))" 2>/dev/null || true)"
HOOK_EVENT_NAME="$(printf '%s' "$HOOK_INPUT" | python3 -c "import sys,json; print(json.loads(sys.stdin.read()).get('hook_event_name','PreToolUse'))" 2>/dev/null || echo PreToolUse)"

PR_BODY=""

case "$TOOL_NAME" in
  mcp__github__create_pull_request)
    PR_BODY="$(printf '%s' "$HOOK_INPUT" | python3 -c "import sys,json; print(json.loads(sys.stdin.read()).get('tool_input',{}).get('body',''))" 2>/dev/null || true)"
    ;;
  Bash|shell|exec|local_shell)
    COMMAND_STR="$(printf '%s' "$HOOK_INPUT" | python3 -c "import sys,json; t=json.loads(sys.stdin.read()).get('tool_input',{}); print(t.get('command','') or ' '.join(t.get('command_array',[])) or ' '.join(t.get('argv',[])))" 2>/dev/null || true)"
    echo "$COMMAND_STR" | grep -q 'gh pr create' || exit 0

    COMMAND_STR="$(echo "$COMMAND_STR" | sed 's/^[[:space:]]*cd[^&]*&&[[:space:]]*//')"

    PR_BODY="$(COMMAND_STR="$COMMAND_STR" python3 -c "
import os, re, shlex, sys
cmd = os.environ.get('COMMAND_STR', '').strip()
try:
    args = shlex.split(cmd)
except ValueError:
    sys.exit(0)

for i, a in enumerate(args):
    if a in ('--body', '-b') and i + 1 < len(args):
        print(args[i + 1])
        break
    if a in ('--body-file', '-F') and i + 1 < len(args):
        try:
            print(open(args[i + 1]).read())
        except OSError:
            pass
        break
    m = re.match(r'^(?:--body|-b)=(.+)$', a, re.DOTALL)
    if m:
        print(m.group(1))
        break
    m = re.match(r'^(?:--body-file|-F)=(.+)$', a)
    if m:
        try:
            print(open(m.group(1)).read())
        except OSError:
            pass
        break
" 2>/dev/null || true)"
    ;;
  *)
    exit 0
    ;;
esac

[ -n "$PR_BODY" ] || exit 0

# Validate required headers from the template are present in the PR body
DENY_REASON="$(TEMPLATE="$TEMPLATE" PR_BODY="$PR_BODY" python3 -c '
import os, re, sys

TEMPLATE = os.environ["TEMPLATE"]
PR_BODY = os.environ["PR_BODY"]
HEADER_RE = re.compile(r"^##\s+.+?\s*$", re.MULTILINE)

required_headers = [m.group(0).rstrip() for m in HEADER_RE.finditer(open(TEMPLATE).read())]
body_headers = {m.group(0).rstrip() for m in HEADER_RE.finditer(PR_BODY)}

missing = [h for h in required_headers if h not in body_headers]
if missing:
    lines = ["PR body is missing required section(s) from .github/pull_request_template.md:"]
    lines += [f"  - {h}" for h in missing]
    lines.append("")
    lines.append("Please include every ## section from the template in the PR body.")
    print("\n".join(lines))
' 2>/dev/null || true)"

if [ -n "$DENY_REASON" ]; then
  HOOK_EVENT_NAME="$HOOK_EVENT_NAME" python3 -c "
import json, os, sys
reason = sys.stdin.read()
print(json.dumps({
    'hookSpecificOutput': {
        'hookEventName': os.environ['HOOK_EVENT_NAME'],
        'permissionDecision': 'deny',
        'permissionDecisionReason': reason,
    }
}))
" <<< "$DENY_REASON"
  exit 0
fi

exit 0
