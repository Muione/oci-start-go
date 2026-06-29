# Phase 11.1: Object Storage -- API SPEC

## Overview

This phase ports the OCI Object Storage management feature from the Java project to Go.
It covers bucket CRUD, object CRUD (upload/download/delete/preview), presigned URL generation
via Pre-Authenticated Requests (PAR), and a full multipart upload workflow with resumable
upload tracking in the database. A scheduled cleanup job aborts stale multipart uploads
older than 24 hours.

All endpoints are protected (SessionAuth + UserContext + TenantContext) and identified by
`tenantId` query/body parameter, which resolves to an OCI `ConfigurationProvider` for the
target tenancy.

## OCI Service

- **Service:** `objectstorage`
- **Go SDK:** `github.com/oracle/oci-go-sdk/v65/objectstorage`
- **Client field on `oci.Clients`:** `ObjectStorage *objectstorage.ObjectStorageClient`
  (already wired in `provider.go` line 78)

## API Endpoints to Implement

All routes mount under the protected group in `server.go`. Suggested prefix:
`/oci/storage` (matching Java `@RequestMapping("/oci/storage")`).

### 1. Namespace

| Method | Path | Description |
|--------|------|-------------|
| GET | `/oci/storage/namespace` | Get the tenancy's object-storage namespace |

**Query Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| tenantId | int64 | yes | DB tenant ID |

**Response (200):**

```json
{
  "success": true,
  "message": "success",
  "data": { "namespace": "abc123" },
  "code": 200
}
```

**OCI SDK Call:**

```go
resp, err := c.ObjectStorage.GetNamespace(ctx, objectstorage.GetNamespaceRequest{})
// resp.Value is the namespace string
```

**Java Reference:** `OciObjectStorageUtil.getNamespace()` (line 259-270),
controller `getNamespace` (line 514-532).

---

### 2. List Buckets (Paginated)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/oci/storage/buckets` | List buckets in the tenancy compartment |

**Query Parameters:**

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| tenantId | int64 | yes | | DB tenant ID |
| limit | int | no | 100 | Page size |
| pageToken | string | no | | Opaque next-page token (opc-next-page) |

**Response (200):**

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "name": "my-bucket",
        "namespace": "abc123",
        "timeCreated": "2025-01-01T00:00:00Z",
        "publicAccess": "NoPublicAccess"
      }
    ],
    "nextPage": "opaque-token-or-null"
  }
}
```

**OCI SDK Call:**

```go
resp, err := c.ObjectStorage.ListBuckets(ctx, objectstorage.ListBucketsRequest{
    NamespaceName: namespace,
    CompartmentId: common.String(compartmentId),
    Limit:         common.Int(limit),
    Page:          pageToken, // nil if empty
    Fields:        []objectstorage.ListBucketsFieldsEnum{objectstorage.ListBucketsFieldsTags},
})
```

**Business Logic:**
- `compartmentId` = `provider.GetTenancyOCID()` (the tenancy OCID is the default compartment).
- Extract `publicAccess` from `bucket.FreeformTags["accessType"]`; default `"NoPublicAccess"`.
- Existing Go `oci.ListBuckets()` (storage.go) does NOT paginate; replace with paginated version.

**Java Reference:** `OciObjectStorageUtil.listBucketsPaginated()` (line 850-877),
controller `listBuckets` (line 74-108).

---

### 3. Create Bucket

| Method | Path | Description |
|--------|------|-------------|
| POST | `/oci/storage/bucket/create` | Create a new bucket |

**Request Body (JSON):**

```json
{
  "tenantId": 1,
  "bucketName": "my-bucket",
  "publicAccessType": "NoPublicAccess"
}
```

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| tenantId | int64 | yes | | DB tenant ID |
| bucketName | string | yes | | Bucket name (trimmed) |
| publicAccessType | string | no | "NoPublicAccess" | One of: `NoPublicAccess`, `ObjectRead`, `ObjectReadWithoutList` |

**Response (200):**

```json
{ "success": true, "message": "存储桶创建成功" }
```

**OCI SDK Call:**

```go
accessType := objectstorage.CreateBucketDetailsPublicAccessTypeNoPublicAccess
// map string to enum ...

