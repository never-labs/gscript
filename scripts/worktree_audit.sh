#!/usr/bin/env bash
# Report prunable, dirty, and ahead/behind git worktrees without modifying them.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

FAIL_ON_FINDINGS=0
JSON_OUT=0
FINDING_STATUS=()
FINDING_PATH=()
FINDING_BRANCH=()
FINDING_DETAIL=()
FINDING_STATUS_KEYS=()
FINDING_STATUS_COUNTS=()

usage() {
    cat <<'EOF'
Usage: scripts/worktree_audit.sh [--fail-on-findings] [--json] [--help]

Lists git worktrees that need attention:
  - prunable worktree records from `git worktree list --porcelain`;
  - dirty worktrees with tracked or untracked changes;
  - worktrees whose branch is ahead of or behind its upstream.

The script is read-only. It never runs `git worktree prune`, removes a
worktree, or changes files. By default findings are reported with exit 0.
Use --fail-on-findings to exit 1 when any finding is present.

Options:
  --json              Print a machine-readable worktree audit report.
EOF
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --fail-on-findings)
            FAIL_ON_FINDINGS=1
            shift
            ;;
        --json)
            JSON_OUT=1
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

json_escape() {
    local value="$1"
    value="${value//\\/\\\\}"
    value="${value//\"/\\\"}"
    value="${value//$'\n'/\\n}"
    value="${value//$'\r'/\\r}"
    value="${value//$'\t'/\\t}"
    printf '%s' "$value"
}

print_json_report() {
    local status="pass"
    if [ "$have_findings" -ne 0 ]; then
        status="findings"
    fi
    printf '{\n'
    printf '  "schema_version": 1,\n'
    printf '  "status": "%s",\n' "$status"
    printf '  "fail_on_findings": %s,\n' "$(if [ "$FAIL_ON_FINDINGS" -eq 1 ]; then printf true; else printf false; fi)"
    printf '  "finding_count": %d,\n' "${#FINDING_STATUS[@]}"
    printf '  "finding_status_count": %d,\n' "${#FINDING_STATUS_KEYS[@]}"
    printf '  "finding_statuses": [\n'
    local status_i=0
    while [ "$status_i" -lt "${#FINDING_STATUS_KEYS[@]}" ]; do
        printf '    {"status": "%s", "count": %d}' \
            "$(json_escape "${FINDING_STATUS_KEYS[$status_i]}")" \
            "${FINDING_STATUS_COUNTS[$status_i]}"
        if [ "$status_i" -lt $((${#FINDING_STATUS_KEYS[@]} - 1)) ]; then
            printf ','
        fi
        printf '\n'
        status_i=$((status_i + 1))
    done
    printf '  ],\n'
    printf '  "findings": [\n'
    local i=0
    while [ "$i" -lt "${#FINDING_STATUS[@]}" ]; do
        printf '    {"status": "%s", "path": "%s", "branch": "%s", "detail": "%s"}' \
            "$(json_escape "${FINDING_STATUS[$i]}")" \
            "$(json_escape "${FINDING_PATH[$i]}")" \
            "$(json_escape "${FINDING_BRANCH[$i]}")" \
            "$(json_escape "${FINDING_DETAIL[$i]}")"
        if [ "$i" -lt $((${#FINDING_STATUS[@]} - 1)) ]; then
            printf ','
        fi
        printf '\n'
        i=$((i + 1))
    done
    printf '  ]\n'
    printf '}\n'
}

record_status_count() {
    local status="$1"
    local i=0
    while [ "$i" -lt "${#FINDING_STATUS_KEYS[@]}" ]; do
        if [ "${FINDING_STATUS_KEYS[$i]}" = "$status" ]; then
            FINDING_STATUS_COUNTS[$i]=$((FINDING_STATUS_COUNTS[$i] + 1))
            return
        fi
        i=$((i + 1))
    done
    FINDING_STATUS_KEYS+=("$status")
    FINDING_STATUS_COUNTS+=(1)
}

print_header() {
    if [ "$JSON_OUT" -eq 1 ]; then
        return
    fi
    printf '%-13s %-48s %-36s %s\n' "STATUS" "WORKTREE" "BRANCH" "DETAIL"
    printf '%-13s %-48s %-36s %s\n' "------" "--------" "------" "------"
}

print_finding() {
    local status="$1"
    local path="$2"
    local branch="$3"
    local detail="$4"

    have_findings=1
    FINDING_STATUS+=("$status")
    FINDING_PATH+=("$path")
    FINDING_BRANCH+=("$branch")
    FINDING_DETAIL+=("$detail")
    record_status_count "$status"
    if [ "$JSON_OUT" -ne 1 ]; then
        printf '%-13s %-48s %-36s %s\n' "$status" "$path" "$branch" "$detail"
    fi
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

if [ "$JSON_OUT" -eq 1 ]; then
    print_json_report
elif [ "$have_findings" -eq 0 ]; then
    echo "No prunable, dirty, or ahead/behind worktrees found."
fi

if [ "$FAIL_ON_FINDINGS" -eq 1 ] && [ "$have_findings" -ne 0 ]; then
    exit 1
fi
