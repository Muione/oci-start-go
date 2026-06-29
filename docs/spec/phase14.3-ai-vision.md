# Phase 14.3 -- OCI AI Vision Integration

## 1. Overview

This phase adds OCI AI Vision integration to the Go project. Unlike the
other Phase 14 features, the Java project has **no implementation code**
for AI Vision -- only an unused Maven dependency (`oci-java-sdk-aivision`)
was declared in `pom.xml`. No controllers, services, DTOs, or utility
classes exist for AI Vision in the Java codebase.

The Go implementation will provide a complete AI Vision API supporting
image analysis (object detection, text recognition, image classification),
document analysis, and video analysis via the OCI AI Vision service.

---

## 2. Database Schema

No new tables are required. AI Vision analysis results are returned
directly from the OCI API. If history/caching is desired in the future,
a `vision_analyses` table can be added.

---

## 3. Existing Go Infrastructure

### 3.1 Already Exists

- **Provider pattern**: `internal/oci/provider.go` -- `Clients` struct,
  `NewClients(prov)`, `NewClientsWithHTTPClient(prov, hc)`
- **Proxy pattern**: `internal/oci/proxy.go` -- `WithProxy(ctx, pool, creds, masterKey, fn)`
- **Route registration**: `internal/httpapi/server.go` -- protected route groups
  with `auth.SessionAuth`, `auth.UserContext`, `auth.TenantContext`
- **Object Storage**: `internal/oci/objectstorage.go` -- for uploading images
  before analysis (presigned URLs, multipart upload)

### 3.2 NOT Yet Implemented

- `internal/oci/aivision.go` -- OCI AI Vision SDK wrapper
- `internal/service/aivision.go` -- AI Vision service layer
- `internal/httpapi/aivision.go` -- HTTP handlers
- `go.mod` dependency: `github.com/oracle/oci-go-sdk/v65/aivision`

---

## 4. OCI SDK Operations Required

The Go OCI SDK package `github.com/oracle/oci-go-sdk/v65/aivision` provides
the `AIServiceVisionClient` with the following operations.

### 4.1 New Client Addition to `oci.Clients`

```go
type Clients struct {
    // ... existing fields ...

    // Phase 14.3: AI Vision
    AiVision *aivision.AIServiceVisionClient  // github.com/oracle/oci-go-sdk/v65/aivision
}
```

### 4.2 Image Analysis Operations

| Operation              | OCI SDK Call                                      | Java Reference |
|------------------------|--------------------------------------------------|----------------|
| Analyze image          | `AIServiceVisionClient.AnalyzeImage`             | (none)         |
| Get image analysis     | `AIServiceVisionClient.GetImageAnalysisResult`   | (none)         |

**AnalyzeImageRequest** fields:
- `AnalyzeImageDetails` -- one of the following:
  - `ImageClassificationFeature` -- classify image content
  - `ObjectDetectionFeature` -- detect and locate objects
  - `TextRecognitionFeature` -- OCR / text extraction
  - `ImageClassificationFeature` + `ObjectDetectionFeature` -- combined

**AnalyzeImageDetails** (inline image):
```go
type AnalyzeImageDetails struct {
    CompartmentId *string
    Image         *InlineImageDetails  // base64-encoded image
    Features      []ImageFeature
    OutputLocation *OutputLocation     // optional: save results to Object Storage
}
```

**InlineImageDetails** fields:
- `Data` (string) -- base64-encoded image data
- `Source` (string) -- "INLINE"

**ObjectStorageLocation** (for large images):
```go
type ObjectStorageLocation struct {
    NamespaceName *string
    BucketName    *string
    ObjectName    *string
}
```

### 4.3 Image Feature Types

| Feature                        | SDK Type                              | Purpose                           |
|--------------------------------|---------------------------------------|-----------------------------------|
| Image Classification           | `ImageClassificationFeature`          | Classify image content            |
| Object Detection               | `ObjectDetectionFeature`              | Detect and locate objects         |
| Text Recognition               | `TextRecognitionFeature`              | OCR / extract text from images    |

**Feature fields:**
- `MaxResults` (int) -- max number of results to return (default 10, max 100)

### 4.4 Document Analysis Operations

| Operation              | OCI SDK Call                                         | Java Reference |
|------------------------|-----------------------------------------------------|----------------|
| Analyze document       | `AIServiceVisionClient.AnalyzeDocument`             | (none)         |
| Get document analysis  | `AIServiceVisionClient.GetDocumentAnalysisResult`   | (none)         |

**AnalyzeDocumentDetails** fields:
- `CompartmentId` (string)
- `Features` -- document analysis features
- `Document` -- inline document or Object Storage reference
- `OutputLocation` -- optional: save results to Object Storage

**Document Feature Types:**

| Feature              | SDK Type                      | Purpose                          |
|----------------------|-------------------------------|----------------------------------|
| Document Table       | `DocumentTableDetectionFeature` | Extract tables from documents    |
| Document Language    | `DocumentLanguageClassificationFeature` | Detect document language  |
| Document Key Value   | `DocumentKeyValueExtractionFeature`    | Extract key-value pairs   |