resp, err := c.ObjectStorage.CreateBucket(ctx, objectstorage.CreateBucketRequest{
    NamespaceName: namespace,
    CreateBucketDetails: objectstorage.CreateBucketDetails{
        Name:            common.String(bucketName),
        CompartmentId:   common.String(compartmentId),
        PublicAccessType: accessType,
        FreeformTags:    map[string]string{"accessType": publicAccessType},
    },
})
```

**Business Logic:**
- `compartmentId` = tenancy OCID.
- Store `accessType` in `FreeformTags` so `listBuckets` can read it back.

**Java Reference:** `OciObjectStorageUtil.createNamedBucket()` (line 358-392),
controller `createBucket` (line 113-128).

---

### 4. Delete Bucket

| Method | Path | Description |
|--------|------|-------------|
| POST | `/oci/storage/bucket/delete` | Delete a bucket (must be empty) |

**Request Body (JSON):**

```json
{
  "tenantId": 1,
  "namespace": "abc123",
  "bucketName": "my-bucket"
}
```

**OCI SDK Call:**

```go
_, err := c.ObjectStorage.DeleteBucket(ctx, objectstorage.DeleteBucketRequest{
    NamespaceName: namespace,
    BucketName:    common.String(bucketName),
})
```

**Java Reference:** `OciObjectStorageUtil.deleteNamedBucket()` (line 882-897),
controller `deleteBucket` (line 538-552).

---

### 5. List Objects (Paginated)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/oci/storage/objects` | List objects in a bucket |

**Query Parameters:**

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| tenantId | int64 | yes | | DB tenant ID |
| namespace | string | yes | | Bucket namespace |
| bucketName | string | yes | | Bucket name |
| prefix | string | no | "" | Object name prefix filter |
| limit | int | no | 100 | Page size |
| startToken | string | no | | Cursor for next page (`nextStartWith`) |

**Response (200):**

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "name": "path/to/file.txt",
        "size": 12345,
        "timeModified": "2025-01-01T00:00:00Z"
      }
    ],
    "nextStartWith": "next-cursor-or-null"
  }
}
```

**OCI SDK Call:**

```go
resp, err := c.ObjectStorage.ListObjects(ctx, objectstorage.ListObjectsRequest{
    NamespaceName: namespace,
    BucketName:    common.String(bucketName),
    Limit:         common.Int(limit),
    Prefix:        common.String(prefix),    // omit if empty
    Start:         common.String(startToken), // omit if empty
})
// resp.ListObjects.NextStartWith for pagination
```

**Java Reference:** `OciObjectStorageUtil.listObjectsPaginated()` (line 902-930),
controller `listObjects` (line 557-592).

---

### 6. Delete Object

| Method | Path | Description |
|--------|------|-------------|
| POST | `/oci/storage/object/delete` | Delete an object from a bucket |

**Request Body (JSON):**

```json
{
  "tenantId": 1,
  "namespace": "abc123",
  "bucketName": "my-bucket",
  "objectName": "path/to/file.txt"
}
```

**OCI SDK Call:**

```go
_, err := c.ObjectStorage.DeleteObject(ctx, objectstorage.DeleteObjectRequest{
    NamespaceName: namespace,
    BucketName:    common.String(bucketName),
    ObjectName:    common.String(objectName),
})
```

**Java Reference:** `OciObjectStorageUtil.deleteNamedObject()` (line 402-421),
controller `deleteObject` (line 162-177).

---

### 7. Upload Object (Single PUT)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/oci/storage/object/upload` | Upload a file via multipart form |

**Request:** `multipart/form-data`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| tenantId | string | yes | DB tenant ID |
| namespace | string | yes | Bucket namespace |
| bucketName | string | yes | Bucket name |
| objectName | string | no | Override object name (defaults to filename) |
| file | file | yes | File to upload |

**Response (200):**

```json
{ "success": true, "message": "上传成功" }
```

**OCI SDK Call:**

```go
_, err := c.ObjectStorage.PutObject(ctx, objectstorage.PutObjectRequest{
    NamespaceName: namespace,
    BucketName:    common.String(bucketName),
    ObjectName:    common.String(objectName),
    PutObjectBody: reader,           // io.Reader from uploaded file
    ContentLength: common.Int64(size),
    ContentType:   common.String(contentType), // from form, default "application/octet-stream"
})
```

