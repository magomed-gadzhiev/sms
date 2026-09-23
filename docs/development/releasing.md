# Releasing

The project uses manual, tag-driven releases: the git tag is the source of
truth for the version, `CHANGELOG.md` is maintained by hand, and the GitHub
Release is created with `gh` from the changelog section.

Versioning follows [Semantic Versioning](https://semver.org/) with the `v`
prefix (`v0.1.0`). While the major version is `0`, breaking changes bump the
minor version; anything else bumps the patch version.

## Cutting a release

1. **Collect the changes.** Move everything from the `[Unreleased]` section of
   `CHANGELOG.md` into a new `## [X.Y.Z] - YYYY-MM-DD` section (Keep a
   Changelog headings: Added / Changed / Deprecated / Removed / Fixed /
   Security). Leave `[Unreleased]` empty.
2. **Update the link references** at the bottom of `CHANGELOG.md`: add
   `[X.Y.Z]: …/releases/tag/vX.Y.Z` and repoint `[Unreleased]` to
   `…/compare/vX.Y.Z...HEAD`.
3. **Open a PR** with the changelog update and merge it into `main`.
4. **Tag.** On the merged `main`:

   ```bash
   git checkout main && git pull
   git tag -a vX.Y.Z -m "vX.Y.Z"
   git push origin vX.Y.Z
   ```

5. **Create the GitHub Release** with the notes taken from the changelog
   section:

   ```bash
   gh release create vX.Y.Z --title "vX.Y.Z" --notes-file <(awk '/^## \[X.Y.Z\]/{f=1;next} /^## /{f=0} f' CHANGELOG.md)
   ```

6. **Check** the release page, the README release badge, and that
   `make version` on the new tag prints `vX.Y.Z`.

## Version in binaries

`internal/version.Version` defaults to `dev` and is stamped at build time.
The Makefile passes `git describe --tags --always --dirty` via ldflags, so
binaries built from a checkout report the release they were built from.
