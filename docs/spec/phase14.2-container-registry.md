# Phase 14.2 -- OCI Container / Artifact Registry (OCIR) Integration

## 1. Overview

This phase ports the Java OCIR integration to Go. The Java implementation
consists of a single utility class (`OcirUtils`) that provides container
repository and image management via the OCI Artifacts SDK. The Java code
has five methods: list repositories, list images, delete image, delete
repository, and a cleanup utility that retains only the N most recent
images. There are no controllers, services, or DTOs -- the class is
currently unused.

The Go implementation will provide a complete OCIR management API:
listing repositories, listing images, getting image details, deleting
images, deleting repositories, and retention-based cleanup.

---

## 2. Database Schema

No new tables are required. Container repositories and images are OCI
managed resources. The API reads directly from the OCI Artifacts service.

---

## 3. Existing Go Infrastructure

### 3.1 Already Exists

- **Provider pattern**: `internal/oci/provider.go` -- `Clients` struct,
  `NewClients(prov)`, `NewClientsWithHTTPClient(prov, hc)`
- **Proxy pattern**: `internal/oci/proxy.go` -- `WithProxy(ctx, pool, creds, masterKey, fn)`
- **Route registration**: `internal/httpapi/server.go` -- protected route groups
  with `auth.SessionAuth`, `auth.UserContext`, `auth.TenantContext`

### 3.2 NOT Yet Implemented

- `internal/oci/ocir.go` -- OCI Artifacts SDK wrapper
- `internal/service/ocir.go` -- OCIR service layer
- `internal/httpapi/ocir.go` -- HTTP handlers
- `go.mod` dependency: `github.com/oracle/oci-go-sdk/v65/artifacts`

---

## 4. OCI SDK Operations Required

The Go OCI SDK package `github.com/oracle/oci-go-sdk/v65/artifacts` provides
the `ArtifactsClient` with the following operations.

### 4.1 New Client Addition to `oci.Clients`

```go
type Clients struct {
    // ... existing fields ...

    // Phase 14.2: Container / Artifact Registry
    Artifacts *artifacts.ArtifactsClient  // github.com/oracle/oci-go-sdk/v65/artifacts
}
```

### 4.2 Container Repository Operations

| Operation              | OCI SDK Call                                    | Java Method                       |
|------------------------|------------------------------------------------|-----------------------------------|
| List repositories      | `ArtifactsClient.ListContainerRepositories`    | `OcirUtils.listRepositories`      |
| Get repository         | `ArtifactsClient.GetContainerRepository`       | (not in Java -- new)              |
| Delete repository      | `ArtifactsClient.DeleteContainerRepository`    | `OcirUtils.deleteRepository`      |

**ListContainerRepositoriesRequest** fields:
- `CompartmentId` (string) -- compartment OCID
- `LifecycleState` (string) -- filter by state, e.g. "AVAILABLE"
- `Limit` (int) -- page size
- `Page` (string) -- pagination token
- `SortBy` (enum) -- `DISPLAYNAME`, `TIMECREATED`
- `SortOrder` (enum) -- `ASC`, `DESC`

**ContainerRepositorySummary** response fields:
- `Id` (string) -- repository OCID
- `CompartmentId` (string)
- `DisplayName` (string) -- repository name
- `NamespaceName` (string) -- OCI tenancy namespace
- `RepositoryName` (string) -- full repository path
- `ImageCount` (int) -- number of images
- `IsPublic` (bool) -- public visibility
- `LifecycleState` (string)
- `TimeCreated` (time.Time)
- `TimeLastPushed` (time.Time) -- last push timestamp

### 4.3 Container Image Operations

| Operation              | OCI SDK Call                                    | Java Method                       |
|------------------------|------------------------------------------------|-----------------------------------|
| List images            | `ArtifactsClient.ListContainerImages`          | `OcirUtils.listImages`            |
| Get image              | `ArtifactsClient.GetContainerImage`            | (not in Java -- new)              |
| Delete image           | `ArtifactsClient.DeleteContainerImage`         | `OcirUtils.deleteImage`           |
| Restore image          | `ArtifactsClient.RestoreContainerImage`        | (not in Java -- new)              |

