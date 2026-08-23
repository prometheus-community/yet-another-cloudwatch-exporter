# RELEASE

Releases are tag-triggered: pushing a `vX.Y.Z` tag runs CI, which builds the artifacts and
publishes them as a **draft** GitHub release that must be manually finalized.

## 1. Prepare the release PR

Bump `VERSION` and add a new section to `CHANGELOG.md`, then open a PR and get it merged to
`master`.

Example: PR [#1901](https://github.com/prometheus-community/yet-another-cloudwatch-exporter/pull/1901)
did exactly this.

## 2. Tag and push

Merging the version-bump PR does **not** trigger a release by itself — pushing the tag is what
triggers the release workflow:

```
git tag vX.Y.Z
git push origin vX.Y.Z
```

## 3. Wait for CI

Pushing the tag runs the `publish_release` job in `.github/workflows/ci.yml` and creates a **draft** GitHub release.

## 4. Publish the release

Go to the repo's [Releases page](https://github.com/prometheus-community/yet-another-cloudwatch-exporter/releases),
review the auto-generated draft for `vX.Y.Z`, edit the notes if needed, and click
**Publish release** to move it out of draft.
