# Phase 11.1: Object Storage -- Task Breakdown

> SPEC: `docs/spec/phase11.1-object-storage.md`
> Pattern references: `internal/httpapi/tenant.go`, `internal/service/tenant.go`, `internal/oci/provider.go`

---

## Dependency Graph

```
T1 (migration .up.sql)
  |
T2 (migration .down.sql)
  |
T3 (sqlc queries)
  |
T4 (sqlc.yaml update + sqlc generate)
  |
T5 (OCI wrapper: internal/oci/objectstorage.go)
  |
T6 (service layer: internal/service/object_storage.go)  <-- depends on T4, T5
  |
T7 (scheduler: multipartCleanupJob)  <-- depends on T6
  |
T8 (HTTP handlers: internal/httpapi/object_storage.go)  <-- depends on T6
  |
T9 (deps.go + route registration)  <-- depends on T8
  |
T10 (deprecate internal/oci/storage.go)  <-- depends on T5
  |
T11 (frontend: ObjectStorage.vue)  <-- depends on T9
  |
T12 (frontend: router registration)  <-- depends on T11
```

---

## Task 1: Database Migration -- Create `oci_multipart_upload_record` Table

**File:** `migrations/0005_multipart_upload.up.sql` (new)

```sql
-- Multipart upload tracking table (parity with Java oci_multipart_upload_record)
CREATE TABLE IF NOT EXISTS oci_multipart_upload_record (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id       INTEGER NOT NULL,
    cloud_type      INTEGER DEFAULT 1,          -- 1=OCI, 2=GCP, 3=Azure, 4=AWS
    tenancy_ocid    TEXT,                        -- redundant: survives tenant delete+reimport
    namespace       TEXT,
    bucket_name     TEXT NOT NULL,
    object_name     TEXT NOT NULL,
    upload_id       TEXT NOT NULL,
    total_size      INTEGER,
    chunk_size      INTEGER,
    total_parts     INTEGER,
    completed_parts TEXT DEFAULT '[]',           -- JSON: [{"partNum":1,"etag":"abc"},...]
    status          TEXT DEFAULT 'uploading',    -- uploading | completed | aborted
    create_time     TEXT,
    update_time     TEXT
);

CREATE INDEX IF NOT EXISTS idx_multipart_upload_tenant_bucket
    ON oci_multipart_upload_record(tenant_id, bucket_name, status);

CREATE INDEX IF NOT EXISTS idx_multipart_upload_upload_id
    ON oci_multipart_upload_record(upload_id);

CREATE INDEX IF NOT EXISTS idx_multipart_upload_stale
    ON oci_multipart_upload_record(status, update_time);
```

**Notes:**
- Follows the naming convention of existing migrations (`0001` through `0004`).
- The `OciMultipartUploadRecord` Go struct already exists in `internal/repo/models.go` (line 375) and matches this schema.

---

## Task 2: Database Migration -- Drop Table

**File:** `migrations/0005_multipart_upload.down.sql` (new)

```sql
DROP TABLE IF EXISTS oci_multipart_upload_record;
```

---

## Task 3: sqlc Queries for Multipart Upload

**File:** `internal/repo/queries/multipart_upload.sql` (new)

```sql
-- name: CreateMultipartUploadRecord :exec
INSERT INTO oci_multipart_upload_record
    (tenant_id, cloud_type, tenancy_ocid, namespace, bucket_name, object_name,
     upload_id, total_size, chunk_size, total_parts, completed_parts, status,
     create_time, update_time)
VALUES (?, 1, ?, ?, ?, ?, ?, ?, ?, ?, '[]', 'uploading', ?, ?);

-- name: FindActiveUploads :many
SELECT * FROM oci_multipart_upload_record
WHERE tenant_id = ? AND bucket_name = ? AND object_name = ? AND status = 'uploading';

-- name: FindByUploadId :one
SELECT * FROM oci_multipart_upload_record WHERE upload_id = ?;

-- name: ListResumableUploads :many
SELECT * FROM oci_multipart_upload_record
WHERE tenant_id = ? AND bucket_name = ? AND status = 'uploading'
ORDER BY create_time DESC;

-- name: UpdateMultipartUploadParts :exec
UPDATE oci_multipart_upload_record
SET completed_parts = ?, update_time = ?
WHERE upload_id = ?;

-- name: UpdateMultipartUploadStatus :exec
UPDATE oci_multipart_upload_record
SET status = ?, update_time = ?
WHERE upload_id = ?;

-- name: FindStaleUploads :many
SELECT * FROM oci_multipart_upload_record
WHERE status = 'uploading' AND update_time < ?;

-- name: FixMultipartUploadTenantId :exec
UPDATE oci_multipart_upload_record
SET tenant_id = ?, update_time = ?
WHERE tenancy_ocid = ? AND tenant_id != ?;

-- name: DeleteMultipartUploadsByTenant :exec
DELETE FROM oci_multipart_upload_record WHERE tenant_id = ?;
```