**Business Logic:**
- If `objectName` is empty, use the uploaded filename.
- Content type defaults to `"application/octet-stream"` if not provided.

**Java Reference:** `OciObjectStorageUtil.uploadNamedObject()` (line 667-694),
controller `uploadObject` (line 206-235).

---

### 8. Download Object

| Method | Path | Description |
|--------|------|-------------|
| GET | `/oci/storage/object/download` | Download an object (browser triggers save) |

**Query Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| tenantId | int64 | yes | DB tenant ID |
| namespace | string | yes | Bucket namespace |
| bucketName | string | yes | Bucket name |
| objectName | string | yes | Object name |

**Response:** Binary stream with headers:
- `Content-Type`: from OCI response (default `application/octet-stream`)
- `Content-Disposition`: `attachment; filename*=UTF-8''<url-encoded-name>`
- `Content-Length`: from OCI response

**OCI SDK Call:**

```go
resp, err := c.ObjectStorage.GetObject(ctx, objectstorage.GetObjectRequest{
    NamespaceName: namespace,
    BucketName:    common.String(bucketName),
    ObjectName:    common.String(objectName),
})
// resp.Content, resp.ContentType, resp.ContentLength
```

**Business Logic:**
- Extract filename from objectName (last segment after `/`).
- URL-encode the filename for `Content-Disposition` header.
- Stream the response body to the HTTP response writer in 8KB chunks.

**Java Reference:** `OciObjectStorageUtil.downloadNamedObject()` (line 705-722),
controller `downloadObject` (line 240-278).

---

### 9. Preview Object

| Method | Path | Description |
|--------|------|-------------|
| GET | `/oci/storage/object/preview` | Preview an object inline (images/text/PDF/JSON/XML) |

**Query Parameters:** Same as Download.

**Response:** Binary stream with headers:
- `Content-Type`: resolved from OCI response or file extension
- `Content-Disposition`: `inline` for previewable types, `attachment` for others

**Business Logic:**
- Previewable content types: `image/*`, `text/*`, `*pdf*`, `*json*`, `*xml*`
- Resolve content type: prefer OCI response; if `application/octet-stream`, infer from extension:
  - `.png` -> `image/png`, `.jpg/.jpeg` -> `image/jpeg`, `.gif` -> `image/gif`
  - `.webp` -> `image/webp`, `.svg` -> `image/svg+xml`, `.pdf` -> `application/pdf`
  - `.json` -> `application/json`, `.txt/.log/.md` -> `text/plain;charset=UTF-8`
  - `.html/.htm` -> `text/html`, `.xml` -> `application/xml`

**Java Reference:** `controller.previewObject` (line 283-326), helper methods
`resolveContentType` (line 328-345), `isInlinePreviewable` (line 347-352).

---

### 10. Generate Presigned URL (PAR)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/oci/storage/object/presigned` | Generate a Pre-Authenticated Request URL |

**Request Body (JSON):**

```json
{
  "tenantId": 1,
  "namespace": "abc123",
  "bucketName": "my-bucket",
  "objectName": "path/to/file.txt",
  "validitySeconds": 3600
}
```

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| tenantId | int64 | yes | | DB tenant ID |
| namespace | string | yes | | Bucket namespace |
| bucketName | string | yes | | Bucket name |
| objectName | string | yes | | Object name |
| validitySeconds | int64 | no | 3600 | URL validity in seconds |

**Response (200):**

```json
{
  "success": true,
  "data": { "url": "https://objectstorage.region.oraclecloud.com/p/..." }
}
```

**OCI SDK Call:**

