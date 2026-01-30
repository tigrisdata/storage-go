## [0.4.1](https://github.com/tigrisdata/storage-go/compare/v0.4.0...v0.4.1) (2026-01-27)

### Bug Fixes

- **simplestorage:** use lower helper for safe pointer dereferencing ([#21](https://github.com/tigrisdata/storage-go/issues/21)) ([90c85ab](https://github.com/tigrisdata/storage-go/commit/90c85aba5209b5c2530cdd109cbce73528a71126))

# [0.4.0](https://github.com/tigrisdata/storage-go/compare/v0.3.0...v0.4.0) (2026-01-26)

### Features

- **simplestorage:** add Head method to Client ([#20](https://github.com/tigrisdata/storage-go/issues/20)) ([3366180](https://github.com/tigrisdata/storage-go/commit/3366180a41e086380e464cf31611db2d65702c1d))

# [0.3.0](https://github.com/tigrisdata/storage-go/compare/v0.2.0...v0.3.0) (2026-01-26)

### Features

- **simplestorage:** add ListResult type and enhance List with pagination ([#14](https://github.com/tigrisdata/storage-go/issues/14)) ([134cc0e](https://github.com/tigrisdata/storage-go/commit/134cc0e3755bb45e12726ce4f4958dbaeb9a8fd3))
- **storage:** suppress AWS SDK logging with Nop logger ([#15](https://github.com/tigrisdata/storage-go/issues/15)) ([d3f9338](https://github.com/tigrisdata/storage-go/commit/d3f9338d952997ae5b8423380dbea53a81867acb))

### BREAKING CHANGES

- **simplestorage:** List() signature changed from List(ctx, prefix, opts)
  to List(ctx, opts). Use WithPrefix(prefix) option instead.

Assisted-by: GLM 4.7 via Claude Code

Signed-off-by: Xe Iaso <xe@tigrisdata.com>

# [0.2.0](https://github.com/tigrisdata/storage-go/compare/v0.1.0...v0.2.0) (2026-01-15)

### Bug Fixes

- require Signed-off-by in commit messages ([#11](https://github.com/tigrisdata/storage-go/issues/11)) ([ee3d297](https://github.com/tigrisdata/storage-go/commit/ee3d29753ee72080586b0d9e83af062628db6826))

### Features

- add bucket management to simplestorage package ([#8](https://github.com/tigrisdata/storage-go/issues/8)) ([a8665a8](https://github.com/tigrisdata/storage-go/commit/a8665a8e020b16295b9c749ba710d210d885d1a4))