**Notes:**
- Follows the query style of existing files like `internal/repo/queries/tenant.sql`.
- The generated Go code will land in `internal/repo/multipart_upload.sql.go`.

---

## Task 4: Update sqlc.yaml and Run `sqlc generate`

**File:** `sqlc.yaml` (modify)

Add to the `queries` list:
```yaml
      - internal/repo/queries/multipart_upload.sql
```

Add to the `schema` list:
```yaml
      - migrations/0005_multipart_upload.up.sql
```

Then run:
```bash
cd /home/ubuntu/workspace-oci-start-rewrite/oci-start-go && sqlc generate
```

**Verification:** `internal/repo/multipart_upload.sql.go` is generated with methods: `CreateMultipartUploadRecord`, `FindActiveUploads`, `FindByUploadId`, `ListResumableUploads`, `UpdateMultipartUploadParts`, `UpdateMultipartUploadStatus`, `FindStaleUploads`, `FixMultipartUploadTenantId`, `DeleteMultipartUploadsByTenant`.

---

## Task 5: OCI Object Storage SDK Wrapper

**File:** `internal/oci/objectstorage.go` (new)

**Pattern:** Follow `internal/oci/backup.go` -- functions take `(ctx context.Context, client *objectstorage.ObjectStorageClient, ...)` or `(ctx context.Context, c Clients, ...)`.

**Imports:**
```go
import (
    "context"
    "fmt"
    "io"
    "time"

    "github.com/oracle/oci-go-sdk/v65/common"
    "github.com/oracle/oci-go-sdk/v65/objectstorage"
)
```

**Functions to implement:**

```go
// GetNamespace returns the tenancy's object-storage namespace string.
func GetNamespace(ctx context.Context, client *objectstorage.ObjectStorageClient) (string, error)

// ListBucketsPaginated lists buckets with pagination support.
// Returns items and the opaque next-page token (opc-next-page).
func ListBucketsPaginated(ctx context.Context, client *objectstorage.ObjectStorageClient,
    namespace, compartmentID string, limit int, pageToken string,
) ([]objectstorage.BucketSummary, *string, error)

// CreateBucket creates a new bucket. Stores publicAccessType in FreeformTags.
func CreateBucket(ctx context.Context, client *objectstorage.ObjectStorageClient,
    namespace, compartmentID, bucketName, publicAccessType string) error

// DeleteBucket deletes a bucket (must be empty).
func DeleteBucket(ctx context.Context, client *objectstorage.ObjectStorageClient,
    namespace, bucketName string) error

// ListObjectsPaginated lists objects in a bucket with cursor-based pagination.
func ListObjectsPaginated(ctx context.Context, client *objectstorage.ObjectStorageClient,
    namespace, bucketName, prefix string, limit int, startToken string,
) ([]objectstorage.ObjectSummary, *string, error)

// PutObject uploads a single object.
func PutObject(ctx context.Context, client *objectstorage.ObjectStorageClient,
    namespace, bucketName, objectName string, body io.Reader, size int64, contentType string) error

// GetObject downloads an object. Returns content reader, content type, and content length.
func GetObject(ctx context.Context, client *objectstorage.ObjectStorageClient,
    namespace, bucketName, objectName string) (io.ReadCloser, string, *int64, error)

// DeleteObject deletes an object from a bucket.
func DeleteObject(ctx context.Context, client *objectstorage.ObjectStorageClient,
    namespace, bucketName, objectName string) error

// CreatePresignedURL creates a Pre-Authenticated Request (PAR) and returns the full URL.
func CreatePresignedURL(ctx context.Context, client *objectstorage.ObjectStorageClient,
    namespace, bucketName, objectName string, validitySeconds int64) (string, error)

// CreateMultipartUpload initiates a multipart upload, returns the upload ID.
func CreateMultipartUpload(ctx context.Context, client *objectstorage.ObjectStorageClient,
    namespace, bucketName, objectName, contentType string) (string, error)

// UploadPart uploads a single part of a multipart upload. Returns the ETag.
func UploadPart(ctx context.Context, client *objectstorage.ObjectStorageClient,
    namespace, bucketName, objectName, uploadID string, partNumber int,
    body io.Reader, size int64) (string, error)

// CommitMultipartUpload finalizes a multipart upload.
func CommitMultipartUpload(ctx context.Context, client *objectstorage.ObjectStorageClient,
    namespace, bucketName, objectName, uploadID string,
    parts []objectstorage.CommitMultipartUploadPartDetails) error

// AbortMultipartUpload cancels a multipart upload.
func AbortMultipartUpload(ctx context.Context, client *objectstorage.ObjectStorageClient,
    namespace, bucketName, objectName, uploadID string) error
```

