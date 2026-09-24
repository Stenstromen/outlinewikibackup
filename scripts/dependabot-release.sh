#!/usr/bin/env bash
# Merge one open Dependabot pull request, or cut a patch release once the queue is empty.
# Subcommands: plan, merge <number>, release
set -euo pipefail

minimum_prs="${MINIMUM_PRS:-3}"
repo="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"

usage() {
  echo "usage: $0 plan|merge <number>|release" >&2
  exit 1
}

latest_tag() {
  local tag
  while read -r tag; do
    if [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
      printf '%s\n' "$tag"
      return 0
    fi
  done < <(git tag -l 'v*' --sort=-v:refname)
  return 0
}

read_open_prs() {
  local text number
  if ! text="$(
    gh pr list --repo "$repo" --state open --base main --limit 100 \
      --json number,author,isDraft,createdAt,headRefName \
      | jq -r '
          [
            .[]
            | select(.isDraft | not)
            | select(
                .author.login == "dependabot[bot]"
                or .author.login == "app/dependabot"
                or (.headRefName | startswith("dependabot/"))
              )
          ]
          | sort_by(.createdAt)
          | .[].number
        '
  )"; then
    echo "Failed to list Dependabot pull requests." >&2
    exit 1
  fi
  prs=()
  while IFS= read -r number; do
    if [[ -n "$number" ]]; then
      prs+=("$number")
    fi
  done <<<"$text"
  return 0
}

has_unreleased_dependabot() {
  local latest="$1"
  local matches
  matches="$(git log "${latest}..origin/main" --grep='dependabot/' --pretty=%H)"
  [[ -n "$matches" ]]
}

fetch_main() {
  git fetch origin main:refs/remotes/origin/main --tags --force
}

set_output() {
  local name="$1"
  local value="$2"
  if [[ -z "${GITHUB_OUTPUT:-}" ]]; then
    echo "GITHUB_OUTPUT is not set" >&2
    exit 1
  fi
  printf '%s=%s\n' "$name" "$value" >> "$GITHUB_OUTPUT"
}

mergeable_state() {
  local number="$1"
  local state="UNKNOWN"
  local attempt
  for attempt in 1 2 3 4 5; do
    state="$(gh pr view "$number" --repo "$repo" --json mergeable --jq .mergeable)"
    if [[ "$state" != "UNKNOWN" ]]; then
      printf '%s\n' "$state"
      return
    fi
    sleep 3
  done
  printf '%s\n' "$state"
}

