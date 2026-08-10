#!/usr/bin/env bash
# Validate release metadata, release documentation, and an optional built binary.
set -euo pipefail

usage() {
	cat >&2 <<'EOF'
Usage: scripts/release-verify.sh --tag vX.Y.Z [options]

Options:
  --target <git-ref>       Commit expected to be released (default: HEAD)
  --binary <path>          Verify CLI help and embedded version from this binary
  --require-clean          Fail when the working tree has tracked or untracked changes
  --require-existing-tag   Fail unless --tag already resolves to a Git tag
EOF
	exit 2
}

tag=""
target="HEAD"
binary=""
require_clean=false
require_existing_tag=false

while [[ $# -gt 0 ]]; do
	case "$1" in
	--tag)
		[[ $# -ge 2 ]] || usage
		tag="$2"
		shift 2
		;;
	--target)
		[[ $# -ge 2 ]] || usage
		target="$2"
		shift 2
		;;
	--binary)
		[[ $# -ge 2 ]] || usage
		binary="$2"
		shift 2
		;;
	--require-clean)
		require_clean=true
		shift
		;;
	--require-existing-tag)
		require_existing_tag=true
		shift
		;;
	-h | --help)
		usage
		;;
	*)
		echo "unknown option: $1" >&2
		usage
		;;
	esac
done

[[ -n "$tag" ]] || usage
semver='^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-([0-9A-Za-z-]+)(\.[0-9A-Za-z-]+)*)?(\+([0-9A-Za-z-]+)(\.[0-9A-Za-z-]+)*)?$'
if ! [[ "$tag" =~ $semver ]]; then
	echo "release tag must be SemVer, for example v1.2.3: $tag" >&2
	exit 1
fi

if "$require_clean" && [[ -n "$(git status --porcelain)" ]]; then
	echo "release verification requires a clean working tree" >&2
	git status --short >&2
	exit 1
fi

target_sha="$(git rev-parse --verify "${target}^{commit}")"
if tag_sha="$(git rev-parse -q --verify "refs/tags/${tag}^{commit}")"; then
	if [[ "$tag_sha" != "$target_sha" ]]; then
		echo "tag $tag points to $tag_sha, not the release commit $target_sha" >&2
		exit 1
	fi
elif "$require_existing_tag"; then
	echo "release tag does not exist: $tag" >&2
	exit 1
else
	echo "pre-tag verification: $tag will be expected to point to $target_sha"
fi

previous_tag=""
while IFS= read -r candidate; do
	if [[ "$candidate" =~ $semver && "$candidate" != "$tag" ]]; then
		previous_tag="$candidate"
		break
	fi
done < <(git tag --merged "$target_sha" --sort=-version:refname)

if [[ -n "$previous_tag" ]]; then
	changed_files="$(git diff --name-only "${previous_tag}..${target_sha}" -- README.md README_zh.md)"
	for readme in README.md README_zh.md; do
		if ! grep -Fxq "$readme" <<<"$changed_files"; then
			echo "$readme has not been updated since $previous_tag; review and update both release READMEs before tagging" >&2
			exit 1
		fi
	done
fi

if [[ -n "$binary" ]]; then
	[[ -x "$binary" ]] || {
		echo "release binary is not executable: $binary" >&2
		exit 1
	}

	help_output="$("$binary" --help)"
	for command in start stop restart status run version; do
		if ! grep -Eq "^  ${command}[[:space:]]" <<<"$help_output"; then
			echo "release binary help is missing command: $command" >&2
			exit 1
		fi
		for readme in README.md README_zh.md; do
			if ! grep -Fq "umbragate ${command}" "$readme"; then
				echo "$readme does not document command: umbragate $command" >&2
				exit 1
			fi
		done
	done

	if ! grep -Fq -- '-config string' <<<"$help_output"; then
		echo "release binary help is missing the -config flag" >&2
		exit 1
	fi
	for readme in README.md README_zh.md; do
		if ! grep -Fq 'umbragate --help' "$readme" || ! grep -Fq -- '-config' "$readme"; then
			echo "$readme does not document --help and -config" >&2
			exit 1
		fi
	done

	expected_version="umbragate ${tag#v}"
	if [[ "$("$binary" version)" != "$expected_version" ]]; then
		echo "release binary version does not match $tag" >&2
		exit 1
	fi
fi

echo "release verification passed for $tag at $target_sha"