**Additional helper (for bucket public access type mapping):**

```go
// mapPublicAccessType maps a string to the SDK enum for CreateBucket.
func mapPublicAccessType(s string) objectstorage.CreateBucketDetailsPublicAccessTypeEnum

// mapListBucketsFields maps field name strings to SDK enum for ListBuckets.
func mapListBucketsFields(fields ...string) []objectstorage.ListBucketsFieldsEnum
```

---

## Task 6: Service Layer -- ObjectStorageService

**File:** `internal/service/object_storage.go` (new)

**Pattern:** Follow `internal/service/tenant.go` -- accepts `*db.Store` and `*oci.ProxyPool` (or `MasterKey`). Builds OCI clients internally from tenant credentials.

```go
package service

type ObjectStorageService struct {
    store     *db.Store
    masterKey []byte
    pool      *oci.ProxyPool
}

func NewObjectStorageService(store *db.Store, masterKey []byte, pool *oci.ProxyPool) *ObjectStorageService
```

**Methods (correspond to 15 API endpoints):**

```go
// GetNamespace returns the tenancy's object-storage namespace.
func (s *ObjectStorageService) GetNamespace(ctx context.Context, tenantID int64) (string, error)

// ListBuckets returns paginated buckets for the tenancy compartment.
func (s *ObjectStorageService) ListBuckets(ctx context.Context, tenantID int64, limit int, pageToken string) (*BucketListResult, error)

// CreateBucket creates a new bucket in the tenancy compartment.
func (s *ObjectStorageService) CreateBucket(ctx context.Context, tenantID int64, bucketName, publicAccessType string) error

// DeleteBucket deletes a bucket (must be empty).
func (s *ObjectStorageService) DeleteBucket(ctx context.Context, tenantID int64, namespace, bucketName string) error

// ListObjects returns paginated objects in a bucket.
func (s *ObjectStorageService) ListObjects(ctx context.Context, tenantID int64, namespace, bucketName, prefix string, limit int, startToken string) (*ObjectListResult, error)

// DeleteObject deletes an object from a bucket.
func (s *ObjectStorageService) DeleteObject(ctx context.Context, tenantID int64, namespace, bucketName, objectName string) error

// UploadObject uploads a file via PutObject.
func (s *ObjectStorageService) UploadObject(ctx context.Context, tenantID int64, namespace, bucketName, objectName string, body io.Reader, size int64, contentType string) error

// DownloadObject returns the object content stream + metadata for the HTTP handler to stream.
func (s *ObjectStorageService) DownloadObject(ctx context.Context, tenantID int64, namespace, bucketName, objectName string) (io.ReadCloser, string, *int64, error)

// PreviewObject returns the object content with resolved content type for inline preview.
func (s *ObjectStorageService) PreviewObject(ctx context.Context, tenantID int64, namespace, bucketName, objectName string) (io.ReadCloser, string, *int64, error)

// GeneratePresignedURL creates a PAR and returns the full URL.
func (s *ObjectStorageService) GeneratePresignedURL(ctx context.Context, tenantID int64, namespace, bucketName, objectName string, validitySeconds int64) (string, error)

// InitiateMultipartUpload starts a multipart upload, aborts existing active uploads for the same file.
func (s *ObjectStorageService) InitiateMultipartUpload(ctx context.Context, tenantID int64, namespace, bucketName, objectName, contentType string, totalSize, chunkSize int64) (*MultipartInitResult, error)

// UploadPart uploads a single part and updates the DB completed_parts JSON.
func (s *ObjectStorageService) UploadPart(ctx context.Context, tenantID int64, namespace, bucketName, objectName, uploadID string, partNumber int, body io.Reader, size int64) (*UploadPartResult, error)

// CommitMultipartUpload finalizes the upload and marks DB record as completed.
func (s *ObjectStorageService) CommitMultipartUpload(ctx context.Context, tenantID int64, namespace, bucketName, objectName, uploadID string, parts []CommitPart) error

// AbortMultipartUpload cancels the upload and marks DB record as aborted.
func (s *ObjectStorageService) AbortMultipartUpload(ctx context.Context, tenantID int64, namespace, bucketName, objectName, uploadID string) error

// ListResumableUploads returns active multipart uploads for a bucket.
func (s *ObjectStorageService) ListResumableUploads(ctx context.Context, tenantID int64, bucketName string) ([]ResumableUpload, error)

// CleanupStaleUploads is called by the scheduler. Aborts uploads older than 24 hours.
func (s *ObjectStorageService) CleanupStaleUploads(ctx context.Context) error
```