**ListContainerImagesRequest** fields:
- `CompartmentId` (string) -- compartment OCID
- `RepositoryName` (string) -- filter by repository
- `ImageId` (string) -- filter by specific image OCID
- `Version` (string) -- filter by tag/version
- `SortBy` (enum) -- `TIMECREATED`, `VERSION`
- `SortOrder` (enum) -- `ASC`, `DESC`
- `Limit` (int) -- page size
- `Page` (string) -- pagination token

**ContainerImageSummary** response fields:
- `Id` (string) -- image OCID
- `CompartmentId` (string)
- `RepositoryName` (string)
- `DisplayName` (string) -- image digest or tag
- `Version` (string) -- image tag (e.g. "latest", "v1.0")
- `LayersSizeInBytes` (int64) -- total layer size
- `SizeInBytes` (int64) -- total image size
- `ManifestSizeInBytes` (int64)
- `LifecycleState` (string) -- `AVAILABLE`, `DELETED`, `DELETING`
- `TimeCreated` (time.Time)
- `TimeLastPulled` (time.Time) -- last pull timestamp

### 4.4 Java Method Mapping

The Java `OcirUtils` implements these five operations:

| Java Method          | SDK Call                            | Go Function                         |
|----------------------|-------------------------------------|-------------------------------------|
| `listRepositories`   | `listContainerRepositories`         | `ListContainerRepositories`         |
| `listImages`         | `listContainerImages`               | `ListContainerImages`               |
| `deleteImage`        | `deleteContainerImage`              | `DeleteContainerImage`              |
| `deleteRepository`   | `deleteContainerRepository`         | `DeleteContainerRepository`         |
| `cleanupOldImages`   | `listImages` + `deleteImage` loop   | `CleanupOldImages`                  |

Java-specific behaviors to replicate:
- `listRepositories` filters by `LifecycleState = AVAILABLE`
- `listImages` sorts by `TimeCreated DESC` (newest first)
- `deleteImage` treats 404 as success (already deleted)
- `deleteRepository` treats 404 as success (already deleted)
- `cleanupOldImages` sorts images newest-first, keeps first N, deletes the rest

---

## 5. Go API Design

### 5.1 Routes

All routes are protected (SessionAuth + UserContext + TenantContext).

```
GET    /api/v1/tenants/:id/ocir/repositories                     -- List repositories
GET    /api/v1/tenants/:id/ocir/repositories/:repoName/images     -- List images in repo
GET    /api/v1/tenants/:id/ocir/repositories/:repoName/images/:imageId -- Get image details
DELETE /api/v1/tenants/:id/ocir/repositories/:repoName/images/:imageId -- Delete image
DELETE /api/v1/tenants/:id/ocir/repositories/:repoId              -- Delete repository
POST   /api/v1/tenants/:id/ocir/repositories/:repoName/cleanup    -- Retention cleanup
```

### 5.2 Query Parameters

**List Repositories:**
- `compartmentId` (required) -- compartment OCID to search in
- `limit` (optional, default 100) -- page size
- `page` (optional) -- pagination token

**List Images:**
- `compartmentId` (required) -- compartment OCID
- `version` (optional) -- filter by tag
- `sortBy` (optional, default "timeCreated") -- sort field
- `sortOrder` (optional, default "DESC") -- sort direction
- `limit` (optional, default 100) -- page size
- `page` (optional) -- pagination token

**Cleanup:**
- `compartmentId` (required) -- compartment OCID
- `keepCount` (required) -- number of recent images to retain

### 5.3 Request/Response DTOs

**List Repositories Response:**
```json
{
  "repositories": [
    {
      "id": "ocid1.containerrepo.oc1...",
      "compartmentId": "ocid1.compartment.oc1...",
      "displayName": "my-app",
      "namespaceName": "mytenancy",
      "repositoryName": "mytenancy/my-app",
      "imageCount": 5,
      "isPublic": false,
      "lifecycleState": "AVAILABLE",
      "timeCreated": "2025-01-01T00:00:00Z",
      "timeLastPushed": "2025-06-01T12:00:00Z"
    }
  ],
  "page": "ey..."
}
```

