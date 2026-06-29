// Package service -- object_storage.go: Phase 11.1 Object Storage service.
// Manages bucket CRUD, object CRUD, presigned URL generation, and multipart
// upload lifecycle with resumable upload tracking in the database.
// Parity with Java OciObjectStorageController + OciMultipartUploadServiceImpl.
package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

// ObjectStorageService manages OCI object storage operations.
type ObjectStorageService struct {
	store     *db.Store
	masterKey []byte
	pool      *oci.ProxyPool
}

// NewObjectStorageService constructs an ObjectStorageService.
func NewObjectStorageService(store *db.Store, masterKey []byte, pool *oci.ProxyPool) *ObjectStorageService {
	return &ObjectStorageService{store: store, masterKey: masterKey, pool: pool}
}

// ---------------------------------------------------------------------------
// Response / VO types
// ---------------------------------------------------------------------------

// BucketListResult is the paginated bucket list response.
type BucketListResult struct {
	Items    []BucketVO `json:"items"`
	NextPage *string    `json:"nextPage"`
}

// BucketVO is a bucket summary for the API response.
type BucketVO struct {
	Name         string    `json:"name"`
	Namespace    string    `json:"namespace"`
	TimeCreated  time.Time `json:"timeCreated"`
	PublicAccess string    `json:"publicAccess"`
}

// ObjectListResult is the paginated object list response.
type ObjectListResult struct {
	Items         []ObjectVO `json:"items"`
	NextStartWith *string    `json:"nextStartWith"`
}

// ObjectVO is an object summary for the API response.
type ObjectVO struct {
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	TimeModified time.Time `json:"timeModified"`
}

// MultipartInitResult is the response for initiating a multipart upload.
type MultipartInitResult struct {
	UploadID   string `json:"uploadId"`
	ObjectName string `json:"objectName"`
	Namespace  string `json:"namespace"`
	BucketName string `json:"bucketName"`
}

// UploadPartResult is the response for uploading a single part.
type UploadPartResult struct {
	PartNum int    `json:"partNum"`
	Etag    string `json:"etag"`
}

// CommitPart represents a part in the commit request.
type CommitPart struct {
	PartNum int    `json:"partNum"`
	Etag    string `json:"etag"`
}

// ResumableUpload is the response for listing resumable uploads.
type ResumableUpload struct {
	ID                 int64        `json:"id"`
	UploadID           string       `json:"uploadId"`
	ObjectName         string       `json:"objectName"`
	BucketName         string       `json:"bucketName"`
	Namespace          string       `json:"namespace"`
	TotalSize          int64        `json:"totalSize"`
	ChunkSize          int64        `json:"chunkSize"`
	TotalParts         int          `json:"totalParts"`
	CompletedPartCount int          `json:"completedPartCount"`
	CompletedParts     []CommitPart `json:"completedParts"`
	CreateTime         string       `json:"createTime"`
}

// ---------------------------------------------------------------------------
// Namespace
// ---------------------------------------------------------------------------

// GetNamespace returns the tenancy's object-storage namespace.
func (s *ObjectStorageService) GetNamespace(ctx context.Context, tenantID int64) (string, error) {
	var namespace string
	err := s.withTenantClient(ctx, tenantID, func(c oci.Clients) error {
		ns, err := oci.GetNamespace(ctx, c.ObjectStorage)
		if err != nil {
			return err
		}
		namespace = ns
		return nil
	})
	return namespace, err
}

// ---------------------------------------------------------------------------
// Bucket CRUD
// ---------------------------------------------------------------------------

