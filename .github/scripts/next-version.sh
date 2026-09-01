#!/usr/bin/env bash

set -euo pipefail

latest_tag=""
while IFS= read -r tag; do
	if [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
		latest_tag="$tag"
		break
	fi
done < <(git tag --list 'v*' --sort=-version:refname)

major=0
minor=0
patch=0
if [[ -n "$latest_tag" ]]; then
	IFS=. read -r major minor patch <<<"${latest_tag#v}"
fi

# This repository's earlier releases were v0.x. The first automated release
# deliberately starts the new release line at v1.0.0.
if [[ -z "$latest_tag" || "$major" -lt 1 ]]; then
	echo "1.0.0"
	exit 0
fi

bump="patch"

while IFS= read -r commit; do
	message="$(git show -s --format=%B "$commit")"
	subject="${message%%$'\n'*}"

	if grep -Eiq '^[[:alnum:]-]+(\([^)]*\))?!:' <<<"$subject" || \
		grep -Eiq '^BREAKING[ -]CHANGE:[[:space:]]' <<<"$message"; then
		bump="major"
		break
	fi

	if grep -Eiq '^feat(\([^)]*\))?:' <<<"$subject"; then
		bump="minor"
	fi
done < <(git rev-list "${latest_tag}..HEAD")

case "$bump" in
	major)
		major=$((major + 1))
		minor=0
		patch=0
		;;
	minor)
		minor=$((minor + 1))
		patch=0
		;;
	patch)
		patch=$((patch + 1))
		;;
esac

echo "${major}.${minor}.${patch}"