```go
expires := time.Now().Add(time.Duration(validitySeconds) * time.Second)
resp, err := c.ObjectStorage.CreatePreauthenticatedRequest(ctx,
    objectstorage.CreatePreauthenticatedRequestRequest{
        NamespaceName: namespace,
        BucketName:    common.String(bucketName),
        CreatePreauthenticatedRequestDetails: objectstorage.CreatePreauthenticatedRequestDetails{
            Name:       common.String(fmt.Sprintf("PAR-%d", time.Now().UnixMilli())),
            ObjectName: common.String(objectName),
            AccessType: objectstorage.CreatePreauthenticatedRequestDetailsAccessTypeObjectRead,
            TimeExpires: &common.SDKTime{Time: expires},
        },
    })
// Build full URL: client.Endpoint() + resp.PreauthenticatedRequest.AccessUri
```

**Java Reference:** `OciObjectStorageUtil.generatePresignedUrlForBucket()` (line 319-349),
controller `generatePresignedUrl` (line 182-201).

---

### 11. Initiate Multipart Upload

| Method | Path | Description |
|--------|------|-------------|
| POST | `/oci/storage/object/multipart/initiate` | Start a multipart upload session |

**Request Body (JSON):**

```json
{
  "tenantId": 1,
  "namespace": "abc123",
  "bucketName": "my-bucket",
  "objectName": "large-file.zip",
  "contentType": "application/octet-stream",
  "totalSize": 1073741824,
  "chunkSize": 10485760
}
```

**Response (200):**

```json
{
  "success": true,
  "data": {
    "uploadId": "xxx-yyy-zzz",
    "objectName": "large-file.zip",
    "namespace": "abc123",
    "bucketName": "my-bucket"
  }
}
```

**OCI SDK Call:**

```go
resp, err := c.ObjectStorage.CreateMultipartUpload(ctx,
    objectstorage.CreateMultipartUploadRequest{
        NamespaceName: namespace,
        BucketName:    common.String(bucketName),
        CreateMultipartUploadDetails: objectstorage.CreateMultipartUploadDetails{
            Object:      common.String(objectName),
            ContentType: common.String(contentType),
        },
    })
// resp.MultipartUpload.UploadId
```

**Business Logic:**
1. Look up active uploads for the same `(tenantId, bucketName, objectName)`.
2. For each active record, call OCI `AbortMultipartUpload` and mark DB record as `aborted`.
3. Call OCI `CreateMultipartUpload` to get a new `uploadId`.
4. Compute `totalParts = ceil(totalSize / chunkSize)`.
5. Insert a new `oci_multipart_upload_record` row with status `uploading`.

**Java Reference:** `OciObjectStorageUtil.initiateMultipartUpload()` (line 731-758),
controller `initiateMultipartUpload` (line 362-402).

---

### 12. Upload Part

| Method | Path | Description |
|--------|------|-------------|
| POST | `/oci/storage/object/multipart/part` | Upload a single part |

**Request:** `multipart/form-data`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| tenantId | string | yes | DB tenant ID |
| namespace | string | yes | Bucket namespace |
| bucketName | string | yes | Bucket name |
| objectName | string | yes | Object name |
| uploadId | string | yes | Upload session ID |
| partNumber | string | yes | Part number (1-based) |
| chunk | file | yes | Part data |

**Response (200):**

```json
{
  "success": true,
  "data": { "partNum": 1, "etag": "abc123..." }
}
```

**OCI SDK Call:**

```go
resp, err := c.ObjectStorage.UploadPart(ctx, objectstorage.UploadPartRequest{
    NamespaceName: namespace,
    BucketName:    common.String(bucketName),
    ObjectName:    common.String(objectName),
    UploadId:      common.String(uploadId),
    UploadPartNum: common.Int(partNumber),
    ContentLength: common.Int64(size),
    UploadPartBody: reader,
})
// resp.ETag
```

**Business Logic:**
- After successful upload, append `{partNum, etag}` to the `completed_parts` JSON array in DB.
- Deduplicate: remove existing entry with same `partNum` before appending.

**Java Reference:** `OciObjectStorageUtil.uploadPart()` (line 763-790),
controller `uploadPart` (line 407-438).

---

### 13. Commit Multipart Upload

| Method | Path | Description |
|--------|------|-------------|
| POST | `/oci/storage/object/multipart/commit` | Finalize the multipart upload |

**Request Body (JSON):**

```json
{
  "tenantId": 1,
  "namespace": "abc123",
  "bucketName": "my-bucket",
  "objectName": "large-file.zip",
  "uploadId": "xxx-yyy-zzz",
  "parts": [
    { "partNum": 1, "etag": "abc..." },
    { "partNum": 2, "etag": "def..." }
  ]
}
```

