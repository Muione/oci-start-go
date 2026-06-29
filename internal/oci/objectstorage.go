// Package oci -- objectstorage.go: OCI Object Storage SDK wrapper (Phase 11.1).
// Ports OciObjectStorageUtil.java. Covers bucket CRUD, object CRUD, presigned
// URL generation via Pre-Authenticated Requests (PAR), and multipart upload
// lifecycle. Functions accept a typed client pointer (matching backup.go pattern)
// so the caller composes proxy routing via WithProxy.
package oci

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

// ---------------------------------------------------------------------------
// Namespace
// ---------------------------------------------------------------------------

// GetNamespace returns the tenancy's object-storage namespace string.
// Parity with OciObjectStorageUtil.getNamespace.
func GetNamespace(ctx context.Context, client *objectstorage.ObjectStorageClient) (string, error) {
	resp, err := client.GetNamespace(ctx, objectstorage.GetNamespaceRequest{})
	if err != nil {
		return "", fmt.Errorf("get namespace: %w", err)
	}
	if resp.Value == nil {
		return "", fmt.Errorf("get namespace: nil value")
	}
	return *resp.Value, nil
}

// ---------------------------------------------------------------------------
// Buckets
// ---------------------------------------------------------------------------

// ListBucketsPaginated lists buckets with pagination support.
// Returns items and the opaque next-page token (opc-next-page).
// Parity with OciObjectStorageUtil.listBucketsPaginated.
func ListBucketsPaginated(ctx context.Context, client *objectstorage.ObjectStorageClient,
	namespace, compartmentID string, limit int, pageToken string,
) ([]objectstorage.BucketSummary, *string, error) {
	req := objectstorage.ListBucketsRequest{
		NamespaceName: common.String(namespace),
		CompartmentId: common.String(compartmentID),
		Limit:         common.Int(limit),
		Fields:        []objectstorage.ListBucketsFieldsEnum{objectstorage.ListBucketsFieldsTags},
	}
	if pageToken != "" {
		req.Page = common.String(pageToken)
	}
	resp, err := client.ListBuckets(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("list buckets: %w", err)
	}
	return resp.Items, resp.OpcNextPage, nil
}

// CreateBucket creates a new bucket. Stores publicAccessType in FreeformTags
// so listBuckets can read it back.
// Parity with OciObjectStorageUtil.createNamedBucket.
func CreateBucket(ctx context.Context, client *objectstorage.ObjectStorageClient,
	namespace, compartmentID, bucketName, publicAccessType string) error {
	accessType := mapPublicAccessType(publicAccessType)
	_, err := client.CreateBucket(ctx, objectstorage.CreateBucketRequest{
		NamespaceName: common.String(namespace),
		CreateBucketDetails: objectstorage.CreateBucketDetails{
			Name:             common.String(bucketName),
			CompartmentId:    common.String(compartmentID),
			PublicAccessType: accessType,
			FreeformTags:     map[string]string{"accessType": publicAccessType},
		},
	})
	if err != nil {
		return fmt.Errorf("create bucket: %w", err)
	}
	return nil
}