// ListBuckets returns paginated buckets for the tenancy compartment.
func (s *ObjectStorageService) ListBuckets(ctx context.Context, tenantID int64, limit int, pageToken string) (*BucketListResult, error) {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := tenantToCredsObj(t)
	compartmentID := creds.Tenancy // tenancy OCID is the default compartment

	var result *BucketListResult
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		ns, err := oci.GetNamespace(ctx, c.ObjectStorage)
		if err != nil {
			return err
		}
		items, nextPage, err := oci.ListBucketsPaginated(ctx, c.ObjectStorage, ns, compartmentID, limit, pageToken)
		if err != nil {
			return err
		}
		buckets := make([]BucketVO, 0, len(items))
		for _, b := range items {
			publicAccess := "NoPublicAccess"
			if b.FreeformTags != nil {
				if v, ok := b.FreeformTags["accessType"]; ok && v != "" {
					publicAccess = v
				}
			}
			vo := BucketVO{
				Name:         derefStr(b.Name),
				Namespace:    derefStr(b.Namespace),
				PublicAccess: publicAccess,
			}
			if b.TimeCreated != nil {
				vo.TimeCreated = b.TimeCreated.Time
			}
			buckets = append(buckets, vo)
		}
		result = &BucketListResult{Items: buckets, NextPage: nextPage}
		return nil
	})
	return result, err
}

// CreateBucket creates a new bucket in the tenancy compartment.
func (s *ObjectStorageService) CreateBucket(ctx context.Context, tenantID int64, bucketName, publicAccessType string) error {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := tenantToCredsObj(t)
	compartmentID := creds.Tenancy

	return oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		ns, err := oci.GetNamespace(ctx, c.ObjectStorage)
		if err != nil {
			return err
		}
		return oci.CreateBucket(ctx, c.ObjectStorage, ns, compartmentID, bucketName, publicAccessType)
	})
}

// DeleteBucket deletes a bucket (must be empty).
func (s *ObjectStorageService) DeleteBucket(ctx context.Context, tenantID int64, namespace, bucketName string) error {
	return s.withTenantClient(ctx, tenantID, func(c oci.Clients) error {
		return oci.DeleteBucket(ctx, c.ObjectStorage, namespace, bucketName)
	})
}

// ---------------------------------------------------------------------------
// Object CRUD
// ---------------------------------------------------------------------------

// ListObjects returns paginated objects in a bucket.
func (s *ObjectStorageService) ListObjects(ctx context.Context, tenantID int64, namespace, bucketName, prefix string, limit int, startToken string) (*ObjectListResult, error) {
	var result *ObjectListResult
	err := s.withTenantClient(ctx, tenantID, func(c oci.Clients) error {
		items, nextStartWith, err := oci.ListObjectsPaginated(ctx, c.ObjectStorage, namespace, bucketName, prefix, limit, startToken)
		if err != nil {
			return err
		}
		objs := make([]ObjectVO, 0, len(items))
		for _, o := range items {
			vo := ObjectVO{
				Name: derefStr(o.Name),
				Size: derefInt64(o.Size),
			}
			if o.TimeModified != nil {
				vo.TimeModified = o.TimeModified.Time
			}
			objs = append(objs, vo)
		}
		result = &ObjectListResult{Items: objs, NextStartWith: nextStartWith}
		return nil
	})
	return result, err
}

// DeleteObject deletes an object from a bucket.
func (s *ObjectStorageService) DeleteObject(ctx context.Context, tenantID int64, namespace, bucketName, objectName string) error {
	return s.withTenantClient(ctx, tenantID, func(c oci.Clients) error {
		return oci.DeleteObject(ctx, c.ObjectStorage, namespace, bucketName, objectName)
	})
}

// UploadObject uploads a file via PutObject.
func (s *ObjectStorageService) UploadObject(ctx context.Context, tenantID int64, namespace, bucketName, objectName string, body io.Reader, size int64, contentType string) error {
	return s.withTenantClient(ctx, tenantID, func(c oci.Clients) error {
		return oci.PutObject(ctx, c.ObjectStorage, namespace, bucketName, objectName, body, size, contentType)
	})
}