**Response (200):**

```json
{ "success": true, "message": "上传完成" }
```

**OCI SDK Call:**

```go
var parts []objectstorage.CommitMultipartUploadPartDetails
for _, p := range req.Parts {
    parts = append(parts, objectstorage.CommitMultipartUploadPartDetails{
        PartNum: common.Int(p.PartNum),
        Etag:    common.String(p.Etag),
    })
}
_, err := c.ObjectStorage.CommitMultipartUpload(ctx,
    objectstorage.CommitMultipartUploadRequest{
        NamespaceName: namespace,
        BucketName:    common.String(bucketName),
        ObjectName:    common.String(objectName),
        UploadId:      common.String(uploadId),
        CommitMultipartUploadDetails: objectstorage.CommitMultipartUploadDetails{
            PartsToCommit: parts,
        },
    })
```

**Business Logic:**
- On success, update DB record status to `completed`.

**Java Reference:** `OciObjectStorageUtil.commitMultipartUpload()` (line 795-820),
controller `commitMultipartUpload` (line 443-468).

---

### 14. Abort Multipart Upload

| Method | Path | Description |
|--------|------|-------------|
| POST | `/oci/storage/object/multipart/abort` | Cancel a multipart upload |

**Request Body (JSON):**

```json
{
  "tenantId": 1,
  "namespace": "abc123",
  "bucketName": "my-bucket",
  "objectName": "large-file.zip",
  "uploadId": "xxx-yyy-zzz"
}
```

**Response (200):**

```json
{ "success": true, "message": "已取消上传" }
```

**OCI SDK Call:**

```go
_, err := c.ObjectStorage.AbortMultipartUpload(ctx,
    objectstorage.AbortMultipartUploadRequest{
        NamespaceName: namespace,
        BucketName:    common.String(bucketName),
        ObjectName:    common.String(objectName),
        UploadId:      common.String(uploadId),
    })
```

**Business Logic:**
- On success, update DB record status to `aborted`.

**Java Reference:** `OciObjectStorageUtil.abortMultipartUpload()` (line 825-845),
controller `abortMultipartUpload` (line 473-492).

---

### 15. List Resumable Uploads

| Method | Path | Description |
|--------|------|-------------|
| GET | `/oci/storage/object/multipart/resumeable` | List resumable uploads for a bucket |

**Query Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| tenantId | int64 | yes | DB tenant ID |
| bucketName | string | yes | Bucket name |

**Response (200):**

```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "uploadId": "xxx-yyy-zzz",
      "objectName": "large-file.zip",
      "bucketName": "my-bucket",
      "namespace": "abc123",
      "totalSize": 1073741824,
      "chunkSize": 10485760,
      "totalParts": 103,
      "completedPartCount": 42,
      "completedParts": [
        { "partNum": 1, "etag": "abc..." },
        { "partNum": 2, "etag": "def..." }
      ],
      "createTime": "2025-04-01 12:00:00"
    }
  ]
}
```

**Business Logic:**
- Query DB for records with `status = 'uploading'` matching `(tenantId, bucketName)`.
- Parse `completed_parts` JSON column into the response array.

**Java Reference:** controller `listResumeableUploads` (line 497-509),
`OciMultipartUploadServiceImpl.listResumeableUploads()` (line 89-112).

---

## OCI SDK Operations Summary

