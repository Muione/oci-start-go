// Package httpapi -- object_storage.go: Phase 11.1 Object Storage HTTP handlers.
// 15 endpoints for bucket CRUD, object CRUD, presigned URLs, and multipart
// upload lifecycle. Follows the handler pattern from tenant.go: factory
// function returns gin.HandlerFunc, closes over *Deps.
package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
	"github.com/Muione/oci-start-go/internal/service"
)

// ---------------------------------------------------------------------------
// Request/Response structs
// ---------------------------------------------------------------------------

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
	TenantID    int64  `json:"tenantId"`
	Namespace   string `json:"namespace"`
	BucketName  string `json:"bucketName"`
	ObjectName  string `json:"objectName"`
	ContentType string `json:"contentType"`
	TotalSize   int64  `json:"totalSize"`
	ChunkSize   int64  `json:"chunkSize"`
}

type multipartCommitReq struct {
	TenantID   int64         `json:"tenantId"`
	Namespace  string        `json:"namespace"`
	BucketName string        `json:"bucketName"`
	ObjectName string        `json:"objectName"`
	UploadID   string        `json:"uploadId"`
	Parts      []commitPart  `json:"parts"`
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

// ---------------------------------------------------------------------------
// Handler factories
// ---------------------------------------------------------------------------

// GET /oci/storage/namespace?tenantId=
func objectStorageNamespace(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid tenantId")
			return
		}
		ns, err := deps.ObjectStorageSvc.GetNamespace(c.Request.Context(), tenantID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "get namespace failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(map[string]string{"namespace": ns}))
	}
}

// GET /oci/storage/buckets?tenantId=&limit=&pageToken=
func objectStorageListBuckets(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid tenantId")
			return
		}
		limit := 100
		if v := c.Query("limit"); v != "" {
			if n, e := strconv.Atoi(v); e == nil && n > 0 {
				limit = n
			}
		}
		pageToken := c.Query("pageToken")
		result, err := deps.ObjectStorageSvc.ListBuckets(c.Request.Context(), tenantID, limit, pageToken)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "list buckets failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(result))
	}
}

// POST /oci/storage/bucket/create
func objectStorageCreateBucket(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createBucketReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.BucketName == "" {
			response.Fail(c, http.StatusBadRequest, "bucketName is required")
			return
		}
		if req.PublicAccessType == "" {
			req.PublicAccessType = "NoPublicAccess"
		}
		if err := deps.ObjectStorageSvc.CreateBucket(c.Request.Context(), req.TenantID, req.BucketName, req.PublicAccessType); err != nil {
			response.Fail(c, http.StatusInternalServerError, "create bucket failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("bucket created"))
	}
}

// POST /oci/storage/bucket/delete
func objectStorageDeleteBucket(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req deleteBucketReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Namespace == "" || req.BucketName == "" {
			response.Fail(c, http.StatusBadRequest, "namespace and bucketName are required")
			return
		}
		if err := deps.ObjectStorageSvc.DeleteBucket(c.Request.Context(), req.TenantID, req.Namespace, req.BucketName); err != nil {
			response.Fail(c, http.StatusInternalServerError, "delete bucket failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("bucket deleted"))
	}
}

// GET /oci/storage/objects?tenantId=&namespace=&bucketName=&prefix=&limit=&startToken=
func objectStorageListObjects(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid tenantId")
			return
		}
		namespace := c.Query("namespace")
		bucketName := c.Query("bucketName")
		if namespace == "" || bucketName == "" {
			response.Fail(c, http.StatusBadRequest, "namespace and bucketName are required")
			return
		}
		prefix := c.Query("prefix")
		limit := 100
		if v := c.Query("limit"); v != "" {
			if n, e := strconv.Atoi(v); e == nil && n > 0 {
				limit = n
			}
		}
		startToken := c.Query("startToken")
		result, err := deps.ObjectStorageSvc.ListObjects(c.Request.Context(), tenantID, namespace, bucketName, prefix, limit, startToken)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "list objects failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(result))
	}
}

