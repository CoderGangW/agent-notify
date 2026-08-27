#!/bin/sh
# ulio-style release: version bump → tag → GitHub release with the body
# from release_notes/unreleased.md → signed macOS artifacts uploaded →
# the note archived as release_notes/v<ver>.md and the draft reset.
#   build/release.sh            # patch-bump from the latest tag
#   build/release.sh 0.3.0      # explicit version
set -e
cd "$(dirname "$0")/.."

NOTES=release_notes/unreleased.md
[ -s "$NOTES" ] || { echo "release_notes/unreleased.md is empty" >&2; exit 1; }

if [ -n "$1" ]; then
  VERSION="$1"
else
  LATEST=$(gh release view --json tagName -q .tagName 2>/dev/null | sed 's/^v//')
  [ -n "$LATEST" ] || LATEST=$(sed -n 's/^const version = "\(.*\)"/\1/p' main.go)
  VERSION=$(echo "$LATEST" | awk -F. '{$NF+=1; print}' OFS=.)
fi
TAG="v$VERSION"
echo "▶ releasing $TAG"

CUR=$(sed -n 's/^const version = "\(.*\)"/\1/p' main.go)
sed -i '' "s/const version = \"$CUR\"/const version = \"$VERSION\"/" main.go
sed -i '' "s/\"$CUR\"/\"$VERSION\"/g" build/winres/winres.json

# strip the draft's comment header for the release body
BODY=$(sed '/^<!--/,/-->$/d' "$NOTES")
git add -A && git commit -m "release: $TAG"
git tag "$TAG"
git push origin main "$TAG"
gh release create "$TAG" --title "$TAG" --notes "$BODY"

./build/release-macos.sh "$TAG"

cp "$NOTES" "release_notes/$TAG.md"
cp release_notes/TEMPLATE.md "$NOTES"
git add release_notes && git commit -m "docs: archive $TAG release notes" && git push origin main
echo "▶ done: $TAG"
