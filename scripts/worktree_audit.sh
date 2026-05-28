#!/usr/bin/env bash
# Report prunable, dirty, and ahead/behind git worktrees without modifying them.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

FAIL_ON_FINDINGS=0

usage() {
    cat <<'EOF'
Usage: scripts/worktree_audit.sh [--fail-on-findings] [--help]

Lists git worktrees that need attention:
  - prunable worktree records from `git worktree list --porcelain`;
  - dirty worktrees with tracked or untracked changes;
  - worktrees whose branch is ahead of or behind its upstream.

The script is read-only. It never runs `git worktree prune`, removes a
worktree, or changes files. By default findings are reported with exit 0.
Use --fail-on-findings to exit 1 when any finding is present.
EOF
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --fail-on-findings)
            FAIL_ON_FINDINGS=1
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "unknown argument: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

have_findings=0

print_header() {
    printf '%-13s %-48s %-36s %s\n' "STATUS" "WORKTREE" "BRANCH" "DETAIL"
    printf '%-13s %-48s %-36s %s\n' "------" "--------" "------" "------"
}

print_finding() {
    local status="$1"
    local path="$2"
    local branch="$3"
    local detail="$4"

    have_findings=1
    printf '%-13s %-48s %-36s %s\n' "$status" "$path" "$branch" "$detail"
}

audit_accessible_worktree() {
    local path="$1"
    local branch="$2"
    local status_line status_body ahead_behind dirty_count detail

    if status_line="$(git -C "$path" status --porcelain=v1 --branch 2>/dev/null)"; then
        status_body="$(printf '%s\n' "$status_line" | sed '1d')"
        if [ -n "$status_body" ]; then
            dirty_count="$(printf '%s\n' "$status_body" | wc -l | tr -d ' ')"
            print_finding "dirty" "$path" "$branch" "$dirty_count changed/untracked paths"
        fi

        ahead_behind="$(
            printf '%s\n' "$status_line" |
                sed -n '1s/^## //p' |
                grep -Eo 'ahead [0-9]+|behind [0-9]+' |
                awk 'NR == 1 { out = $0; next } { out = out ", " $0 } END { print out }'
        )" || true
        if [ -n "$ahead_behind" ]; then
            detail="$ahead_behind"
            print_finding "ahead/behind" "$path" "$branch" "$detail"
        fi
    else
        print_finding "unreadable" "$path" "$branch" "git status failed"
    fi
}

print_header

path=""
branch=""
detached=0
prunable=""

flush_entry() {
    local branch_label

    if [ -z "$path" ]; then
        return
    fi

    if [ "$detached" -eq 1 ]; then
        branch_label="(detached)"
    elif [ -n "$branch" ]; then
        branch_label="${branch#refs/heads/}"
    else
        branch_label="(unknown)"
    fi

    if [ -n "$prunable" ]; then
        print_finding "prunable" "$path" "$branch_label" "$prunable"
    elif [ -d "$path" ]; then
        audit_accessible_worktree "$path" "$branch_label"
    else
        print_finding "missing" "$path" "$branch_label" "path does not exist"
    fi

    path=""
    branch=""
    detached=0
    prunable=""
}

while IFS= read -r line; do
    if [ -z "$line" ]; then
        flush_entry
        continue
    fi

    case "$line" in
        worktree\ *)
            path="${line#worktree }"
            ;;
        branch\ *)
            branch="${line#branch }"
            ;;
        detached)
            detached=1
            ;;
        prunable\ *)
            prunable="${line#prunable }"
            ;;
    esac
done < <(git worktree list --porcelain)
flush_entry

if [ "$have_findings" -eq 0 ]; then
    echo "No prunable, dirty, or ahead/behind worktrees found."
fi

if [ "$FAIL_ON_FINDINGS" -eq 1 ] && [ "$have_findings" -ne 0 ]; then
    exit 1
fi