// DeleteBucket deletes a bucket (must be empty).
// Parity with OciObjectStorageUtil.deleteNamedBucket.
func DeleteBucket(ctx context.Context, client *objectstorage.ObjectStorageClient,
	namespace, bucketName string) error {
	_, err := client.DeleteBucket(ctx, objectstorage.DeleteBucketRequest{
		NamespaceName: common.String(namespace),
		BucketName:    common.String(bucketName),
	})
	if err != nil {
		return fmt.Errorf("delete bucket: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Objects
// ---------------------------------------------------------------------------

// ListObjectsPaginated lists objects in a bucket with cursor-based pagination.
// Parity with OciObjectStorageUtil.listObjectsPaginated.
func ListObjectsPaginated(ctx context.Context, client *objectstorage.ObjectStorageClient,
	namespace, bucketName, prefix string, limit int, startToken string,
) ([]objectstorage.ObjectSummary, *string, error) {
	req := objectstorage.ListObjectsRequest{
		NamespaceName: common.String(namespace),
		BucketName:    common.String(bucketName),
		Limit:         common.Int(limit),
	}
	if prefix != "" {
		req.Prefix = common.String(prefix)
	}
	if startToken != "" {
		req.Start = common.String(startToken)
	}
	resp, err := client.ListObjects(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("list objects: %w", err)
	}
	if resp.ListObjects.Objects == nil {
		return nil, resp.ListObjects.NextStartWith, nil
	}
	return resp.ListObjects.Objects, resp.ListObjects.NextStartWith, nil
}

// PutObject uploads a single object.
// Parity with OciObjectStorageUtil.uploadNamedObject.
func PutObject(ctx context.Context, client *objectstorage.ObjectStorageClient,
	namespace, bucketName, objectName string, body io.Reader, size int64, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err := client.PutObject(ctx, objectstorage.PutObjectRequest{
		NamespaceName: common.String(namespace),
		BucketName:    common.String(bucketName),
		ObjectName:    common.String(objectName),
		PutObjectBody: io.NopCloser(body),
		ContentLength: common.Int64(size),
		ContentType:   common.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("put object: %w", err)
	}
	return nil
}

// GetObject downloads an object. Returns content reader, content type, and
// content length.
// Parity with OciObjectStorageUtil.downloadNamedObject.
func GetObject(ctx context.Context, client *objectstorage.ObjectStorageClient,
	namespace, bucketName, objectName string) (io.ReadCloser, string, *int64, error) {
	resp, err := client.GetObject(ctx, objectstorage.GetObjectRequest{
		NamespaceName: common.String(namespace),
		BucketName:    common.String(bucketName),
		ObjectName:    common.String(objectName),
	})
	if err != nil {
		return nil, "", nil, fmt.Errorf("get object: %w", err)
	}
	ct := "application/octet-stream"
	if resp.ContentType != nil {
		ct = *resp.ContentType
	}
	return resp.Content, ct, resp.ContentLength, nil
}

// DeleteObject deletes an object from a bucket.
// Parity with OciObjectStorageUtil.deleteNamedObject.
func DeleteObject(ctx context.Context, client *objectstorage.ObjectStorageClient,
	namespace, bucketName, objectName string) error {
	_, err := client.DeleteObject(ctx, objectstorage.DeleteObjectRequest{
		NamespaceName: common.String(namespace),
		BucketName:    common.String(bucketName),
		ObjectName:    common.String(objectName),
	})
	if err != nil {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Presigned URL (Pre-Authenticated Request)
// ---------------------------------------------------------------------------

// CreatePresignedURL creates a Pre-Authenticated Request (PAR) and returns
// the full URL. Parity with OciObjectStorageUtil.generatePresignedUrlForBucket.
func CreatePresignedURL(ctx context.Context, client *objectstorage.ObjectStorageClient,
	namespace, bucketName, objectName string, validitySeconds int64) (string, error) {
	if validitySeconds <= 0 {
		validitySeconds = 3600
	}
	expires := time.Now().Add(time.Duration(validitySeconds) * time.Second)
	resp, err := client.CreatePreauthenticatedRequest(ctx,
		objectstorage.CreatePreauthenticatedRequestRequest{
			NamespaceName: common.String(namespace),
			BucketName:    common.String(bucketName),
			CreatePreauthenticatedRequestDetails: objectstorage.CreatePreauthenticatedRequestDetails{
				Name:       common.String(fmt.Sprintf("PAR-%d", time.Now().UnixMilli())),
				ObjectName: common.String(objectName),
				AccessType: objectstorage.CreatePreauthenticatedRequestDetailsAccessTypeObjectread,
				TimeExpires: &common.SDKTime{Time: expires},
			},
		})
	if err != nil {
		return "", fmt.Errorf("create presigned URL: %w", err)
	}
	if resp.AccessUri == nil {
		return "", fmt.Errorf("create presigned URL: nil access URI")
	}
	// Build full URL: client endpoint + access URI.
	fullURL := client.Endpoint() + *resp.AccessUri
	return fullURL, nil
}

// ---------------------------------------------------------------------------
// Multipart Upload
// ---------------------------------------------------------------------------

// CreateMultipartUpload initiates a multipart upload, returns the upload ID.
// Parity with OciObjectStorageUtil.initiateMultipartUpload.
func CreateMultipartUpload(ctx context.Context, client *objectstorage.ObjectStorageClient,
	namespace, bucketName, objectName, contentType string) (string, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	resp, err := client.CreateMultipartUpload(ctx,
		objectstorage.CreateMultipartUploadRequest{
			NamespaceName: common.String(namespace),
			BucketName:    common.String(bucketName),
			CreateMultipartUploadDetails: objectstorage.CreateMultipartUploadDetails{
				Object:      common.String(objectName),
				ContentType: common.String(contentType),
			},
		})
	if err != nil {
		return "", fmt.Errorf("create multipart upload: %w", err)
	}
	if resp.MultipartUpload.UploadId == nil {
		return "", fmt.Errorf("create multipart upload: nil upload ID")
	}
	return *resp.MultipartUpload.UploadId, nil
}

// UploadPart uploads a single part of a multipart upload. Returns the ETag.
// Parity with OciObjectStorageUtil.uploadPart.
func UploadPart(ctx context.Context, client *objectstorage.ObjectStorageClient,
	namespace, bucketName, objectName, uploadID string, partNumber int,
	body io.Reader, size int64) (string, error) {
	resp, err := client.UploadPart(ctx, objectstorage.UploadPartRequest{
		NamespaceName:  common.String(namespace),
		BucketName:     common.String(bucketName),
		ObjectName:     common.String(objectName),
		UploadId:       common.String(uploadID),
		UploadPartNum:  common.Int(partNumber),
		ContentLength:  common.Int64(size),
		UploadPartBody: io.NopCloser(body),
	})
	if err != nil {
		return "", fmt.Errorf("upload part %d: %w", partNumber, err)
	}
	if resp.ETag == nil {
		return "", fmt.Errorf("upload part %d: nil etag", partNumber)
	}
	return *resp.ETag, nil
}

// CommitMultipartUpload finalizes a multipart upload.
// Parity with OciObjectStorageUtil.commitMultipartUpload.
func CommitMultipartUpload(ctx context.Context, client *objectstorage.ObjectStorageClient,
	namespace, bucketName, objectName, uploadID string,
	parts []objectstorage.CommitMultipartUploadPartDetails) error {
	_, err := client.CommitMultipartUpload(ctx,
		objectstorage.CommitMultipartUploadRequest{
			NamespaceName: common.String(namespace),
			BucketName:    common.String(bucketName),
			ObjectName:    common.String(objectName),
			UploadId:      common.String(uploadID),
			CommitMultipartUploadDetails: objectstorage.CommitMultipartUploadDetails{
				PartsToCommit: parts,
			},
		})
	if err != nil {
		return fmt.Errorf("commit multipart upload: %w", err)
	}
	return nil
}

// AbortMultipartUpload cancels a multipart upload.
// Parity with OciObjectStorageUtil.abortMultipartUpload.
func AbortMultipartUpload(ctx context.Context, client *objectstorage.ObjectStorageClient,
	namespace, bucketName, objectName, uploadID string) error {
	_, err := client.AbortMultipartUpload(ctx,
		objectstorage.AbortMultipartUploadRequest{
			NamespaceName: common.String(namespace),
			BucketName:    common.String(bucketName),
			ObjectName:    common.String(objectName),
			UploadId:      common.String(uploadID),
		})
	if err != nil {
		return fmt.Errorf("abort multipart upload: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// mapPublicAccessType maps a string to the SDK enum for CreateBucket.
func mapPublicAccessType(s string) objectstorage.CreateBucketDetailsPublicAccessTypeEnum {
	switch s {
	case "ObjectRead":
		return objectstorage.CreateBucketDetailsPublicAccessTypeObjectread
	case "ObjectReadWithoutList":
		return objectstorage.CreateBucketDetailsPublicAccessTypeObjectreadwithoutlist
	default:
		return objectstorage.CreateBucketDetailsPublicAccessTypeNopublicaccess
	}
}
