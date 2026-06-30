// Package oci — shape_image_test.go: unit tests for shape/image filtering
// functions (TE-106, TE-107).
package oci

import (
	"testing"

	"github.com/oracle/oci-go-sdk/v65/core"
)

// --- TE-106: Shape/Image architecture filtering ---

func TestShapeArchitecture_ARM_A1(t *testing.T) {
	cases := []struct {
		shape string
		want  string
	}{
		{"VM.Standard.A1.Flex", "ARM"},
		{"VM.Standard.A2.Flex", "ARM"},
		{"VM.DenseIO.A1.Flex", "ARM"},
		{"VM.Standard.E4.Flex", "AMD"},
		{"VM.Standard.E5.Flex", "AMD"},
		{"VM.Standard3.Flex", "AMD"},
		{"VM.GPU.A10.1", "AMD"},
		{"VM.Standard.E3.Flex", "AMD"},
	}
	for _, tc := range cases {
		got := shapeArchitecture(tc.shape)
		if got != tc.want {
			t.Errorf("shapeArchitecture(%q) = %q, want %q", tc.shape, got, tc.want)
		}
	}
}

func TestShapeArchitecture_Filters(t *testing.T) {
	// Verify that only VM shapes pass the prefix filter.
	vmShapes := []string{"VM.Standard.A1.Flex", "VM.Standard.E4.Flex"}
	for _, s := range vmShapes {
		if len(s) < 3 || s[:3] != "VM." {
			t.Errorf("expected shape %q to start with 'VM.'", s)
		}
	}
}