// POST /oci/storage/object/delete
func objectStorageDeleteObject(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req deleteObjectReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Namespace == "" || req.BucketName == "" || req.ObjectName == "" {
			response.Fail(c, http.StatusBadRequest, "namespace, bucketName, and objectName are required")
			return
		}
		if err := deps.ObjectStorageSvc.DeleteObject(c.Request.Context(), req.TenantID, req.Namespace, req.BucketName, req.ObjectName); err != nil {
			response.Fail(c, http.StatusInternalServerError, "delete object failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("object deleted"))
	}
}

// POST /oci/storage/object/upload (multipart/form-data)
func objectStorageUpload(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantIDStr := c.PostForm("tenantId")
		tenantID, err := strconv.ParseInt(tenantIDStr, 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid tenantId")
			return
		}
		namespace := c.PostForm("namespace")
		bucketName := c.PostForm("bucketName")
		if namespace == "" || bucketName == "" {
			response.Fail(c, http.StatusBadRequest, "namespace and bucketName are required")
			return
		}

		fh, err := c.FormFile("file")
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "missing file field")
			return
		}

		objectName := c.PostForm("objectName")
		if objectName == "" {
			objectName = fh.Filename
		}

		contentType := fh.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		f, err := fh.Open()
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "open uploaded file failed")
			return
		}
		defer f.Close()

		if err := deps.ObjectStorageSvc.UploadObject(c.Request.Context(), tenantID, namespace, bucketName, objectName, f, fh.Size, contentType); err != nil {
			response.Fail(c, http.StatusInternalServerError, "upload failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("upload successful"))
	}
}

// GET /oci/storage/object/download?tenantId=&namespace=&bucketName=&objectName=
func objectStorageDownload(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid tenantId")
			return
		}
		namespace := c.Query("namespace")
		bucketName := c.Query("bucketName")
		objectName := c.Query("objectName")
		if namespace == "" || bucketName == "" || objectName == "" {
			response.Fail(c, http.StatusBadRequest, "namespace, bucketName, and objectName are required")
			return
		}

		rc, ct, cl, err := deps.ObjectStorageSvc.DownloadObject(c.Request.Context(), tenantID, namespace, bucketName, objectName)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "download failed: "+err.Error())
			return
		}
		defer rc.Close()

		// Extract filename from object path.
		fileName := path.Base(objectName)
		encodedName := url.PathEscape(fileName)

		c.Header("Content-Type", ct)
		if cl != nil && *cl > 0 {
			c.Header("Content-Length", strconv.FormatInt(*cl, 10))
		}
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", encodedName))

		buf := make([]byte, 8192)
		_, _ = io.CopyBuffer(c.Writer, rc, buf)
	}
}

// GET /oci/storage/object/preview?tenantId=&namespace=&bucketName=&objectName=
func objectStoragePreview(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid tenantId")
			return
		}
		namespace := c.Query("namespace")
		bucketName := c.Query("bucketName")
		objectName := c.Query("objectName")
		if namespace == "" || bucketName == "" || objectName == "" {
			response.Fail(c, http.StatusBadRequest, "namespace, bucketName, and objectName are required")
			return
		}

		rc, ct, cl, err := deps.ObjectStorageSvc.PreviewObject(c.Request.Context(), tenantID, namespace, bucketName, objectName)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "preview failed: "+err.Error())
			return
		}
		defer rc.Close()

		c.Header("Content-Type", ct)
		if cl != nil && *cl > 0 {
			c.Header("Content-Length", strconv.FormatInt(*cl, 10))
		}

		// Inline for previewable types, attachment for others.
		if isInlinePreviewable(ct) {
			c.Header("Content-Disposition", "inline")
		} else {
			fileName := path.Base(objectName)
			encodedName := url.PathEscape(fileName)
			c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", encodedName))
		}

		buf := make([]byte, 8192)
		_, _ = io.CopyBuffer(c.Writer, rc, buf)
	}
}

// POST /oci/storage/object/presigned
func objectStoragePresigned(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req presignedReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Namespace == "" || req.BucketName == "" || req.ObjectName == "" {
			response.Fail(c, http.StatusBadRequest, "namespace, bucketName, and objectName are required")
			return
		}
		if req.ValiditySeconds <= 0 {
			req.ValiditySeconds = 3600
		}
		presignedURL, err := deps.ObjectStorageSvc.GeneratePresignedURL(c.Request.Context(), req.TenantID, req.Namespace, req.BucketName, req.ObjectName, req.ValiditySeconds)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "generate presigned URL failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(map[string]string{"url": presignedURL}))
	}
}