| Operation | Go SDK Method | Key Request Fields | Key Response Fields |
|-----------|--------------|-------------------|-------------------|
| GetNamespace | `GetNamespace(ctx, GetNamespaceRequest{})` | (none) | `.Value` (string) |
| ListBuckets | `ListBuckets(ctx, ...)` | `NamespaceName`, `CompartmentId`, `Limit`, `Page`, `Fields` | `.Items` ([]BucketSummary), `.OpcNextPage` |
| CreateBucket | `CreateBucket(ctx, ...)` | `NamespaceName`, `CreateBucketDetails{Name, CompartmentId, PublicAccessType, FreeformTags}` | (void) |
| DeleteBucket | `DeleteBucket(ctx, ...)` | `NamespaceName`, `BucketName` | (void) |
| ListObjects | `ListObjects(ctx, ...)` | `NamespaceName`, `BucketName`, `Limit`, `Prefix`, `Start` | `.ListObjects.Objects` ([]ObjectSummary), `.ListObjects.NextStartWith` |
| PutObject | `PutObject(ctx, ...)` | `NamespaceName`, `BucketName`, `ObjectName`, `PutObjectBody`, `ContentLength`, `ContentType` | (void) |
| GetObject | `GetObject(ctx, ...)` | `NamespaceName`, `BucketName`, `ObjectName` | `.Content` (io.ReadCloser), `.ContentType`, `.ContentLength` |
| DeleteObject | `DeleteObject(ctx, ...)` | `NamespaceName`, `BucketName`, `ObjectName` | (void) |
| CreatePreauthenticatedRequest | `CreatePreauthenticatedRequest(ctx, ...)` | `NamespaceName`, `BucketName`, `CreatePreauthenticatedRequestDetails{Name, ObjectName, AccessType, TimeExpires}` | `.PreauthenticatedRequest.AccessUri` |
| CreateMultipartUpload | `CreateMultipartUpload(ctx, ...)` | `NamespaceName`, `BucketName`, `CreateMultipartUploadDetails{Object, ContentType}` | `.MultipartUpload.UploadId` |
| UploadPart | `UploadPart(ctx, ...)` | `NamespaceName`, `BucketName`, `ObjectName`, `UploadId`, `UploadPartNum`, `ContentLength`, `UploadPartBody` | `.ETag` |
| CommitMultipartUpload | `CommitMultipartUpload(ctx, ...)` | `NamespaceName`, `BucketName`, `ObjectName`, `UploadId`, `CommitMultipartUploadDetails{PartsToCommit}` | (void) |
| AbortMultipartUpload | `AbortMultipartUpload(ctx, ...)` | `NamespaceName`, `BucketName`, `ObjectName`, `UploadId` | (void) |

## Business Logic

### Multipart Upload Flow

```
Client                          Server                          OCI
  |                               |                              |
  |-- POST /multipart/initiate -->|                              |
  |                               |-- CreateMultipartUpload ---->|
  |                               |<-- uploadId ----------------|
  |                               |-- INSERT DB record          |
  |<-- {uploadId} ----------------|                              |
  |                               |                              |
  |-- POST /multipart/part (1) -->|                              |
  |                               |-- UploadPart (partNum=1) --->|
  |                               |<-- etag --------------------|
  |                               |-- UPDATE DB completed_parts  |
  |<-- {partNum, etag} -----------|                              |
  |                               |                              |
  |-- POST /multipart/part (N) -->| ... repeat ...               |
  |                               |                              |
  |-- POST /multipart/commit ---->|                              |
  |                               |-- CommitMultipartUpload ---->|
  |                               |<-- success -----------------|
  |                               |-- UPDATE DB status=completed |
  |<-- "上传完成" ----------------|                              |
```

### Deduplication on Initiate

Before creating a new multipart upload for `(tenantId, bucketName, objectName)`:
1. Query DB for all records with `status = 'uploading'` matching the triple.
2. For each, call `AbortMultipartUpload` on OCI, then mark DB record as `aborted`.
3. This ensures at most one active upload per file per bucket.

### Resumable Upload Tracking

The `oci_multipart_upload_record` table tracks:
- Which file is being uploaded (namespace, bucket, object)
- The OCI `uploadId`
- Total size and chunk size for the client to compute part numbers
- A JSON array of completed `{partNum, etag}` pairs
- Status: `uploading` / `completed` / `aborted`

The client calls the resumeable endpoint on reconnect, skips already-completed parts,
and continues from the next part number.

### Presigned URL Generation

Uses OCI Pre-Authenticated Requests (PAR), NOT S3-style presigned URLs.
A PAR creates a temporary access URI on the bucket that allows anonymous read
access to a specific object for a limited time. The full URL is:
`{client_endpoint}{access_uri}`.

### Cleanup of Stale Uploads (Scheduler)

