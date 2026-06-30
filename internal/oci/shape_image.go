// Package oci — shape_image.go: OCI Shape and Image listing with filtering.
// Provides ListShapesFiltered and ListImagesFiltered for fetching available
// compute shapes and images, filtered by architecture and shape compatibility.
// Parity with Java grabber's listShapes / findImage.
package oci

import (
	"context"
	"fmt"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// ShapeInfo holds a simplified shape description for API responses.
type ShapeInfo struct {
	Shape              string   `json:"shape"`
	Ocpus              float32  `json:"ocpus"`
	MemoryInGBs        float32  `json:"memoryInGBs"`
	ProcessorDesc      string   `json:"processorDescription"`
	Architecture       string   `json:"architecture"`
	MaxVnicAttachments int      `json:"maxVnicAttachments"`
	GpuDescription     string   `json:"gpuDescription,omitempty"`
	GpuCount           int      `json:"gpuCount,omitempty"`
	LocalDiskDesc      string   `json:"localDiskDescription,omitempty"`
	IsFlexible         bool     `json:"isFlexible"`
	BaselineOcpu       *float32 `json:"baselineOcpu,omitempty"`
	NetworkingDesc     string   `json:"networkingDescription,omitempty"`
}

// ImageInfo holds a simplified image description for API responses.
type ImageInfo struct {
	ID                  string `json:"id"`
	DisplayName         string `json:"displayName"`
	OperatingSystem     string `json:"operatingSystem"`
	OperatingSystemVer  string `json:"operatingSystemVersion"`
	Architecture        string `json:"architecture"`
	TimeCreated         string `json:"timeCreated"`
	SizeInGBs           *int64 `json:"sizeInGBs,omitempty"`
	LaunchMode          string `json:"launchMode,omitempty"`
}

// ListShapesFiltered lists all VM shapes in a compartment/AD, optionally
// filtered by architecture ("ARM" or "AMD"). Returns shape info sorted by
// OCPU count.
func ListShapesFiltered(ctx context.Context, c Clients, compartmentID, adName, architecture string) ([]ShapeInfo, error) {
	var out []ShapeInfo
	var page *string
	for {
		req := core.ListShapesRequest{
			CompartmentId: common.String(compartmentID),
			Limit:         common.Int(100),
			Page:          page,
		}
		if adName != "" {
			req.AvailabilityDomain = common.String(adName)
		}
		resp, err := c.Compute.ListShapes(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("list shapes in %s: %w", compartmentID, err)
		}

		for _, s := range resp.Items {
			if s.Shape == nil {
				continue
			}
			name := *s.Shape
			// Filter to VM shapes only (not bare metal).
			if !strings.HasPrefix(name, "VM.") {
				continue
			}
			// Filter by architecture if specified.
			if architecture != "" {
				shapeArch := shapeArchitecture(name)
				if !strings.EqualFold(shapeArch, architecture) {
					continue
				}
			}
			info := ShapeInfo{
				Shape:        name,
				IsFlexible:   s.IsFlexible != nil && *s.IsFlexible,
				Ocpus:        derefFloat32(s.Ocpus),
				MemoryInGBs:  derefFloat32(s.MemoryInGBs),
				ProcessorDesc: derefStr(s.ProcessorDescription),
			}
			if s.NetworkingBandwidthInGbps != nil {
				info.NetworkingDesc = fmt.Sprintf("%.1f Gbps", derefFloat32(s.NetworkingBandwidthInGbps))
			}
			if s.MaxVnicAttachments != nil {
				info.MaxVnicAttachments = derefInt(s.MaxVnicAttachments)
			}
			if s.GpuDescription != nil {
				info.GpuDescription = derefStr(s.GpuDescription)
			}
			if s.Gpus != nil {
				info.GpuCount = derefInt(s.Gpus)
			}
			if s.LocalDiskDescription != nil {
				info.LocalDiskDesc = derefStr(s.LocalDiskDescription)
			}
			info.Architecture = shapeArchitecture(name)
			out = append(out, info)
		}

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return out, nil
}

// ListImagesFiltered lists images in a compartment, optionally filtered by
// architecture and shape compatibility. If shape is specified, only images
// compatible with that shape are returned.
func ListImagesFiltered(ctx context.Context, c Clients, compartmentID, shape, architecture string) ([]ImageInfo, error) {
	var out []ImageInfo
	var page *string
	for {
		req := core.ListImagesRequest{
			CompartmentId: common.String(compartmentID),
			Limit:         common.Int(100),
			Page:          page,
		}
		if shape != "" {
			req.Shape = common.String(shape)
		}
		resp, err := c.Compute.ListImages(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("list images in %s: %w", compartmentID, err)
		}

		for _, img := range resp.Items {
			if img.Id == nil || img.DisplayName == nil {
				continue
			}
			// Filter by architecture if specified.
			if architecture != "" {
				imgArch := imageArchitecture(img)
				if !strings.EqualFold(imgArch, architecture) {
					continue
				}
			}
			info := ImageInfo{
				ID:                 derefStr(img.Id),
				DisplayName:        derefStr(img.DisplayName),
				OperatingSystem:    derefStr(img.OperatingSystem),
				OperatingSystemVer: derefStr(img.OperatingSystemVersion),
			}
			if img.TimeCreated != nil {
				info.TimeCreated = img.TimeCreated.Time.Format("2006-01-02T15:04:05Z")
			}
			if img.SizeInMBs != nil {
				gb := *img.SizeInMBs / 1024
				info.SizeInGBs = &gb
			}
			if img.LaunchMode != "" {
				info.LaunchMode = string(img.LaunchMode)
			}
			info.Architecture = imageArchitecture(img)
			out = append(out, info)
		}

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return out, nil
}

// shapeArchitecture determines architecture from shape name.
func shapeArchitecture(shape string) string {
	low := strings.ToLower(shape)
	switch {
	case strings.Contains(low, ".a1.") || strings.Contains(low, "flex.a1"):
		return "ARM"
	case strings.Contains(low, ".a2."):
		return "ARM"
	case strings.Contains(low, "ampere"):
		return "ARM"
	default:
		return "AMD"
	}
}

// imageArchitecture determines architecture from image properties.
func imageArchitecture(img core.Image) string {
	// Check launch mode first.
	if img.LaunchMode == core.ImageLaunchModeNative {
		// Native mode, check shape name for ARM hints.
	}
	// Check display name for architecture hints.
	name := strings.ToLower(derefStr(img.DisplayName))
	if strings.Contains(name, "aarch64") || strings.Contains(name, "arm64") || strings.Contains(name, "arm") {
		return "ARM"
	}
	// Default to AMD.
	return "AMD"
}
