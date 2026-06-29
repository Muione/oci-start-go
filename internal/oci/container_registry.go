// Package oci -- container_registry.go: Phase 14.2 OCI Container/Artifact Registry
// SDK wrapper. Parity with Java OcirUtils: list repositories, list images,
// delete image, delete repository, and cleanup old images.
package oci

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/oracle/oci-go-sdk/v65/artifacts"
	"github.com/oracle/oci-go-sdk/v65/common"
)

// ---------------------------------------------------------------------------
// DTOs
// ---------------------------------------------------------------------------

// ContainerRepositoryVO is a simplified container repository for the API response.
type ContainerRepositoryVO struct {
	ID              string `json:"id"`
	CompartmentID   string `json:"compartmentId"`
	DisplayName     string `json:"displayName"`
	NamespaceName   string `json:"namespaceName"`
	RepositoryName  string `json:"repositoryName"`
	ImageCount      int    `json:"imageCount"`
	IsPublic        bool   `json:"isPublic"`
	LifecycleState  string `json:"lifecycleState"`
	TimeCreated     string `json:"timeCreated,omitempty"`
	TimeLastPushed  string `json:"timeLastPushed,omitempty"`
}

// ContainerImageVO is a simplified container image for the API response.
type ContainerImageVO struct {
	ID               string `json:"id"`
	CompartmentID    string `json:"compartmentId"`
	RepositoryName   string `json:"repositoryName"`
	DisplayName      string `json:"displayName"`
	Version          string `json:"version"`
	LayersSizeInBytes int64 `json:"layersSizeInBytes"`
	SizeInBytes      int64  `json:"sizeInBytes"`
	LifecycleState   string `json:"lifecycleState"`
	TimeCreated      string `json:"timeCreated,omitempty"`
	TimeLastPulled   string `json:"timeLastPulled,omitempty"`
}

// CleanupResult is the response for the cleanup operation.
type CleanupResult struct {
	RepositoryName  string   `json:"repositoryName"`
	TotalImages     int      `json:"totalImages"`
	KeepCount       int      `json:"keepCount"`
	DeletedCount    int      `json:"deletedCount"`
	DeletedImageIDs []string `json:"deletedImageIds"`
}

// ---------------------------------------------------------------------------
// SDK wrappers
// ---------------------------------------------------------------------------