// POST /oci/storage/object/multipart/initiate
func objectStorageMultipartInitiate(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req multipartInitReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Namespace == "" || req.BucketName == "" || req.ObjectName == "" {
			response.Fail(c, http.StatusBadRequest, "namespace, bucketName, and objectName are required")
			return
		}
		if req.TotalSize <= 0 || req.ChunkSize <= 0 {
			response.Fail(c, http.StatusBadRequest, "totalSize and chunkSize must be positive")
			return
		}
		if req.ContentType == "" {
			req.ContentType = "application/octet-stream"
		}
		result, err := deps.ObjectStorageSvc.InitiateMultipartUpload(c.Request.Context(), req.TenantID, req.Namespace, req.BucketName, req.ObjectName, req.ContentType, req.TotalSize, req.ChunkSize)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "initiate multipart upload failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(result))
	}
}

// POST /oci/storage/object/multipart/part (multipart/form-data)
func objectStorageMultipartPart(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantIDStr := c.PostForm("tenantId")
		tenantID, err := strconv.ParseInt(tenantIDStr, 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid tenantId")
			return
		}
		namespace := c.PostForm("namespace")
		bucketName := c.PostForm("bucketName")
		objectName := c.PostForm("objectName")
		uploadID := c.PostForm("uploadId")
		partNumberStr := c.PostForm("partNumber")
		if namespace == "" || bucketName == "" || objectName == "" || uploadID == "" || partNumberStr == "" {
			response.Fail(c, http.StatusBadRequest, "all fields are required")
			return
		}
		partNumber, err := strconv.Atoi(partNumberStr)
		if err != nil || partNumber <= 0 {
			response.Fail(c, http.StatusBadRequest, "invalid partNumber")
			return
		}

		fh, err := c.FormFile("chunk")
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "missing chunk file")
			return
		}

		f, err := fh.Open()
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "open chunk file failed")
			return
		}
		defer f.Close()

		result, err := deps.ObjectStorageSvc.UploadPart(c.Request.Context(), tenantID, namespace, bucketName, objectName, uploadID, partNumber, f, fh.Size)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "upload part failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(result))
	}
}

// POST /oci/storage/object/multipart/commit
func objectStorageMultipartCommit(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req multipartCommitReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Namespace == "" || req.BucketName == "" || req.ObjectName == "" || req.UploadID == "" {
			response.Fail(c, http.StatusBadRequest, "namespace, bucketName, objectName, and uploadId are required")
			return
		}
		parts := make([]service.CommitPart, 0, len(req.Parts))
		for _, p := range req.Parts {
			parts = append(parts, service.CommitPart{PartNum: p.PartNum, Etag: p.Etag})
		}
		if err := deps.ObjectStorageSvc.CommitMultipartUpload(c.Request.Context(), req.TenantID, req.Namespace, req.BucketName, req.ObjectName, req.UploadID, parts); err != nil {
			response.Fail(c, http.StatusInternalServerError, "commit failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("upload completed"))
	}
}

// POST /oci/storage/object/multipart/abort
func objectStorageMultipartAbort(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req multipartAbortReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Namespace == "" || req.BucketName == "" || req.ObjectName == "" || req.UploadID == "" {
			response.Fail(c, http.StatusBadRequest, "namespace, bucketName, objectName, and uploadId are required")
			return
		}
		if err := deps.ObjectStorageSvc.AbortMultipartUpload(c.Request.Context(), req.TenantID, req.Namespace, req.BucketName, req.ObjectName, req.UploadID); err != nil {
			response.Fail(c, http.StatusInternalServerError, "abort failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("upload aborted"))
	}
}

// GET /oci/storage/object/multipart/resumeable?tenantId=&bucketName=
func objectStorageMultipartResumable(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid tenantId")
			return
		}
		bucketName := c.Query("bucketName")
		if bucketName == "" {
			response.Fail(c, http.StatusBadRequest, "bucketName is required")
			return
		}
		result, err := deps.ObjectStorageSvc.ListResumableUploads(c.Request.Context(), tenantID, bucketName)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "list resumable uploads failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(result))
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// isInlinePreviewable returns true if the content type should be displayed
// inline. Parity with Java controller.isInlinePreviewable.
func isInlinePreviewable(contentType string) bool {
	ct := contentType
	if len(ct) >= 5 && ct[:5] == "image" {
		return true
	}
	if len(ct) >= 4 && ct[:4] == "text" {
		return true
	}
	if contains(ct, "pdf") || contains(ct, "json") || contains(ct, "xml") {
		return true
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
