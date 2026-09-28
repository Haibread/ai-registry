#!/usr/bin/env bash
# Cuts a release: bumps the version in the repository, commits, tags, and asks
# before pushing. Pushing the tag is what makes CI publish.
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/release.sh [--dry-run] <version>
       scripts/release.sh [--dry-run] --chart [--app-version <version>] <version>

Releases the application (tag vX.Y.Z: server and web images, GitHub Release),
or with --chart the Helm chart (tag chart-X.Y.Z), which versions on its own.

An application release needs a "## vX.Y.Z" section in CHANGELOG.md, committed
beforehand: the GitHub Release takes its notes from it.

Options:
  --chart                  release the Helm chart instead of the application
  --app-version <version>  application version the chart deploys
                           (default: the latest v* tag)
  --dry-run                show what would change, change nothing
  -h, --help               show this help

Examples:
  scripts/release.sh 0.4.0
  scripts/release.sh 0.5.0-rc1
  scripts/release.sh --chart 0.4.0
  scripts/release.sh --chart --app-version 0.4.0 0.4.1
  scripts/release.sh --dry-run 0.4.1
USAGE
}

die() {
  echo "error: $*" >&2
  exit 1
}

chart=false
dry_run=false
app_version=""
version=""
while [ $# -gt 0 ]; do
  case "$1" in
    --chart) chart=true ;;
    --dry-run) dry_run=true ;;
    --app-version)
      [ $# -ge 2 ] || die "--app-version needs a value"
      app_version="$2"
      shift
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    -*) die "unknown option $1 (see --help)" ;;
    *)
      [ -z "$version" ] || die "only one version can be given"
      version="$1"
      ;;
  esac
  shift
done

semver='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$'
[ -n "$version" ] || die "a version is required (see --help)"
[[ "$version" =~ $semver ]] || die "${version} is not a semantic version (X.Y.Z)"
[ -z "$app_version" ] || $chart || die "--app-version only applies to --chart"

cd "$(git rev-parse --show-toplevel)"

branch="$(git rev-parse --abbrev-ref HEAD)"
default_branch="$(git symbolic-ref --short refs/remotes/origin/HEAD 2>/dev/null | sed 's|^origin/||')"
default_branch="${default_branch:-main}"
[ "$branch" = "$default_branch" ] || die "releases are cut from ${default_branch}, not ${branch}"
[ -z "$(git status --porcelain)" ] || die "the working tree is not clean; commit or stash first"
git fetch --quiet --tags origin "$branch"
behind="$(git rev-list --count "HEAD..origin/${branch}")"
[ "$behind" -eq 0 ] || die "${branch} is ${behind} commit(s) behind origin; pull first"

chart_dir=deploy/helm/ai-registry
chart_file="${chart_dir}/Chart.yaml"
if $chart; then
  tag="chart-${version}"
  if [ -z "$app_version" ]; then
    # Without the suffix setting, git ranks v1.0.0-rc1 above v1.0.0.
    latest="$(git -c versionsort.suffix=- tag --list 'v*' --sort=-version:refname | head -n 1)"
    [ -n "$latest" ] || die "no application release (v*) exists yet; pass --app-version"
    app_version="${latest#v}"
  fi
  [[ "$app_version" =~ $semver ]] || die "${app_version} is not a semantic version"
  git rev-parse --quiet --verify "refs/tags/v${app_version}" >/dev/null \
    || die "v${app_version} is not a released application version"
  command -v helm-docs >/dev/null || die "helm-docs is missing: see CONTRIBUTING.md"
  current="$(sed -n 's/^version: *//p' "$chart_file")"
  subject="chore(chart): release ${version}"
else
  tag="v${version}"
  grep -Eq "^## ${tag//./\\.}([[:space:]]|$)" CHANGELOG.md \
    || die "CHANGELOG.md has no \"## ${tag}\" section; add and commit it first"
  command -v npm >/dev/null || die "npm is missing"
  current="$(sed -n 's/^  "version": "\(.*\)",$/\1/p' web/package.json)"
  subject="chore: release ${version}"
fi
! git rev-parse --quiet --verify "refs/tags/${tag}" >/dev/null || die "tag ${tag} already exists"

if $dry_run; then
  echo "current version: ${current}"
  echo "target version:  ${version}"
  if $chart; then echo "app version:     ${app_version}"; fi
  echo "commit:          ${subject}"
  echo "tag:             ${tag}"
  exit 0
fi

if $chart; then
  sed -i.bak -e "s/^version: .*/version: ${version}/" -e "s/^appVersion: .*/appVersion: \"${app_version}\"/" "$chart_file"
  rm "${chart_file}.bak"
  # Same invocation as the helm-docs pre-commit hook, so the output is identical.
  (cd "$chart_dir" && helm-docs --log-level warning)
  git add "$chart_file" "${chart_dir}/README.md"
else
  npm --prefix web version "$version" --no-git-tag-version --allow-same-version >/dev/null
  git add web/package.json web/package-lock.json
fi
git diff --cached --quiet && die "nothing changed: the repository already says ${version}"
git commit --quiet --message "$subject"
git tag --annotate "$tag" --message "$subject"
echo "committed \"${subject}\" and tagged ${tag}"

push_command="git push --atomic origin ${branch} ${tag}"
if [ -t 0 ]; then
  printf 'push %s and %s to origin? this publishes the release. [y/N] ' "$branch" "$tag"
  read -r reply
else
  reply=""
fi
case "$reply" in
  y | Y | yes)
    $push_command
    ;;
  *)
    echo "not pushed. publish later with: ${push_command}"
    ;;
esac