// DownloadObject returns the object content stream + metadata for the HTTP
// handler to stream.
func (s *ObjectStorageService) DownloadObject(ctx context.Context, tenantID int64, namespace, bucketName, objectName string) (io.ReadCloser, string, *int64, error) {
	var rc io.ReadCloser
	var ct string
	var cl *int64
	err := s.withTenantClient(ctx, tenantID, func(c oci.Clients) error {
		var innerErr error
		rc, ct, cl, innerErr = oci.GetObject(ctx, c.ObjectStorage, namespace, bucketName, objectName)
		return innerErr
	})
	return rc, ct, cl, err
}

// PreviewObject returns the object content with resolved content type for
// inline preview.
func (s *ObjectStorageService) PreviewObject(ctx context.Context, tenantID int64, namespace, bucketName, objectName string) (io.ReadCloser, string, *int64, error) {
	rc, ct, cl, err := s.DownloadObject(ctx, tenantID, namespace, bucketName, objectName)
	if err != nil {
		return nil, "", nil, err
	}
	ct = resolveContentType(ct, objectName)
	return rc, ct, cl, nil
}

// GeneratePresignedURL creates a PAR and returns the full URL.
func (s *ObjectStorageService) GeneratePresignedURL(ctx context.Context, tenantID int64, namespace, bucketName, objectName string, validitySeconds int64) (string, error) {
	var url string
	err := s.withTenantClient(ctx, tenantID, func(c oci.Clients) error {
		var innerErr error
		url, innerErr = oci.CreatePresignedURL(ctx, c.ObjectStorage, namespace, bucketName, objectName, validitySeconds)
		return innerErr
	})
	return url, err
}

// ---------------------------------------------------------------------------
// Multipart Upload
// ---------------------------------------------------------------------------