### 4.5 Video Analysis Operations (Async)

| Operation              | OCI SDK Call                                       | Java Reference |
|------------------------|---------------------------------------------------|----------------|
| Analyze video          | `AIServiceVisionClient.AnalyzeVideo`              | (none)         |
| Get video analysis     | `AIServiceVisionClient.GetVideoAnalysisResult`    | (none)         |
| Cancel video analysis  | `AIServiceVisionClient.CancelVideoAnalysis`       | (none)         |

**AnalyzeVideoDetails** fields:
- `CompartmentId` (string)
- `Features` -- video analysis features
- `Video` -- Object Storage reference
- `OutputLocation` -- save results to Object Storage

Video analysis is asynchronous -- the API returns a `workRequestId` that
must be polled via `GetWorkRequest` until completion.

### 4.6 Model Management

| Operation              | OCI SDK Call                              | Java Reference |
|------------------------|------------------------------------------|----------------|
| List models            | `AIServiceVisionClient.ListModels`       | (none)         |
| Get model              | `AIServiceVisionClient.GetModel`         | (none)         |
| Create model           | `AIServiceVisionClient.CreateModel`      | (none)         |
| Delete model           | `AIServiceVisionClient.DeleteModel`      | (none)         |
| Update model           | `AIServiceVisionClient.UpdateModel`      | (none)         |

Custom models allow training on domain-specific images.

### 4.7 Project Management

| Operation              | OCI SDK Call                                | Java Reference |
|------------------------|--------------------------------------------|----------------|
| List projects          | `AIServiceVisionClient.ListProjects`       | (none)         |
| Get project            | `AIServiceVisionClient.GetProject`         | (none)         |
| Create project         | `AIServiceVisionClient.CreateProject`      | (none)         |
| Delete project         | `AIServiceVisionClient.DeleteProject`      | (none)         |
| Update project         | `AIServiceVisionClient.UpdateProject`      | (none)         |

---

## 5. Go API Design

### 5.1 Routes

All routes are protected (SessionAuth + UserContext + TenantContext).

```
POST   /api/v1/tenants/:id/aivision/analyze/image      -- Analyze image (inline base64)
POST   /api/v1/tenants/:id/aivision/analyze/image-obj   -- Analyze image from Object Storage
POST   /api/v1/tenants/:id/aivision/analyze/document    -- Analyze document
POST   /api/v1/tenants/:id/aivision/analyze/video       -- Start video analysis (async)
GET    /api/v1/tenants/:id/aivision/analyze/video/:workRequestId -- Get video analysis status
DELETE /api/v1/tenants/:id/aivision/analyze/video/:workRequestId -- Cancel video analysis
```

### 5.2 Request/Response DTOs

**Analyze Image Request (inline):**
```json
{
  "compartmentId": "ocid1.compartment.oc1...",
  "imageData": "base64-encoded-image-data",
  "features": ["CLASSIFICATION", "OBJECT_DETECTION", "TEXT_RECOGNITION"],
  "maxResults": 10
}
```

**Analyze Image Request (Object Storage):**
```json
{
  "compartmentId": "ocid1.compartment.oc1...",
  "namespaceName": "mytenancy",
  "bucketName": "my-bucket",
  "objectName": "images/photo.jpg",
  "features": ["CLASSIFICATION", "OBJECT_DETECTION"],
  "maxResults": 20
}
```

**Analyze Image Response:**
```json
{
  "imageClassificationLabels": [
    {
      "name": "cat",
      "confidence": 0.97
    },
    {
      "name": "animal",
      "confidence": 0.95
    }
  ],
  "detectedObjects": [
    {
      "name": "cat",
      "confidence": 0.93,
      "boundingPolygon": {
        "normalizedVertices": [
          { "x": 0.1, "y": 0.2 },
          { "x": 0.5, "y": 0.2 },
          { "x": 0.5, "y": 0.8 },
          { "x": 0.1, "y": 0.8 }
        ]
      },
      "labels": [
        { "name": "cat", "confidence": 0.93 }
      ]
    }
  ],
  "detectedText": [
    {
      "text": "Hello World",
      "confidence": 0.99,
      "boundingPolygon": {
        "normalizedVertices": [
          { "x": 0.1, "y": 0.1 },
          { "x": 0.5, "y": 0.1 },
          { "x": 0.5, "y": 0.15 },
          { "x": 0.1, "y": 0.15 }
        ]
      },
      "words": [
        { "text": "Hello", "confidence": 0.99 },
        { "text": "World", "confidence": 0.98 }
      ]
    }
  ],
  "imageMetadata": {
    "width": 1920,
    "height": 1080
  }
}
```

**Analyze Document Request:**
```json
{
  "compartmentId": "ocid1.compartment.oc1...",
  "namespaceName": "mytenancy",
  "bucketName": "my-bucket",
  "objectName": "documents/invoice.pdf",
  "features": ["TABLE_DETECTION", "KEY_VALUE_EXTRACTION"],
  "maxResults": 50
}
```