- **Schedule:** Daily at 02:00 (already registered in `scheduler.go` line 150-152).
- **Threshold:** Records with `status = 'uploading'` and `update_time < now - 24 hours`.
- **Action:** For each stale record:
  1. Resolve tenant via `tenantId` (fallback to `tenancyOcid` if tenant was deleted/reimported).
  2. Call `AbortMultipartUpload` on OCI.
  3. Mark DB record as `aborted`.
- **Current Go status:** The scheduler job exists as a stub (`multipartCleanupJob` in
  scheduler.go line 228-248). It must be fully implemented.

### Pagination

- **Buckets:** Uses OCI `Page` / `OpcNextPage` (opaque token pagination).
- **Objects:** Uses OCI `Start` / `NextStartWith` (cursor-based pagination).
- Both return `{items: [...], nextCursor: "..."}` to the frontend.

## Database Changes

### New Migration: `0005_multipart_upload.up.sql`

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

### New Migration: `0005_multipart_upload.down.sql`

```sql
DROP TABLE IF EXISTS oci_multipart_upload_record;
```

### New sqlc Query File: `internal/repo/queries/multipart_upload.sql`

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

**Note:** Add the `oci_multipart_upload_record` schema to the `schema` list in `sqlc.yaml`
and the query file to the `queries` list. After adding, run `sqlc generate` to produce
the Go code in `internal/repo/`.

## Go Implementation Notes

### Files to Create

| File | Purpose |
|------|---------|
| `internal/oci/objectstorage.go` | All OCI Object Storage SDK wrapper functions (replaces + expands `storage.go`) |
| `internal/httpapi/object_storage.go` | HTTP handler functions for all 15 endpoints |
| `internal/service/object_storage.go` | Service layer: multipart upload DB logic, tenant resolution |
| `internal/repo/queries/multipart_upload.sql` | sqlc queries for the multipart upload table |
| `migrations/0005_multipart_upload.up.sql` | Migration: create `oci_multipart_upload_record` |
| `migrations/0005_multipart_upload.down.sql` | Migration: drop table |

### Files to Modify

| File | Change |
|------|--------|
| `internal/httpapi/server.go` | Add 15 object-storage routes under the `pro` group |
| `internal/httpapi/deps.go` | Add `ObjectStorageSvc *service.ObjectStorageService` to `Deps` |
| `internal/scheduler/scheduler.go` | Implement `multipartCleanupJob()` fully (replace stub at line 228) |
| `internal/repo/models.go` | Auto-generated: will include `OciMultipartUploadRecord` struct (already present at line 375) |
| `sqlc.yaml` | Add new query file and schema file entries |
| `internal/oci/storage.go` | Deprecate or remove (replaced by `objectstorage.go`) |

### Existing Patterns to Follow

1. **Handler pattern** (from `tenant.go`): Factory function returns `gin.HandlerFunc`, closes over `*Deps`.
   Use `response.OK(c, ...)` and `response.Fail(c, httpStatus, msg)`.

2. **OCI wrapper pattern** (from `backup.go`): Functions take `(ctx context.Context, client *objectstorage.ObjectStorageClient, ...)` or `(ctx context.Context, c Clients, ...)`.

3. **Service layer pattern** (from `service.TenantService`): Accepts `*db.Store` and `*oci.ProxyPool` (or `MasterKey`). Builds OCI clients internally from tenant credentials.

4. **File upload handling** (from `tenantSave` in `tenant.go`): Use `c.FormFile("fieldName")` for multipart uploads.

5. **Scheduler pattern** (from `scheduler.go`): Register cron job in `registerJobs()`, implement as method on `*Scheduler`.

6. **Tenant resolution for multipart cleanup**: Follow the Java `resolveTenant` pattern -- try `tenantId` first, fall back to `tenancyOcid` lookup.

### Edge Cases from Java Source

1. **Deduplication on initiate:** Java aborts ALL active uploads for the same file before creating a new one. The Go version must do the same.

2. **Tenant delete+reimport:** The `tenancyOcid` column is a redundant backup. If a tenant is deleted and reimported (new DB ID), the cleanup job can still find the tenant by OCID and fix the record's `tenant_id`. This is the `resolveTenant` / `fixTenantId` logic.