// InitiateMultipartUpload starts a multipart upload, aborts existing active
// uploads for the same file.
func (s *ObjectStorageService) InitiateMultipartUpload(ctx context.Context, tenantID int64, namespace, bucketName, objectName, contentType string, totalSize, chunkSize int64) (*MultipartInitResult, error) {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := tenantToCredsObj(t)
	q := repo.New(s.store.Write)

	// Abort existing active uploads for the same file.
	activeUploads, err := q.FindActiveUploads(ctx, repo.FindActiveUploadsParams{
		TenantID:   tenantID,
		BucketName: bucketName,
		ObjectName: objectName,
	})
	if err != nil {
		return nil, fmt.Errorf("find active uploads: %w", err)
	}
	for _, rec := range activeUploads {
		// Abort on OCI (best-effort, ignore errors for deleted tenants).
		_ = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
			return oci.AbortMultipartUpload(ctx, c.ObjectStorage,
				nsStr(rec.Namespace), bucketName, objectName, rec.UploadID)
		})
		// Mark DB record as aborted.
		_ = q.UpdateMultipartUploadStatus(ctx, repo.UpdateMultipartUploadStatusParams{
			Status:     sql.NullString{String: "aborted", Valid: true},
			UpdateTime: sql.NullString{String: nowStr(), Valid: true},
			UploadID:   rec.UploadID,
		})
	}

	// Create new multipart upload on OCI.
	var uploadID string
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		var innerErr error
		uploadID, innerErr = oci.CreateMultipartUpload(ctx, c.ObjectStorage, namespace, bucketName, objectName, contentType)
		return innerErr
	})
	if err != nil {
		return nil, err
	}

	// Compute total parts.
	totalParts := int(math.Ceil(float64(totalSize) / float64(chunkSize)))
	now := nowStr()

	// Insert DB record.
	err = q.CreateMultipartUploadRecord(ctx, repo.CreateMultipartUploadRecordParams{
		TenantID:    tenantID,
		TenancyOcid: sql.NullString{String: creds.Tenancy, Valid: true},
		Namespace:   sql.NullString{String: namespace, Valid: true},
		BucketName:  bucketName,
		ObjectName:  objectName,
		UploadID:    uploadID,
		TotalSize:   sql.NullInt64{Int64: totalSize, Valid: true},
		ChunkSize:   sql.NullInt64{Int64: chunkSize, Valid: true},
		TotalParts:  sql.NullInt64{Int64: int64(totalParts), Valid: true},
		CreateTime:  sql.NullString{String: now, Valid: true},
		UpdateTime:  sql.NullString{String: now, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("insert multipart upload record: %w", err)
	}

	return &MultipartInitResult{
		UploadID:   uploadID,
		ObjectName: objectName,
		Namespace:  namespace,
		BucketName: bucketName,
	}, nil
}

// UploadPart uploads a single part and updates the DB completed_parts JSON.
func (s *ObjectStorageService) UploadPart(ctx context.Context, tenantID int64, namespace, bucketName, objectName, uploadID string, partNumber int, body io.Reader, size int64) (*UploadPartResult, error) {
	// Upload to OCI.
	var etag string
	err := s.withTenantClient(ctx, tenantID, func(c oci.Clients) error {
		var innerErr error
		etag, innerErr = oci.UploadPart(ctx, c.ObjectStorage, namespace, bucketName, objectName, uploadID, partNumber, body, size)
		return innerErr
	})
	if err != nil {
		return nil, err
	}

	// Update DB completed_parts (deduplicate by partNum).
	q := repo.New(s.store.Write)
	rec, err := q.FindByUploadId(ctx, uploadID)
	if err != nil {
		return nil, fmt.Errorf("find upload record: %w", err)
	}

	parts := parseCompletedParts(nsStr(rec.CompletedParts))
	// Remove existing entry with same partNum.
	filtered := make([]CommitPart, 0, len(parts))
	for _, p := range parts {
		if p.PartNum != partNumber {
			filtered = append(filtered, p)
		}
	}
	filtered = append(filtered, CommitPart{PartNum: partNumber, Etag: etag})

	partsJSON, err := json.Marshal(filtered)
	if err != nil {
		return nil, fmt.Errorf("marshal completed parts: %w", err)
	}

	err = q.UpdateMultipartUploadParts(ctx, repo.UpdateMultipartUploadPartsParams{
		CompletedParts: sql.NullString{String: string(partsJSON), Valid: true},
		UpdateTime:     sql.NullString{String: nowStr(), Valid: true},
		UploadID:       uploadID,
	})
	if err != nil {
		return nil, fmt.Errorf("update completed parts: %w", err)
	}

	return &UploadPartResult{PartNum: partNumber, Etag: etag}, nil
}

// CommitMultipartUpload finalizes the upload and marks DB record as completed.
func (s *ObjectStorageService) CommitMultipartUpload(ctx context.Context, tenantID int64, namespace, bucketName, objectName, uploadID string, parts []CommitPart) error {
	// Build OCI commit parts.
	ociParts := make([]objectstorage.CommitMultipartUploadPartDetails, 0, len(parts))
	for _, p := range parts {
		ociParts = append(ociParts, objectstorage.CommitMultipartUploadPartDetails{
			PartNum: &p.PartNum,
			Etag:    &p.Etag,
		})
	}

	err := s.withTenantClient(ctx, tenantID, func(c oci.Clients) error {
		return oci.CommitMultipartUpload(ctx, c.ObjectStorage, namespace, bucketName, objectName, uploadID, ociParts)
	})
	if err != nil {
		return err
	}

	// Mark DB record as completed.
	return repo.New(s.store.Write).UpdateMultipartUploadStatus(ctx, repo.UpdateMultipartUploadStatusParams{
		Status:     sql.NullString{String: "completed", Valid: true},
		UpdateTime: sql.NullString{String: nowStr(), Valid: true},
		UploadID:   uploadID,
	})
}

// AbortMultipartUpload cancels the upload and marks DB record as aborted.
func (s *ObjectStorageService) AbortMultipartUpload(ctx context.Context, tenantID int64, namespace, bucketName, objectName, uploadID string) error {
	err := s.withTenantClient(ctx, tenantID, func(c oci.Clients) error {
		return oci.AbortMultipartUpload(ctx, c.ObjectStorage, namespace, bucketName, objectName, uploadID)
	})
	if err != nil {
		return err
	}

	return repo.New(s.store.Write).UpdateMultipartUploadStatus(ctx, repo.UpdateMultipartUploadStatusParams{
		Status:     sql.NullString{String: "aborted", Valid: true},
		UpdateTime: sql.NullString{String: nowStr(), Valid: true},
		UploadID:   uploadID,
	})
}

// ListResumableUploads returns active multipart uploads for a bucket.
func (s *ObjectStorageService) ListResumableUploads(ctx context.Context, tenantID int64, bucketName string) ([]ResumableUpload, error) {
	rows, err := repo.New(s.store.Read).ListResumableUploads(ctx, repo.ListResumableUploadsParams{
		TenantID:   tenantID,
		BucketName: bucketName,
	})
	if err != nil {
		return nil, fmt.Errorf("list resumable uploads: %w", err)
	}

	out := make([]ResumableUpload, 0, len(rows))
	for _, r := range rows {
		parts := parseCompletedParts(nsStr(r.CompletedParts))
		vo := ResumableUpload{
			ID:                 r.ID,
			UploadID:           r.UploadID,
			ObjectName:         r.ObjectName,
			BucketName:         r.BucketName,
			Namespace:          nsStr(r.Namespace),
			TotalSize:          ni64(r.TotalSize),
			ChunkSize:          ni64(r.ChunkSize),
			TotalParts:         int(ni64(r.TotalParts)),
			CompletedPartCount: len(parts),
			CompletedParts:     parts,
			CreateTime:         nsStr(r.CreateTime),
		}
		out = append(out, vo)
	}
	return out, nil
}

// CleanupStaleUploads is called by the scheduler. Aborts uploads older than
// 24 hours.
func (s *ObjectStorageService) CleanupStaleUploads(ctx context.Context) error {
	threshold := time.Now().Add(-24 * time.Hour).Format(httpTimeFmt)
	stale, err := repo.New(s.store.Read).FindStaleUploads(ctx, sql.NullString{String: threshold, Valid: true})
	if err != nil {
		return fmt.Errorf("find stale uploads: %w", err)
	}

	q := repo.New(s.store.Write)
	for _, rec := range stale {
		// Resolve tenant: try tenantId first, fall back to tenancyOcid.
		creds, resolveErr := s.resolveTenantForCleanup(ctx, rec)
		if resolveErr != nil {
			// Cannot resolve tenant -- mark as aborted to stop retries.
			_ = q.UpdateMultipartUploadStatus(ctx, repo.UpdateMultipartUploadStatusParams{
				Status:     sql.NullString{String: "aborted", Valid: true},
				UpdateTime: sql.NullString{String: nowStr(), Valid: true},
				UploadID:   rec.UploadID,
			})
			continue
		}

		// Abort on OCI (best-effort).
		_ = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
			return oci.AbortMultipartUpload(ctx, c.ObjectStorage,
				nsStr(rec.Namespace), rec.BucketName, rec.ObjectName, rec.UploadID)
		})

		// Mark DB record as aborted.
		_ = q.UpdateMultipartUploadStatus(ctx, repo.UpdateMultipartUploadStatusParams{
			Status:     sql.NullString{String: "aborted", Valid: true},
			UpdateTime: sql.NullString{String: nowStr(), Valid: true},
			UploadID:   rec.UploadID,
		})
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// withTenantClient looks up a tenant by ID, builds OCI clients via proxy,
// and runs fn.
func (s *ObjectStorageService) withTenantClient(ctx context.Context, tenantID int64, fn func(oci.Clients) error) error {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := tenantToCredsObj(t)
	return oci.WithProxy(ctx, s.pool, creds, s.masterKey, fn)
}

// resolveTenantForCleanup resolves OCI credentials for a stale upload record.
// Tries tenantId first; falls back to tenancyOcid if the tenant was deleted
// and reimported (new DB ID). Fixes the record's tenant_id on success.
func (s *ObjectStorageService) resolveTenantForCleanup(ctx context.Context, rec repo.OciMultipartUploadRecord) (oci.Credentials, error) {
	// Try by tenant ID first.
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, rec.TenantID)
	if err == nil {
		return tenantToCredsObj(t), nil
	}

	// Fallback: find by tenancy OCID.
	if !rec.TenancyOcid.Valid || rec.TenancyOcid.String == "" {
		return oci.Credentials{}, fmt.Errorf("tenant %d not found and no tenancy_ocid fallback", rec.TenantID)
	}

	// Search all tenants for a matching tenancy OCID.
	tenants, err := repo.New(s.store.Read).ListTenants(ctx)
	if err != nil {
		return oci.Credentials{}, fmt.Errorf("list tenants for fallback: %w", err)
	}
	for _, t2 := range tenants {
		if nsStr(t2.Tenancy) == rec.TenancyOcid.String {
			// Get full tenant record (ListTenantsRow omits key_file_blob).
			full, err := repo.New(s.store.Read).FindTenantByID(ctx, t2.ID)
			if err != nil {
				return oci.Credentials{}, fmt.Errorf("find tenant %d for fallback: %w", t2.ID, err)
			}
			// Fix the record's tenant_id.
			_ = repo.New(s.store.Write).FixMultipartUploadTenantId(ctx, repo.FixMultipartUploadTenantIdParams{
				TenantID:    t2.ID,
				UpdateTime:  sql.NullString{String: nowStr(), Valid: true},
				TenancyOcid: rec.TenancyOcid,
				TenantID_2:  rec.TenantID,
			})
			return tenantToCredsObj(full), nil
		}
	}

	return oci.Credentials{}, fmt.Errorf("no tenant found for tenancy_ocid %s", rec.TenancyOcid.String)
}