**Response types (in the same file or a types file):**

```go
type BucketListResult struct {
    Items    []BucketVO `json:"items"`
    NextPage *string    `json:"nextPage"`
}

type BucketVO struct {
    Name         string    `json:"name"`
    Namespace    string    `json:"namespace"`
    TimeCreated  time.Time `json:"timeCreated"`
    PublicAccess string    `json:"publicAccess"`
}

type ObjectListResult struct {
    Items         []ObjectVO `json:"items"`
    NextStartWith *string    `json:"nextStartWith"`
}

type ObjectVO struct {
    Name         string    `json:"name"`
    Size         int64     `json:"size"`
    TimeModified time.Time `json:"timeModified"`
}

type MultipartInitResult struct {
    UploadID   string `json:"uploadId"`
    ObjectName string `json:"objectName"`
    Namespace  string `json:"namespace"`
    BucketName string `json:"bucketName"`
}

type UploadPartResult struct {
    PartNum int    `json:"partNum"`
    Etag    string `json:"etag"`
}

type CommitPart struct {
    PartNum int    `json:"partNum"`
    Etag    string `json:"etag"`
}

type ResumableUpload struct {
    ID                int64        `json:"id"`
    UploadID          string       `json:"uploadId"`
    ObjectName        string       `json:"objectName"`
    BucketName        string       `json:"bucketName"`
    Namespace         string       `json:"namespace"`
    TotalSize         int64        `json:"totalSize"`
    ChunkSize         int64        `json:"chunkSize"`
    TotalParts        int          `json:"totalParts"`
    CompletedPartCount int         `json:"completedPartCount"`
    CompletedParts    []CommitPart `json:"completedParts"`
    CreateTime        string       `json:"createTime"`
}
```

**Internal helpers:**

```go
// resolveTenantFromID fetches a tenant by ID and returns OCI credentials + clients.
func (s *ObjectStorageService) resolveTenantFromID(ctx context.Context, tenantID int64) (oci.Credentials, oci.Clients, error)

// resolveContentType infers content type from file extension when OCI returns application/octet-stream.
func resolveContentType(ociContentType, objectName string) string

// isInlinePreviewable returns true if the content type should be displayed inline.
func isInlinePreviewable(contentType string) bool

// extractFilename extracts the filename from an object path (last segment after /).
func extractFilename(objectName string) string
```

**Key business logic:**
- `InitiateMultipartUpload`: Query DB for active uploads on `(tenantId, bucketName, objectName)`, abort each via OCI SDK, mark DB as `aborted`, then create new upload.
- `UploadPart`: After OCI upload succeeds, read existing `completed_parts` JSON from DB, deduplicate by `partNum`, append new `{partNum, etag}`, write back.
- `CleanupStaleUploads`: Query DB for `status = 'uploading' AND update_time < now - 24h`, for each: resolve tenant (try `tenantId` first, fall back to `tenancyOcid`), call OCI `AbortMultipartUpload`, mark DB as `aborted`.