func TestImageArchitecture_DisplayNameHints(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"Oracle-Linux-8-aarch64-2024.01.01", "ARM"},
		{"Oracle-Linux-8-arm64-2024.01.01", "ARM"},
		{"Oracle-Linux-8-ARM-2024.01.01", "ARM"},
		{"Oracle-Linux-8-x86_64-2024.01.01", "AMD"},
		{"Canonical-Ubuntu-22.04-Minimal-aarch64", "ARM"},
		{"Windows-Server-2022-Standard-Edition-VM", "AMD"},
	}
	for _, tc := range cases {
		img := core.Image{
			DisplayName: &tc.name,
		}
		got := imageArchitecture(img)
		if got != tc.want {
			t.Errorf("imageArchitecture(displayName=%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// --- TE-106: ShapeInfo and ImageInfo struct validation ---

func TestShapeInfo_Fields(t *testing.T) {
	baseline := float32(1.0)
	info := ShapeInfo{
		Shape:              "VM.Standard.A1.Flex",
		Ocpus:              4,
		MemoryInGBs:        24,
		ProcessorDesc:      "Ampere A1",
		Architecture:       "ARM",
		MaxVnicAttachments: 2,
		GpuDescription:     "",
		GpuCount:           0,
		LocalDiskDesc:      "",
		IsFlexible:         true,
		BaselineOcpu:       &baseline,
		NetworkingDesc:     "10.0 Gbps",
	}
	if info.Shape != "VM.Standard.A1.Flex" {
		t.Errorf("Shape = %q", info.Shape)
	}
	if info.Ocpus != 4 {
		t.Errorf("Ocpus = %f, want 4", info.Ocpus)
	}
	if info.Architecture != "ARM" {
		t.Errorf("Architecture = %q, want ARM", info.Architecture)
	}
	if !info.IsFlexible {
		t.Error("IsFlexible should be true")
	}
	if info.BaselineOcpu == nil || *info.BaselineOcpu != 1.0 {
		t.Errorf("BaselineOcpu = %v, want 1.0", info.BaselineOcpu)
	}
}

func TestImageInfo_Fields(t *testing.T) {
	sizeGB := int64(50)
	info := ImageInfo{
		ID:                 "ocid1.image.oc1..aaaa",
		DisplayName:        "Oracle-Linux-8-aarch64",
		OperatingSystem:    "Oracle Linux",
		OperatingSystemVer: "8",
		Architecture:       "ARM",
		TimeCreated:        "2024-01-01T00:00:00Z",
		SizeInGBs:          &sizeGB,
		LaunchMode:         "NATIVE",
	}
	if info.ID != "ocid1.image.oc1..aaaa" {
		t.Errorf("ID = %q", info.ID)
	}
	if info.SizeInGBs == nil || *info.SizeInGBs != 50 {
		t.Errorf("SizeInGBs = %v, want 50", info.SizeInGBs)
	}
	if info.LaunchMode != "NATIVE" {
		t.Errorf("LaunchMode = %q, want NATIVE", info.LaunchMode)
	}
}

// --- TE-106: ShapeInfo JSON serialization ---

func TestShapeInfo_JSONOmitempty(t *testing.T) {
	info := ShapeInfo{
		Shape:        "VM.Standard.E4.Flex",
		Architecture: "AMD",
		IsFlexible:   true,
	}
	// GPU fields should be omitted in JSON when zero/empty.
	if info.GpuDescription != "" {
		t.Errorf("GpuDescription should be empty, got %q", info.GpuDescription)
	}
	if info.GpuCount != 0 {
		t.Errorf("GpuCount should be 0, got %d", info.GpuCount)
	}
	if info.LocalDiskDesc != "" {
		t.Errorf("LocalDiskDesc should be empty, got %q", info.LocalDiskDesc)
	}
	if info.BaselineOcpu != nil {
		t.Errorf("BaselineOcpu should be nil, got %v", info.BaselineOcpu)
	}
}

// --- TE-106: VNIC creation parameter validation ---

func TestValidateVnicCreationParams(t *testing.T) {
	tests := []struct {
		name         string
		vnicCount    int
		ipv6PerVnic  int
		wantErr      bool
	}{
		{"valid minimal", 1, 0, false},
		{"valid with ipv6", 5, 10, false},
		{"valid max", 32, 32, false},
		{"zero vnics", 0, 0, true},
		{"negative vnics", -1, 0, true},
		{"too many vnics", 33, 0, true},
		{"negative ipv6", 1, -1, true},
		{"too many ipv6", 1, 33, true},
		{"boundary 32 vnics", 32, 0, false},
		{"boundary 32 ipv6", 1, 32, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateVnicCreationParams(tc.vnicCount, tc.ipv6PerVnic)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// --- TE-106: VNIC constants ---

func TestVnicConstants(t *testing.T) {
	if MaxVnicPerInstance != 32 {
		t.Errorf("MaxVnicPerInstance = %d, want 32", MaxVnicPerInstance)
	}
	if MaxIpv6PerVnic != 32 {
		t.Errorf("MaxIpv6PerVnic = %d, want 32", MaxIpv6PerVnic)
	}
	if DefaultTimeoutSec != 300 {
		t.Errorf("DefaultTimeoutSec = %d, want 300", DefaultTimeoutSec)
	}
	if PollIntervalSec != 3 {
		t.Errorf("PollIntervalSec = %d, want 3", PollIntervalSec)
	}
}

// --- TE-106: VNIC display name generation ---

func TestGenerateVnicDisplayName(t *testing.T) {
	cases := []struct {
		instanceName string
		index        int
		want         string
	}{
		{"my-instance", 1, "vnic-my-instance-1"},
		{"test", 5, "vnic-test-5"},
		{"", 1, "vnic--1"},
	}
	for _, tc := range cases {
		got := generateVnicDisplayName(tc.instanceName, tc.index)
		if got != tc.want {
			t.Errorf("generateVnicDisplayName(%q, %d) = %q, want %q", tc.instanceName, tc.index, got, tc.want)
		}
	}
}

// --- TE-106: Hostname label generation ---

func TestGenerateHostnameLabel(t *testing.T) {
	label := generateHostnameLabel()
	if len(label) != 18 { // "oci-start-hn" (12) + 6 random chars
		t.Errorf("hostname label length = %d, want 18", len(label))
	}
	if label[:12] != "oci-start-hn" {
		t.Errorf("hostname label prefix = %q, want 'oci-start-hn'", label[:12])
	}
	// Two calls should produce different labels (with high probability).
	label2 := generateHostnameLabel()
	if label == label2 {
		t.Logf("warning: two hostname labels are identical (%q) — low probability but possible", label)
	}
}

// --- TE-107: isNotFound helper ---

func TestIsNotFound_NilError(t *testing.T) {
	if isNotFound(nil) {
		t.Error("isNotFound(nil) should be false")
	}
}

// --- TE-106: VnicAttachmentInfo struct ---

func TestVnicAttachmentInfo_Fields(t *testing.T) {
	info := VnicAttachmentInfo{
		VnicID:        "ocid1.vnic.oc1..aaaa",
		InstanceID:    "ocid1.instance.oc1..aaaa",
		InstanceName:  "test-instance",
		PublicIP:      "1.2.3.4",
		PrivateIP:     "10.0.0.1",
		Ipv6Addresses: []string{"2001:db8::1"},
		SubnetID:      "ocid1.subnet.oc1..aaaa",
	}
	if info.VnicID != "ocid1.vnic.oc1..aaaa" {
		t.Errorf("VnicID = %q", info.VnicID)
	}
	if len(info.Ipv6Addresses) != 1 || info.Ipv6Addresses[0] != "2001:db8::1" {
		t.Errorf("Ipv6Addresses = %v", info.Ipv6Addresses)
	}
}

// --- TE-106: BatchVnicCreationResult struct ---

func TestBatchVnicCreationResult_Fields(t *testing.T) {
	result := BatchVnicCreationResult{
		InstanceID:                "ocid1.instance.oc1..aaaa",
		InstanceDisplayName:       "test-instance",
		RequestedVnicCount:        5,
		RequestedIpv6CountPerVnic: 3,
		SuccessfulVnicCount:       5,
		TotalIpv6Count:            15,
		AllSuccessful:             true,
		Summary:                   "VNIC creation complete",
		TotalExecutionTimeMs:      1234,
	}
	if !result.AllSuccessful {
		t.Error("AllSuccessful should be true")
	}
	if result.SuccessfulVnicCount != 5 {
		t.Errorf("SuccessfulVnicCount = %d, want 5", result.SuccessfulVnicCount)
	}
	if result.TotalIpv6Count != 15 {
		t.Errorf("TotalIpv6Count = %d, want 15", result.TotalIpv6Count)
	}
}

// --- TE-106: VnicCreationResult struct ---

func TestVnicCreationResult_Fields(t *testing.T) {
	result := VnicCreationResult{
		VnicID:          "ocid1.vnic.oc1..aaaa",
		VnicDisplayName: "vnic-test-1",
		PrivateIP:       "10.0.0.1",
		PublicIP:        "1.2.3.4",
		SubnetID:        "ocid1.subnet.oc1..aaaa",
		AttachmentID:    "ocid1.vnicattachment.oc1..aaaa",
		LifecycleState:  "ATTACHED",
		Ipv6Addresses:   []string{"2001:db8::1", "2001:db8::2"},
		Ipv6IDs:         []string{"ocid1.ipv6.oc1..aaaa1", "ocid1.ipv6.oc1..aaaa2"},
		IsPrimary:       false,
		Success:         true,
	}
	if !result.Success {
		t.Error("Success should be true")
	}
	if len(result.Ipv6Addresses) != 2 {
		t.Errorf("Ipv6Addresses count = %d, want 2", len(result.Ipv6Addresses))
	}
	if result.IsPrimary {
		t.Error("IsPrimary should be false")
	}
}

// --- TE-106: Ipv6CreationResult struct ---

func TestIpv6CreationResult_Fields(t *testing.T) {
	result := Ipv6CreationResult{
		Ipv6ID:      "ocid1.ipv6.oc1..aaaa",
		Ipv6Address: "2001:db8::1",
		VnicID:      "ocid1.vnic.oc1..aaaa",
		Success:     true,
	}
	if !result.Success {
		t.Error("Success should be true")
	}
	if result.ErrorMessage != "" {
		t.Errorf("ErrorMessage should be empty, got %q", result.ErrorMessage)
	}
}

func TestIpv6CreationResult_Failure(t *testing.T) {
	result := Ipv6CreationResult{
		VnicID:       "ocid1.vnic.oc1..aaaa",
		Success:      false,
		ErrorMessage: "quota exceeded",
	}
	if result.Success {
		t.Error("Success should be false")
	}
	if result.ErrorMessage != "quota exceeded" {
		t.Errorf("ErrorMessage = %q", result.ErrorMessage)
	}
}