// tenantToCredsObj converts a repo.Tenant to oci.Credentials.
func tenantToCredsObj(t repo.Tenant) oci.Credentials {
	return oci.Credentials{
		Tenancy:     nsStr(t.Tenancy),
		UserID:      nsStr(t.TenantID),
		Fingerprint: nsStr(t.Fingerprint),
		Region:      nsStr(t.Region),
		KeyFileBlob: nsStr(t.KeyFileBlob),
		KeyFile:     nsStr(t.KeyFile),
	}
}

// parseCompletedParts parses the JSON completed_parts column.
func parseCompletedParts(jsonStr string) []CommitPart {
	if jsonStr == "" || jsonStr == "[]" {
		return nil
	}
	var parts []CommitPart
	if err := json.Unmarshal([]byte(jsonStr), &parts); err != nil {
		return nil
	}
	return parts
}

// resolveContentType infers content type from file extension when OCI returns
// application/octet-stream. Parity with Java controller.resolveContentType.
func resolveContentType(ociContentType, objectName string) string {
	if ociContentType != "" && ociContentType != "application/octet-stream" {
		return ociContentType
	}
	ext := strings.ToLower(filepath.Ext(objectName))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".json":
		return "application/json"
	case ".txt", ".log", ".md":
		return "text/plain;charset=UTF-8"
	case ".html", ".htm":
		return "text/html"
	case ".xml":
		return "application/xml"
	default:
		return ociContentType
	}
}

// nsStr unwraps a sql.NullString, returning "" when invalid.
func nsStr(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}

// ni64 unwraps a sql.NullInt64, returning 0 when invalid.
func ni64(v sql.NullInt64) int64 {
	if v.Valid {
		return v.Int64
	}
	return 0
}

// nowStr returns the current time formatted for DB storage.
func nowStr() string {
	return time.Now().Format(httpTimeFmt)
}

// derefStr returns *s or "" if nil.
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// derefInt64 returns *v or 0 if nil.
func derefInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}