**Analyze Document Response:**
```json
{
  "pages": [
    {
      "pageNumber": 1,
      "dimensions": { "width": 612, "height": 792 },
      "detectedDocumentTypes": ["INVOICE"],
      "detectedLanguages": [
        { "language": "ENGLISH", "confidence": 0.99 }
      ],
      "tables": [
        {
          "rowCount": 5,
          "columnCount": 3,
          "cells": [
            { "text": "Item", "rowIndex": 0, "columnIndex": 0 },
            { "text": "Qty", "rowIndex": 0, "columnIndex": 1 },
            { "text": "Price", "rowIndex": 0, "columnIndex": 2 }
          ]
        }
      ],
      "keyValuePairs": [
        {
          "key": { "text": "Invoice Number", "confidence": 0.95 },
          "value": { "text": "INV-2025-001", "confidence": 0.97 }
        }
      ]
    }
  ]
}
```

**Video Analysis Request:**
```json
{
  "compartmentId": "ocid1.compartment.oc1...",
  "namespaceName": "mytenancy",
  "bucketName": "my-bucket",
  "objectName": "videos/sample.mp4",
  "features": ["OBJECT_TRACKING"],
  "outputNamespaceName": "mytenancy",
  "outputBucketName": "my-bucket",
  "outputPrefix": "vision-output/"
}
```

**Video Analysis Status Response:**
```json
{
  "workRequestId": "ocid1.aivisionworkrequest.oc1...",
  "status": "IN_PROGRESS",
  "percentComplete": 45,
  "timeAccepted": "2025-06-01T12:00:00Z",
  "timeStarted": "2025-06-01T12:00:05Z",
  "timeFinished": null
}
```

---

## 6. File Structure

```
internal/
  oci/
    aivision.go         # OCI AI Vision SDK wrapper functions
  service/
    aivision.go         # AI Vision service layer
  httpapi/
    aivision.go         # HTTP handlers for AI Vision endpoints
```

---

## 7. Implementation Notes

### 7.1 `oci/aivision.go` Functions

```
AnalyzeImage(ctx, client *aivision.AIServiceVisionClient, req AnalyzeImageRequest) (*AnalyzeImageResponse, error)
AnalyzeImageFromObjectStorage(ctx, client *aivision.AIServiceVisionClient, req AnalyzeImageOSRequest) (*AnalyzeImageResponse, error)
AnalyzeDocument(ctx, client *aivision.AIServiceVisionClient, req AnalyzeDocumentRequest) (*AnalyzeDocumentResponse, error)
AnalyzeVideo(ctx, client *aivision.AIServiceVisionClient, req AnalyzeVideoRequest) (*AnalyzeVideoResponse, error)
GetVideoAnalysisStatus(ctx, client *aivision.AIServiceVisionClient, workRequestID string) (*WorkRequestStatus, error)
CancelVideoAnalysis(ctx, client *aivision.AIServiceVisionClient, workRequestID string) error
```

### 7.2 Proxy Integration

AI Vision operations should go through the existing `WithProxy` decorator:

```go
WithProxy(ctx, pool, creds, masterKey, func(c Clients) error {
    result, err := oci.AnalyzeImage(ctx, c.AiVision, req)
    // ...
})
```

### 7.3 Parity with Java

There is no Java implementation to port. This is a greenfield feature.
The Go implementation is the first and only implementation.

| Feature                      | Java | Go |
|------------------------------|------|-----|
| Image classification         | --   | YES |
| Object detection             | --   | YES |
| Text recognition (OCR)       | --   | YES |
| Document table extraction    | --   | YES |
| Document key-value extraction| --   | YES |
| Video analysis (async)       | --   | YES |
| Custom model management      | --   | future |

### 7.4 Key Considerations

- **Image Size Limits**: Inline base64 images are limited to ~7MB after
  encoding. For larger images, use the Object Storage path. The handler
  should reject oversized inline images with a clear error message.
- **Base64 Encoding**: The frontend sends images as base64-encoded strings.
  The handler should validate the base64 format before passing to the SDK.
- **Feature Selection**: The API allows specifying which features to run.
  The handler should validate that at least one feature is requested.
- **Async Video Analysis**: Video analysis is asynchronous. The API returns
  a `workRequestId`. The frontend should poll the status endpoint until
  completion. Results are written to the specified Object Storage output
  location.
- **Compartment ID**: The compartment ID determines billing and access
  control. It should default to the tenant's root compartment if not
  specified.
- **Output Location**: Both image and document analysis can optionally
  write results to Object Storage. This is useful for large documents
  or batch processing. The output location requires namespace, bucket,
  and prefix.
- **Rate Limiting**: OCI AI Vision has service limits. The Go wrapper
  should handle 429 (Too Many Requests) responses with exponential backoff.
- **Pagination**: `ListModels` and `ListProjects` support pagination.
  Analysis results are returned in full (no pagination needed).
- **Model Training**: Custom model management (create/update/delete projects
  and models) is documented in the SDK but is a lower priority for the
  initial implementation. It can be added in a future phase.
- **Confidence Thresholds**: The response includes confidence scores for
  all detections. The frontend can filter by confidence threshold for
  display purposes.