// ListContainerRepositories returns all container repositories in a compartment.
func ListContainerRepositories(ctx context.Context, c *artifacts.ArtifactsClient, compartmentID string) ([]ContainerRepositoryVO, error) {
	var all []ContainerRepositoryVO
	var page *string

	for {
		resp, err := c.ListContainerRepositories(ctx, artifacts.ListContainerRepositoriesRequest{
			CompartmentId:  common.String(compartmentID),
			LifecycleState: common.String("AVAILABLE"),
			Page:           page,
		})
		if err != nil {
			return nil, fmt.Errorf("ocir: list repositories: %w", err)
		}
		for _, r := range resp.Items {
			all = append(all, containerRepoToVO(r))
		}
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return all, nil
}

// DeleteContainerRepository deletes a container repository by ID.
// Treats 404 as success (already deleted).
func DeleteContainerRepository(ctx context.Context, c *artifacts.ArtifactsClient, repositoryID string) error {
	_, err := c.DeleteContainerRepository(ctx, artifacts.DeleteContainerRepositoryRequest{
		RepositoryId: common.String(repositoryID),
	})
	if err != nil {
		if isOCI404(err) {
			return nil
		}
		return fmt.Errorf("ocir: delete repository: %w", err)
	}
	return nil
}

// ListContainerImages returns container images in a repository.
func ListContainerImages(ctx context.Context, c *artifacts.ArtifactsClient, compartmentID, repositoryName string) ([]ContainerImageVO, error) {
	var all []ContainerImageVO
	var page *string

	for {
		resp, err := c.ListContainerImages(ctx, artifacts.ListContainerImagesRequest{
			CompartmentId:  common.String(compartmentID),
			RepositoryName: common.String(repositoryName),
			SortBy:         artifacts.ListContainerImagesSortByTimecreated,
			SortOrder:      artifacts.ListContainerImagesSortOrderDesc,
			Page:           page,
		})
		if err != nil {
			return nil, fmt.Errorf("ocir: list images: %w", err)
		}
		for _, img := range resp.Items {
			all = append(all, containerImageToVO(img))
		}
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return all, nil
}

// GetContainerImage returns a single container image by ID.
func GetContainerImage(ctx context.Context, c *artifacts.ArtifactsClient, imageID string) (*ContainerImageVO, error) {
	resp, err := c.GetContainerImage(ctx, artifacts.GetContainerImageRequest{
		ImageId: common.String(imageID),
	})
	if err != nil {
		return nil, fmt.Errorf("ocir: get image: %w", err)
	}
	vo := ContainerImageVO{
		ID:               derefStr(resp.Id),
		CompartmentID:    derefStr(resp.CompartmentId),
		RepositoryName:   derefStr(resp.RepositoryName),
		DisplayName:      derefStr(resp.DisplayName),
		Version:          derefStr(resp.Version),
		LayersSizeInBytes: derefInt64(resp.LayersSizeInBytes),
		SizeInBytes:      derefInt64(resp.SizeInBytes),
		LifecycleState:   derefStr(resp.LifecycleState),
	}
	if resp.TimeCreated != nil {
		vo.TimeCreated = resp.TimeCreated.Time.Format(timeLayout)
	}
	if resp.TimeLastPulled != nil {
		vo.TimeLastPulled = resp.TimeLastPulled.Time.Format(timeLayout)
	}
	return &vo, nil
}

// DeleteContainerImage deletes a container image by ID.
// Treats 404 as success (already deleted).
func DeleteContainerImage(ctx context.Context, c *artifacts.ArtifactsClient, imageID string) error {
	_, err := c.DeleteContainerImage(ctx, artifacts.DeleteContainerImageRequest{
		ImageId: common.String(imageID),
	})
	if err != nil {
		if isOCI404(err) {
			return nil
		}
		return fmt.Errorf("ocir: delete image: %w", err)
	}
	return nil
}

// CleanupOldImages retains only the keepCount most recent images in a repository
// and deletes the rest. Continues on individual delete failures.
func CleanupOldImages(ctx context.Context, c *artifacts.ArtifactsClient, compartmentID, repositoryName string, keepCount int) (*CleanupResult, error) {
	if keepCount < 0 {
		keepCount = 0
	}

	// Fetch all images (already sorted by TimeCreated DESC).
	images, err := ListContainerImages(ctx, c, compartmentID, repositoryName)
	if err != nil {
		return nil, err
	}

	result := &CleanupResult{
		RepositoryName: repositoryName,
		TotalImages:    len(images),
		KeepCount:      keepCount,
	}

	if len(images) <= keepCount {
		return result, nil
	}

	// Sort newest first (ListContainerImages already returns DESC, but be safe).
	sort.Slice(images, func(i, j int) bool {
		return images[i].TimeCreated > images[j].TimeCreated
	})

	// Delete images beyond keepCount.
	toDelete := images[keepCount:]
	for _, img := range toDelete {
		if err := DeleteContainerImage(ctx, c, img.ID); err != nil {
			// Log and continue with remaining images.
			continue
		}
		result.DeletedCount++
		result.DeletedImageIDs = append(result.DeletedImageIDs, img.ID)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func containerRepoToVO(r artifacts.ContainerRepositorySummary) ContainerRepositoryVO {
	vo := ContainerRepositoryVO{
		ID:             derefStr(r.Id),
		CompartmentID:  derefStr(r.CompartmentId),
		DisplayName:    derefStr(r.DisplayName),
		NamespaceName:  derefStr(r.NamespaceName),
		RepositoryName: derefStr(r.RepositoryName),
		LifecycleState: derefStr(r.LifecycleState),
	}
	if r.ImageCount != nil {
		vo.ImageCount = *r.ImageCount
	}
	if r.IsPublic != nil {
		vo.IsPublic = *r.IsPublic
	}
	if r.TimeCreated != nil {
		vo.TimeCreated = r.TimeCreated.Time.Format(timeLayout)
	}
	if r.TimeLastPushed != nil {
		vo.TimeLastPushed = r.TimeLastPushed.Time.Format(timeLayout)
	}
	return vo
}

func containerImageToVO(img artifacts.ContainerImageSummary) ContainerImageVO {
	vo := ContainerImageVO{
		ID:               derefStr(img.Id),
		CompartmentID:    derefStr(img.CompartmentId),
		RepositoryName:   derefStr(img.RepositoryName),
		DisplayName:      derefStr(img.DisplayName),
		Version:          derefStr(img.Version),
		LayersSizeInBytes: derefInt64(img.LayersSizeInBytes),
		SizeInBytes:      derefInt64(img.SizeInBytes),
		LifecycleState:   derefStr(img.LifecycleState),
	}
	if img.TimeCreated != nil {
		vo.TimeCreated = img.TimeCreated.Time.Format(timeLayout)
	}
	if img.TimeLastPulled != nil {
		vo.TimeLastPulled = img.TimeLastPulled.Time.Format(timeLayout)
	}
	return vo
}

// isOCI404 checks if an OCI SDK error is a 404 response.
func isOCI404(err error) bool {
	if err == nil {
		return false
	}
	if ociErr, ok := common.IsServiceError(err); ok {
		return ociErr.GetHTTPStatusCode() == 404
	}
	return false
}

// ensure unused import does not fail compilation.
var _ = http.StatusOK