3. **Content type resolution for preview:** The Java code has an explicit fallback chain when OCI returns `application/octet-stream`. Must replicate the extension-based inference.

4. **Filename encoding for download/preview:** Java uses `URLEncoder.encode(fileName, "UTF-8").replace("+", "%20")` for RFC 5987 `filename*=UTF-8''...` encoding. Go equivalent: `url.PathEscape(fileName)`.

5. **Stream buffering:** Java reads in 8KB chunks. Go should use `io.CopyBuffer` with a similar buffer size.

6. **Bucket creation FreeformTags:** Java stores `accessType` in `FreeformTags` so it can be read back during `listBuckets`. The Go version must do the same.

7. **`completedParts` JSON management:** The Java code deduplicates by `partNum` before appending. The Go service must do `removeIf(partNum matches)` then `append`.

## Java Reference

| File | Key Lines | Description |
|------|-----------|-------------|
| `oci-server/.../controller/OciObjectStorageController.java` | 1-593 | All 16 endpoints |
| `oci-server/.../utils/oracle/OciObjectStorageUtil.java` | 1-931 | All OCI SDK calls |
| `oci-server/.../service/OciMultipartUploadService.java` | 1-38 | Service interface |
| `oci-server/.../service/impl/OciMultipartUploadServiceImpl.java` | 1-167 | Service implementation |
| `oci-dao/.../entity/OciMultipartUploadRecord.java` | 1-75 | Entity definition |
| `oci-dao/.../repository/OciMultipartUploadRecordRepository.java` | 1-62 | Repository queries |
| `oci-server/.../config/task/MultipartUploadCleanupTask.java` | 1-66 | Cleanup task logic |
| `oci-server/.../job/MultipartUploadCleanupJob.java` | 1-37 | Quartz job wrapper |
| `oci-server/.../pojo/request/CreateBucketRequest.java` | 1-19 | Request DTO |
| `oci-server/.../pojo/request/DeleteBucketRequest.java` | 1-16 | Request DTO |
| `oci-server/.../pojo/request/DeleteObjectRequest.java` | 1-22 | Request DTO |
| `oci-server/.../pojo/request/PresignedUrlRequest.java` | 1-25 | Request DTO |
| `oci-server/.../pojo/request/InitiateMultipartUploadRequest.java` | 1-29 | Request DTO |
| `oci-server/.../pojo/request/CommitMultipartUploadRequest.java` | 1-36 | Request DTO |
| `oci-server/.../pojo/request/AbortMultipartUploadRequest.java` | 1-25 | Request DTO |
| `oci-server/.../pojo/response/BucketVO.java` | 1-14 | Response VO |
| `oci-server/.../pojo/response/ObjectVO.java` | 1-13 | Response VO |
| `oci-server/.../pojo/response/NamespaceVO.java` | 1-11 | Response VO |
| `oci-server/.../pojo/response/PresignedUrlVO.java` | 1-11 | Response VO |
| `oci-server/.../pojo/response/InitiateMultipartUploadVO.java` | 1-14 | Response VO |
| `oci-server/.../pojo/response/MultipartUploadRecordVO.java` | 1-31 | Response VO |
| `oci-server/.../pojo/response/UploadPartVO.java` | 1-12 | Response VO |

### Go Reference (Existing)

| File | Description |
|------|-------------|
| `internal/oci/storage.go` | Existing `ListBuckets` (non-paginated, to be replaced) |
| `internal/oci/provider.go` | `Clients` struct with `ObjectStorage` field (line 78) |
| `internal/oci/backup.go` | Pattern for OCI SDK wrapper functions |
| `internal/httpapi/server.go` | Route registration pattern |
| `internal/httpapi/deps.go` | Dependency injection pattern |
| `internal/httpapi/tenant.go` | Handler + file upload pattern |
| `internal/scheduler/scheduler.go` | Scheduler with stub `multipartCleanupJob` (line 228) |
| `internal/response/response.go` | `ApiResponse` envelope |
| `internal/repo/models.go` | `OciMultipartUploadRecord` struct already defined (line 375) |
| `migrations/` | Migration file naming convention |
| `sqlc.yaml` | sqlc configuration |
