// Package service -- ai_vision.go: Phase 14.3 AI Vision service.
// Resolves tenant credentials from DB, builds OCI clients via oci.WithProxy,
// and delegates to OCI AI Vision SDK wrappers for image analysis, document
// analysis, and async video analysis.
package service

import (
	"context"
	"fmt"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
)

// AIVisionService provides OCI AI Vision operations per tenant.
type AIVisionService struct {
	store     *db.Store
	masterKey []byte
	pool      *oci.ProxyPool
}

// NewAIVisionService constructs an AIVisionService.
func NewAIVisionService(store *db.Store, masterKey []byte, pool *oci.ProxyPool) *AIVisionService {
	return &AIVisionService{store: store, masterKey: masterKey, pool: pool}
}

// AnalyzeImage analyzes an image (inline base64 or Object Storage reference)
// using OCI AI Vision classification, object detection, or text recognition.
func (s *AIVisionService) AnalyzeImage(ctx context.Context, tenantID int64, in oci.AnalyzeImageInput) (*oci.AnalyzeImageOutput, error) {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := oci.Credentials{
		Tenancy:     nsStr(t.Tenancy),
		UserID:      nsStr(t.TenantID),
		Fingerprint: nsStr(t.Fingerprint),
		Region:      nsStr(t.Region),
		KeyFileBlob: nsStr(t.KeyFileBlob),
		KeyFile:     nsStr(t.KeyFile),
	}

	var result *oci.AnalyzeImageOutput
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		var innerErr error
		result, innerErr = oci.AnalyzeImage(ctx, c.AiVision, in)
		return innerErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// AnalyzeDocument analyzes a document from Object Storage using OCI AI Vision
// for table detection, key-value extraction, or language classification.
func (s *AIVisionService) AnalyzeDocument(ctx context.Context, tenantID int64, in oci.AnalyzeDocumentInput) (*oci.AnalyzeDocumentOutput, error) {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := oci.Credentials{
		Tenancy:     nsStr(t.Tenancy),
		UserID:      nsStr(t.TenantID),
		Fingerprint: nsStr(t.Fingerprint),
		Region:      nsStr(t.Region),
		KeyFileBlob: nsStr(t.KeyFileBlob),
		KeyFile:     nsStr(t.KeyFile),
	}

	var result *oci.AnalyzeDocumentOutput
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		var innerErr error
		result, innerErr = oci.AnalyzeDocument(ctx, c.AiVision, in)
		return innerErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// CreateVideoJob starts an async video analysis job using OCI AI Vision.
func (s *AIVisionService) CreateVideoJob(ctx context.Context, tenantID int64, in oci.AnalyzeVideoInput) (*oci.AnalyzeVideoOutput, error) {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := oci.Credentials{
		Tenancy:     nsStr(t.Tenancy),
		UserID:      nsStr(t.TenantID),
		Fingerprint: nsStr(t.Fingerprint),
		Region:      nsStr(t.Region),
		KeyFileBlob: nsStr(t.KeyFileBlob),
		KeyFile:     nsStr(t.KeyFile),
	}

	var result *oci.AnalyzeVideoOutput
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		var innerErr error
		result, innerErr = oci.AnalyzeVideo(ctx, c.AiVision, in)
		return innerErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetVideoJob returns the current status of a video analysis job.
func (s *AIVisionService) GetVideoJob(ctx context.Context, tenantID int64, videoJobID string) (*oci.VideoAnalysisStatus, error) {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := oci.Credentials{
		Tenancy:     nsStr(t.Tenancy),
		UserID:      nsStr(t.TenantID),
		Fingerprint: nsStr(t.Fingerprint),
		Region:      nsStr(t.Region),
		KeyFileBlob: nsStr(t.KeyFileBlob),
		KeyFile:     nsStr(t.KeyFile),
	}

	var result *oci.VideoAnalysisStatus
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		var innerErr error
		result, innerErr = oci.GetVideoAnalysisStatus(ctx, c.AiVision, videoJobID)
		return innerErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// CancelVideoJob cancels a running video analysis job.
func (s *AIVisionService) CancelVideoJob(ctx context.Context, tenantID int64, videoJobID string) error {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := oci.Credentials{
		Tenancy:     nsStr(t.Tenancy),
		UserID:      nsStr(t.TenantID),
		Fingerprint: nsStr(t.Fingerprint),
		Region:      nsStr(t.Region),
		KeyFileBlob: nsStr(t.KeyFileBlob),
		KeyFile:     nsStr(t.KeyFile),
	}

	return oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		return oci.CancelVideoAnalysis(ctx, c.AiVision, videoJobID)
	})
}