cmd_plan() {
  fetch_main
  local -a prs=()
  local latest="" pending=0 number state
  read_open_prs
  latest="$(latest_tag)"
  if [[ -n "$latest" ]] && has_unreleased_dependabot "$latest"; then
    pending=1
  fi

  if [[ ${#prs[@]} -eq 0 ]]; then
    if [[ "$pending" -eq 1 ]]; then
      set_output action release
      set_output pr ""
      echo "No open Dependabot pull requests. Creating the patch release."
      return
    fi
    set_output action noop
    set_output pr ""
    echo "Nothing to do."
    return
  fi

  if [[ ${#prs[@]} -lt "$minimum_prs" && "$pending" -eq 0 ]]; then
    set_output action noop
    set_output pr ""
    echo "Waiting for at least ${minimum_prs} Dependabot pull requests (have ${#prs[@]})."
    return
  fi

  if [[ "$pending" -eq 1 && ${#prs[@]} -lt "$minimum_prs" ]]; then
    echo "Release batch already started since ${latest}. ${#prs[@]} Dependabot pull request(s) still open."
  fi

  for number in "${prs[@]}"; do
    state="$(mergeable_state "$number")"
    if [[ "$state" == "MERGEABLE" ]]; then
      set_output action merge
      set_output pr "$number"
      echo "Will test and merge pull request #${number}."
      return
    fi
    echo "Pull request #${number} is ${state}."
  done

  set_output action noop
  set_output pr ""
  echo "No mergeable Dependabot pull request yet. Waiting for Dependabot to rebase."
}

assert_dependabot_pr() {
  local number="$1"
  local meta
  meta="$(gh pr view "$number" --repo "$repo" --json author,baseRefName,state,isDraft,headRefName,headRepository)"
  jq -e --arg repo "$repo" '
    .state == "OPEN"
    and (.isDraft | not)
    and (.baseRefName == "main")
    and (
      .author.login == "dependabot[bot]"
      or .author.login == "app/dependabot"
      or (.headRefName | startswith("dependabot/"))
    )
    and ((.headRepository.nameWithOwner | ascii_downcase) == ($repo | ascii_downcase))
  ' <<<"$meta" >/dev/null
}

cmd_merge() {
  local number="${1:-}"
  local state sha
  local -a prs=()
  [[ "$number" =~ ^[0-9]+$ ]] || usage
  release_after=false
  trap 'set_output release "$release_after"' EXIT
  fetch_main
  git checkout -B main origin/main

  if ! assert_dependabot_pr "$number"; then
    echo "Pull request #${number} is not an open Dependabot pull request against main."
    exit 1
  fi

  state="$(mergeable_state "$number")"
  if [[ "$state" != "MERGEABLE" ]]; then
    echo "Pull request #${number} is ${state}. Waiting for Dependabot to rebase."
    exit 0
  fi

  git fetch origin "pull/${number}/head:pr-${number}"
  sha="$(git rev-parse "pr-${number}")"
  git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
  git config user.name "github-actions[bot]"
  if ! git merge --no-edit "pr-${number}"; then
    git merge --abort || true
    echo "Pull request #${number} conflicts with main. Waiting for Dependabot to rebase."
    exit 0
  fi

  bash scripts/integration-test.sh

  gh pr merge "$number" --repo "$repo" --merge --match-head-commit "$sha"

  read_open_prs
  if [[ ${#prs[@]} -eq 0 ]]; then
    release_after="true"
    echo "Merged #${number}. No Dependabot pull requests remain, so the patch release can be created."
  else
    echo "Merged #${number}. ${#prs[@]} Dependabot pull request(s) still open."
    echo "Starting the next run to test one more pull request."
    gh workflow run dependabot-patch-release.yaml --repo "$repo" --ref main
  fi
}

cmd_release() {
  local latest major minor patch tag notes
  local -a prs=()
  fetch_main
  latest="$(latest_tag)"
  if [[ -z "$latest" ]]; then
    echo "No semver tag found."
    exit 1
  fi
  if ! has_unreleased_dependabot "$latest"; then
    echo "No unreleased Dependabot merges on main."
    exit 0
  fi
  read_open_prs
  if [[ ${#prs[@]} -gt 0 ]]; then
    echo "Dependabot pull requests are still open. Not releasing."
    exit 0
  fi

  IFS=. read -r major minor patch <<<"${latest#v}"
  tag="v${major}.${minor}.$((patch + 1))"
  if git rev-parse -q --verify "refs/tags/${tag}" >/dev/null; then
    echo "Tag ${tag} already exists."
    exit 1
  fi

  notes="$(git log "${latest}..origin/main" --grep='dependabot/' --pretty=format:'- %s')"
  gh release create "$tag" \
    --repo "$repo" \
    --target main \
    --title "$tag" \
    --notes "$(printf '%s\n\n%s\n' "Dependabot updates:" "$notes")"
  echo "Created ${tag}."

  # A release created with GITHUB_TOKEN does not start other workflows.
  # OutlineWiki Backup CI builds the image and updates Flux; workflow_dispatch is allowed.
  gh workflow run main.yaml --repo "$repo" --ref main -f "tag=${tag}"
  echo "Started OutlineWiki Backup CI for ${tag}."
}

command="${1:-}"
case "$command" in
  plan) cmd_plan ;;
  merge) cmd_merge "${2:-}" ;;
  release) cmd_release ;;
  *) usage ;;
esac