---

## Task 7: Implement Scheduler Cleanup Job

**File:** `internal/scheduler/scheduler.go` (modify)

Replace the stub `multipartCleanupJob()` (lines 228-248) with a real implementation that:

1. Calls `s.objectStorageSvc.CleanupStaleUploads(ctx)`.
2. Logs results.

**Prerequisite:** Add `objectStorageSvc *service.ObjectStorageService` field to the `Scheduler` struct and `SvcSet`.

**Changes to `registerJobs()`:** No change needed -- the cron expression `0 0 2 * * *` is already registered at line 150-152.

**Updated stub:**

```go
func (s *Scheduler) multipartCleanupJob() {
    if s.objectStorageSvc == nil {
        s.logger.Debug().Msg("scheduler: MultipartUploadCleanupJob skipped — service not configured")
        return
    }
    ctx := context.Background()
    if err := s.objectStorageSvc.CleanupStaleUploads(ctx); err != nil {
        s.logger.Error().Err(err).Msg("scheduler: MultipartUploadCleanupJob failed")
    } else {
        s.logger.Info().Msg("scheduler: MultipartUploadCleanupJob completed")
    }
}
```

---

## Task 8: HTTP Handlers

**File:** `internal/httpapi/object_storage.go` (new)

**Pattern:** Follow `internal/httpapi/tenant.go` -- factory function returns `gin.HandlerFunc`, closes over `*Deps`. Use `response.OK(c, ...)` and `response.Fail(c, httpStatus, msg)`.

**Imports:**
```go
import (
    "fmt"
    "io"
    "net/http"
    "net/url"
    "path"
    "strconv"

    "github.com/gin-gonic/gin"
    "github.com/Muione/oci-start-go/internal/response"
)
```

**Handler functions (15 total):**

```go
// GET /oci/storage/namespace?tenantId=
func objectStorageNamespace(deps *Deps) gin.HandlerFunc

// GET /oci/storage/buckets?tenantId=&limit=&pageToken=
func objectStorageListBuckets(deps *Deps) gin.HandlerFunc

// POST /oci/storage/bucket/create
func objectStorageCreateBucket(deps *Deps) gin.HandlerFunc

// POST /oci/storage/bucket/delete
func objectStorageDeleteBucket(deps *Deps) gin.HandlerFunc

// GET /oci/storage/objects?tenantId=&namespace=&bucketName=&prefix=&limit=&startToken=
func objectStorageListObjects(deps *Deps) gin.HandlerFunc

// POST /oci/storage/object/delete
func objectStorageDeleteObject(deps *Deps) gin.HandlerFunc

// POST /oci/storage/object/upload (multipart/form-data)
func objectStorageUpload(deps *Deps) gin.HandlerFunc

// GET /oci/storage/object/download?tenantId=&namespace=&bucketName=&objectName=
func objectStorageDownload(deps *Deps) gin.HandlerFunc

// GET /oci/storage/object/preview?tenantId=&namespace=&bucketName=&objectName=
func objectStoragePreview(deps *Deps) gin.HandlerFunc

// POST /oci/storage/object/presigned
func objectStoragePresigned(deps *Deps) gin.HandlerFunc

// POST /oci/storage/object/multipart/initiate
func objectStorageMultipartInitiate(deps *Deps) gin.HandlerFunc

// POST /oci/storage/object/multipart/part (multipart/form-data)
func objectStorageMultipartPart(deps *Deps) gin.HandlerFunc

// POST /oci/storage/object/multipart/commit
func objectStorageMultipartCommit(deps *Deps) gin.HandlerFunc

// POST /oci/storage/object/multipart/abort
func objectStorageMultipartAbort(deps *Deps) gin.HandlerFunc

// GET /oci/storage/object/multipart/resumeable?tenantId=&bucketName=
func objectStorageMultipartResumable(deps *Deps) gin.HandlerFunc
```

**Request/Response structs (define in same file):**

