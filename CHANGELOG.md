# [0.7.0](https://github.com/tigrisdata/storage-go/compare/v0.6.0...v0.7.0) (2026-06-02)

- feat(simplestorage)!: achieve feature parity with TypeScript SDK ([#26](https://github.com/tigrisdata/storage-go/issues/26)) ([bb8c144](https://github.com/tigrisdata/storage-go/commit/bb8c144904087277f8b75be4d8d9bcada15ba6a9))
- feat(simplestorage)!: support iterators ([#27](https://github.com/tigrisdata/storage-go/issues/27)) ([3f6dd42](https://github.com/tigrisdata/storage-go/commit/3f6dd4202a2f63a2f756f630030865a06dcbd92c))

### BREAKING CHANGES

- Client.List now takes ListOption rather than
  ClientOption. The ClientOption helpers WithStartAfter, WithMaxKeys,
  WithDelimiter, WithPrefix, and WithPaginationToken have been removed.
  Migrate callers to the new ListOption equivalents; WithPaginationToken
  is now WithContinueToken.

Assisted-by: Claude Opus 4.7 via Claude Code
Signed-off-by: Xe Iaso <xe@tigrisdata.com>

- fix(simplestorage): if enriched bucket info can't be fetched, downgrade to simple bucket info

Signed-off-by: Xe Iaso <xe@tigrisdata.com>

- fix(simplestorage): use lower for truncation detection

Signed-off-by: Xe Iaso <xe@tigrisdata.com>

- refactor(simplestorage): remove dead list result types

BucketList, SnapshotList, and ListResult are no longer referenced
after List operations were converted to iterators. Drop the unused
types.

Assisted-by: Claude Opus 4.7 via Claude Code
Signed-off-by: Xe Iaso <xe@tigrisdata.com>

- fix(simplestorage): preserve Created when GrabForkInfo enriches bucket info

Info() returns a BucketInfo populated from HeadBucketForkOrSnapshot,
which has no creation timestamp. When Buckets() opted into
GrabForkInfo and Info() succeeded, the yielded BucketInfo dropped
the CreationDate that ListBuckets already provided, so callers
paradoxically lost Created by asking for more information.

Copy CreationDate from the ListBuckets response onto the enriched
BucketInfo before yielding, matching the GrabForkInfo=false and
Info() error fallback paths.

Assisted-by: Claude Opus 4.7 via Claude Code
Signed-off-by: Xe Iaso <xe@tigrisdata.com>

- fix(simplestorage): honor WithListLimit in Buckets iterator

The Buckets iterator hardcoded MaxBuckets to 50 via a local constant,
silently ignoring o.MaxKeys set by WithListLimit. Use o.MaxKeys when
provided, falling back to 50 as the default page size.

Assisted-by: Claude Opus 4.7 via Claude Code
Signed-off-by: Xe Iaso <xe@tigrisdata.com>

- storage.Client.CreateBucketSnapshot now returns
  *CreateBucketSnapshotOutput instead of *s3.CreateBucketOutput. The
  new type embeds *s3.CreateBucketOutput so existing field access
  still works; callers that bound the return value to
  *s3.CreateBucketOutput must re-type their variables.
- simplestorage.Client.List now returns \*ListResult
  instead of []Object to carry CommonPrefixes, PaginationToken, and
  HasMore.

Assisted-by: Claude Opus 4.7 via Claude Code
Signed-off-by: Xe Iaso <xe@tigrisdata.com>

- fix(simplestorage): default bucket Access to private

BucketOptions.defaults() left Access as the empty zero value, which
made bucketACL emit no canned ACL header. The WithBucketAccess
docstring already documented private as the default, so align the
implementation with the contract: seed Access in defaults() and have
bucketACL fall through to BucketCannedACLPrivate so unset values still
land on the wire as private.

Add a regression test that asserts the default.

Signed-off-by: Xe Iaso <xe@tigrisdata.com>
Assisted-by: Claude Opus 4.7 via Claude Code
Signed-off-by: Xe Iaso <xe@tigrisdata.com>

- fix(simplestorage): handle rand.Read error in generateRandomSuffix

Two issues:

- rand.Read could return an error that the previous code silently
  dropped, leaving the suffix all-zeros if entropy failed.
- length/2 truncated for odd lengths, producing a hex string shorter
  than the requested length before slicing.

Return (string, error), use (length+1)/2 bytes so the hex output is
always at least length characters before truncation, and propagate
the error from Put with bucket/key context.

Signed-off-by: Xe Iaso <xe@tigrisdata.com>
Assisted-by: Claude Opus 4.7 via Claude Code
Signed-off-by: Xe Iaso <xe@tigrisdata.com>

# [0.6.0](https://github.com/tigrisdata/storage-go/compare/v0.5.0...v0.6.0) (2026-04-06)

### Features

- add Bundle API support for streaming multi-object tar download ([1a89e45](https://github.com/tigrisdata/storage-go/commit/1a89e45f0232b0377f32bc41b730bf648ce9f06e))

# [0.5.0](https://github.com/tigrisdata/storage-go/compare/v0.4.1...v0.5.0) (2026-01-30)

### Features

- **simplestorage:** add For method to create bucket-scoped client copies ([#23](https://github.com/tigrisdata/storage-go/issues/23)) ([a64539b](https://github.com/tigrisdata/storage-go/commit/a64539b180f5bd060cf67d4b095519f177140d8c))
- **simplestorage:** add presigned URL API ([#22](https://github.com/tigrisdata/storage-go/issues/22)) ([0271427](https://github.com/tigrisdata/storage-go/commit/02714270b6c2682484be10d27f2f6decb8d4dc7b))

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