**List Images Response:**
```json
{
  "images": [
    {
      "id": "ocid1.containerimage.oc1...",
      "compartmentId": "ocid1.compartment.oc1...",
      "repositoryName": "mytenancy/my-app",
      "displayName": "sha256:abc123...",
      "version": "v1.2.3",
      "layersSizeInBytes": 52428800,
      "sizeInBytes": 62914560,
      "lifecycleState": "AVAILABLE",
      "timeCreated": "2025-06-01T12:00:00Z",
      "timeLastPulled": "2025-06-02T08:00:00Z"
    }
  ],
  "page": "ey..."
}
```

**Cleanup Response:**
```json
{
  "repositoryName": "mytenancy/my-app",
  "totalImages": 15,
  "keepCount": 5,
  "deletedCount": 10,
  "deletedImageIds": [
    "ocid1.containerimage.oc1...",
    "..."
  ]
}
```

---

## 6. File Structure

```
internal/
  oci/
    ocir.go             # OCI Artifacts SDK wrapper functions
  service/
    ocir.go             # OCIR service layer
  httpapi/
    ocir.go             # HTTP handlers for OCIR endpoints
```

---

## 7. Implementation Notes

### 7.1 `oci/ocir.go` Functions

```
ListContainerRepositories(ctx, client *artifacts.ArtifactsClient, compartmentID string) ([]ContainerRepositorySummary, error)
GetContainerRepository(ctx, client *artifacts.ArtifactsClient, repositoryID string) (*ContainerRepository, error)
DeleteContainerRepository(ctx, client *artifacts.ArtifactsClient, repositoryID string) error
ListContainerImages(ctx, client *artifacts.ArtifactsClient, compartmentID, repositoryName string, opts ListImagesOptions) ([]ContainerImageSummary, error)
GetContainerImage(ctx, client *artifacts.ArtifactsClient, imageID string) (*ContainerImage, error)
DeleteContainerImage(ctx, client *artifacts.ArtifactsClient, imageID string) error
CleanupOldImages(ctx, client *artifacts.ArtifactsClient, compartmentID, repositoryName string, keepCount int) (*CleanupResult, error)
```

### 7.2 Proxy Integration

OCIR operations should go through the existing `WithProxy` decorator:

```go
WithProxy(ctx, pool, creds, masterKey, func(c Clients) error {
    repos, err := oci.ListContainerRepositories(ctx, c.Artifacts, compartmentID)
    // ...
})
```

### 7.3 Parity with Java

| Java Class / Method                 | Go Equivalent                          |
|-------------------------------------|----------------------------------------|
| `OcirUtils.listRepositories`        | `oci.ListContainerRepositories`        |
| `OcirUtils.listImages`              | `oci.ListContainerImages`              |
| `OcirUtils.deleteImage`             | `oci.DeleteContainerImage`             |
| `OcirUtils.deleteRepository`        | `oci.DeleteContainerRepository`        |
| `OcirUtils.cleanupOldImages`        | `oci.CleanupOldImages`                 |
| (no controller)                     | `httpapi.OcirListRepositories`         |
| (no controller)                     | `httpapi.OcirListImages`               |
| (no controller)                     | `httpapi.OcirDeleteImage`              |
| (no controller)                     | `httpapi.OcirDeleteRepository`         |
| (no controller)                     | `httpapi.OcirCleanup`                  |

### 7.4 Key Considerations

- **Namespace**: OCI Container Registry uses the tenancy namespace as a prefix.
  The namespace can be retrieved via `ArtifactsClient.GetContainerConfiguration`
  or the `objectstorage.GetNamespace` call (already available in the Go project).
- **Pagination**: `ListContainerRepositories` and `ListContainerImages` both
  support `Limit` and `Page` parameters. Use the standard `OpcNextPage` loop.
- **Sorting**: Java code sorts images by `TimeCreated DESC`. The Go wrapper
  should default to the same sort order.
- **404 Handling**: Both `deleteImage` and `deleteRepository` in Java treat
  HTTP 404 as success. The Go wrapper should replicate this behavior.
- **Cleanup Atomicity**: The `CleanupOldImages` function iterates and deletes
  images one by one. If a delete fails mid-cleanup, the function should
  continue with remaining images and report the count of successful deletions.
- **Image Size Formatting**: The API returns sizes in bytes. The frontend
  can format to human-readable units (KB, MB, GB).
- **Pagination in Cleanup**: If a repository has more images than the API
  page limit, the cleanup function must paginate through all images before
  deciding which to delete. The Java code does not paginate (fetches one page),
  but the Go implementation should handle this correctly.