```go
type createBucketReq struct {
    TenantID         int64  `json:"tenantId"`
    BucketName       string `json:"bucketName"`
    PublicAccessType string `json:"publicAccessType"`
}

type deleteBucketReq struct {
    TenantID   int64  `json:"tenantId"`
    Namespace  string `json:"namespace"`
    BucketName string `json:"bucketName"`
}

type deleteObjectReq struct {
    TenantID   int64  `json:"tenantId"`
    Namespace  string `json:"namespace"`
    BucketName string `json:"bucketName"`
    ObjectName string `json:"objectName"`
}

type presignedReq struct {
    TenantID        int64  `json:"tenantId"`
    Namespace       string `json:"namespace"`
    BucketName      string `json:"bucketName"`
    ObjectName      string `json:"objectName"`
    ValiditySeconds int64  `json:"validitySeconds"`
}

type multipartInitReq struct {
    TenantID   int64  `json:"tenantId"`
    Namespace  string `json:"namespace"`
    BucketName string `json:"bucketName"`
    ObjectName string `json:"objectName"`
    ContentType string `json:"contentType"`
    TotalSize  int64  `json:"totalSize"`
    ChunkSize  int64  `json:"chunkSize"`
}

type multipartCommitReq struct {
    TenantID   int64        `json:"tenantId"`
    Namespace  string       `json:"namespace"`
    BucketName string       `json:"bucketName"`
    ObjectName string       `json:"objectName"`
    UploadID   string       `json:"uploadId"`
    Parts      []commitPart `json:"parts"`
}

type commitPart struct {
    PartNum int    `json:"partNum"`
    Etag    string `json:"etag"`
}

type multipartAbortReq struct {
    TenantID   int64  `json:"tenantId"`
    Namespace  string `json:"namespace"`
    BucketName string `json:"bucketName"`
    ObjectName string `json:"objectName"`
    UploadID   string `json:"uploadId"`
}
```

**Download/Preview handler details:**
- Stream response body in 8KB chunks using `io.CopyBuffer`.
- Set `Content-Disposition: attachment; filename*=UTF-8''<url-encoded-name>` for download.
- Set `Content-Disposition: inline` for previewable types.
- URL-encode filename using `url.PathEscape`.

---

## Task 9: Dependency Injection and Route Registration

### 9a. Update `internal/httpapi/deps.go`

Add to the `Deps` struct:

```go
// Phase 11.1: Object Storage.
ObjectStorageSvc *service.ObjectStorageService
```

### 9b. Update `internal/httpapi/server.go`

Add the 15 object-storage routes under the `pro` group, before the SPA fallback:

```go
// Phase 11.1: Object Storage.
pro.GET("/oci/storage/namespace", objectStorageNamespace(deps))
pro.GET("/oci/storage/buckets", objectStorageListBuckets(deps))
pro.POST("/oci/storage/bucket/create", objectStorageCreateBucket(deps))
pro.POST("/oci/storage/bucket/delete", objectStorageDeleteBucket(deps))
pro.GET("/oci/storage/objects", objectStorageListObjects(deps))
pro.POST("/oci/storage/object/delete", objectStorageDeleteObject(deps))
pro.POST("/oci/storage/object/upload", objectStorageUpload(deps))
pro.GET("/oci/storage/object/download", objectStorageDownload(deps))
pro.GET("/oci/storage/object/preview", objectStoragePreview(deps))
pro.POST("/oci/storage/object/presigned", objectStoragePresigned(deps))
pro.POST("/oci/storage/object/multipart/initiate", objectStorageMultipartInitiate(deps))
pro.POST("/oci/storage/object/multipart/part", objectStorageMultipartPart(deps))
pro.POST("/oci/storage/object/multipart/commit", objectStorageMultipartCommit(deps))
pro.POST("/oci/storage/object/multipart/abort", objectStorageMultipartAbort(deps))
pro.GET("/oci/storage/object/multipart/resumeable", objectStorageMultipartResumable(deps))
```

### 9c. Wire `ObjectStorageService` in `main.go`

Add to the service construction section:

```go
objectStorageSvc := service.NewObjectStorageService(store, masterKey, proxyPool)
```

Pass to both `Deps` and `scheduler.SvcSet`.

---

## Task 10: Deprecate Existing `storage.go`

**File:** `internal/oci/storage.go` (modify)

The existing `ListBuckets` function (non-paginated) is superseded by `ListBucketsPaginated` in `objectstorage.go`.

Options:
1. Keep `storage.go` with a deprecation comment pointing to `objectstorage.go`.
2. Remove `storage.go` entirely if no other code calls `oci.ListBuckets`.

