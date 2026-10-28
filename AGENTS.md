# MCLauncher TUI — Repository Notes

## Release checklist

Always follow this when publishing a release.

1. Verify the tree is clean and all checks pass locally:
   - `gofmt -l .` (CI enforces formatting on Linux)
   - `go vet ./...`
   - `go test ./...`
2. Write release notes to `docs/releases/vX.Y.Z.md` **before tagging**. Notes are written in Chinese unless the user asks otherwise and include:
   - a short release summary
   - new features
   - improvements and fixes
   - platform and compatibility notes
   - upgrade and installation notes
   - `**完整变更**：https://github.com/MoonCC233/MCLauncher-TUI/compare/vPREV...vX.Y.Z`
3. Commit the release notes and any related changes to `main`, then push.
4. Create and push an annotated tag:

   ```bash
   git tag -a vX.Y.Z -m "vX.Y.Z"
   git push origin vX.Y.Z
   ```

5. The `Release` workflow requires `docs/releases/<tag>.md` and passes it to GoReleaser via `--release-notes`; pushing the tag builds all platform archives and publishes the GitHub Release with those notes.
6. Verify `https://github.com/MoonCC233/MCLauncher-TUI/releases/tag/vX.Y.Z` shows the notes and every expected asset.
7. If GitHub returns 5xx during push or API access, retry with `git -c http.version=HTTP/1.1`.

Never publish a release without update details on the Release page.
