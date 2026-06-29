// Package oci -- storage.go: basic object-storage domain (Phase 3). Ports
// OciObjectStorageUtil.listBuckets (get namespace -> list buckets).
//
// Deprecated: Use objectstorage.go functions instead (Phase 11.1).
// ListBucketsPaginated provides pagination support and is used by the
// ObjectStorageService.
package oci

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

// ListBuckets resolves the tenancy's object-storage namespace then lists
// buckets in compartmentID. Parity with OciObjectStorageUtil.listBuckets.
//
// Deprecated: Use ListBucketsPaginated in objectstorage.go instead.
func ListBuckets(ctx context.Context, c Clients, compartmentID string) ([]objectstorage.BucketSummary, error) {
	nsResp, err := c.ObjectStorage.GetNamespace(ctx, objectstorage.GetNamespaceRequest{})
	if err != nil {
		return nil, fmt.Errorf("get namespace: %w", err)
	}
	resp, err := c.ObjectStorage.ListBuckets(ctx, objectstorage.ListBucketsRequest{
		NamespaceName: nsResp.Value,
		CompartmentId: common.String(compartmentID),
	})
	if err != nil {
		return nil, fmt.Errorf("list buckets: %w", err)
	}
	return resp.Items, nil
}