Check for callers:
```bash
grep -rn "oci.ListBuckets" internal/
```

If no callers, delete `storage.go`. Otherwise, mark as deprecated and update callers.

---

## Task 11: Frontend -- ObjectStorage.vue Page

**File:** `frontend/src/views/ObjectStorage.vue` (new)

**Features:**
- Bucket list with pagination (next page token).
- Create bucket dialog (name, publicAccessType dropdown).
- Delete bucket button (confirmation dialog).
- Click a bucket to list objects (breadcrumb navigation).
- Object list with pagination (startToken cursor).
- Upload file button (multipart/form-data to `/oci/storage/object/upload`).
- Download button per object (link to `/oci/storage/object/download`).
- Preview button for viewable types (modal or inline).
- Delete object button.
- Generate presigned URL button (shows URL in dialog).
- Multipart upload progress bar for large files (initiate -> upload parts -> commit).
- Resumable uploads panel (lists active uploads, allows resume).

**Pattern:** Follow existing views like `Tenants.vue` for table layout, dialogs, and API calls.

---

## Task 12: Frontend -- Router Registration

**File:** `frontend/src/router/index.ts` (modify)

Add inside the `children` array of the authenticated route group:

```ts
{ path: 'storage', name: 'storage', component: () => import('../views/ObjectStorage.vue') },
```

---

## Test Checklist

### Backend

- [ ] GET `/oci/storage/namespace` returns namespace string
- [ ] GET `/oci/storage/buckets` returns paginated bucket list with `nextPage`
- [ ] GET `/oci/storage/buckets?pageToken=X` fetches next page
- [ ] POST `/oci/storage/bucket/create` creates bucket with `FreeformTags.accessType`
- [ ] POST `/oci/storage/bucket/delete` deletes empty bucket
- [ ] POST `/oci/storage/bucket/delete` on non-empty bucket returns error
- [ ] GET `/oci/storage/objects` returns paginated object list with `nextStartWith`
- [ ] POST `/oci/storage/object/upload` uploads file via multipart form
- [ ] POST `/oci/storage/object/upload` with `objectName` override works
- [ ] GET `/oci/storage/object/download` streams file with correct `Content-Disposition`
- [ ] GET `/oci/storage/object/preview` streams inline for image/text/PDF
- [ ] GET `/oci/storage/object/preview` falls back to `attachment` for unknown types
- [ ] POST `/oci/storage/object/presigned` returns valid PAR URL
- [ ] POST `/oci/storage/object/delete` deletes object

### Multipart Upload

- [ ] POST `/oci/storage/object/multipart/initiate` creates upload session + DB record
- [ ] Initiating a new upload for the same file aborts the previous one
- [ ] POST `/oci/storage/object/multipart/part` uploads part and updates `completed_parts` JSON
- [ ] Uploading the same `partNum` twice deduplicates (replaces)
- [ ] POST `/oci/storage/object/multipart/commit` finalizes upload, DB status = `completed`
- [ ] POST `/oci/storage/object/multipart/abort` cancels upload, DB status = `aborted`
- [ ] GET `/oci/storage/object/multipart/resumeable` lists active uploads with correct part counts
- [ ] Scheduler cleanup job aborts uploads older than 24 hours
- [ ] Cleanup job handles deleted/reimported tenants via `tenancyOcid` fallback

### Edge Cases

- [ ] Tenant delete+reimport: multipart records fix `tenant_id` via `tenancyOcid`
- [ ] Content type fallback: OCI returns `application/octet-stream` -> infer from extension
- [ ] Filename encoding: download `Content-Disposition` uses RFC 5987 encoding
- [ ] Large file: stream in 8KB chunks (no memory blowup)
- [ ] Empty bucket name / namespace returns 400
- [ ] Unauthenticated request returns 401

### Frontend

- [ ] Storage page loads and displays bucket list
- [ ] Create bucket dialog works
- [ ] Delete bucket with confirmation
- [ ] Navigate into bucket, see object list
- [ ] Upload file shows progress
- [ ] Download file triggers browser save
- [ ] Preview image/text/PDF inline
- [ ] Generate presigned URL shows copyable link
- [ ] Multipart upload: progress bar, pause/resume
- [ ] Pagination: next page loads correctly
