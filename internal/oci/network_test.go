// Package oci — network_test.go: unit tests for GetPrimaryVnic's core helper
// (P-3). Asserts GetVnic is called at most once per attachment (no 2x
// redundancy between the primary scan and the fallback).
package oci

import (
	"context"
	"errors"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/core"
)

// TestGetPrimaryVnicFromAttachments_NoPrimarySinglePass: N attachments, none
// marked primary. The fallback must reuse the already-fetched Vnic instead of
// re-calling GetVnic. Redundant implementation calls GetVnic 2N times.
func TestGetPrimaryVnicFromAttachments_NoPrimarySinglePass(t *testing.T) {
	const N = 3
	ids := []string{"vnic-a", "vnic-b", "vnic-c"}
	attachments := []core.VnicAttachment{
		{VnicId: &ids[0]},
		{VnicId: &ids[1]},
		{VnicId: &ids[2]},
	}
	calls := 0
	got, err := getPrimaryVnicFromAttachments(context.Background(), attachments, "inst-1",
		func(ctx context.Context, vnicID *string) (core.Vnic, error) {
			calls++
			return core.Vnic{Id: vnicID}, nil // IsPrimary nil -> not primary
		})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if calls > N {
		t.Errorf("GetVnic called %d times, want <= %d (no 2x fallback redundancy)", calls, N)
	}
	if got.Id == nil || *got.Id != ids[0] {
		t.Errorf("returned vnic id = %v, want first reachable %q", got.Id, ids[0])
	}
}

// TestGetPrimaryVnicFromAttachments_PrimaryFound: primary is the last
// attachment. Must return it; GetVnic must not exceed the attachment count.
func TestGetPrimaryVnicFromAttachments_PrimaryFound(t *testing.T) {
	ids := []string{"vnic-a", "vnic-b", "vnic-c"}
	attachments := []core.VnicAttachment{
		{VnicId: &ids[0]},
		{VnicId: &ids[1]},
		{VnicId: &ids[2]},
	}
	primary := "vnic-c"
	calls := 0
	got, err := getPrimaryVnicFromAttachments(context.Background(), attachments, "inst-1",
		func(ctx context.Context, vnicID *string) (core.Vnic, error) {
			calls++
			isPrim := *vnicID == primary
			return core.Vnic{Id: vnicID, IsPrimary: &isPrim}, nil
		})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got.Id == nil || *got.Id != primary {
		t.Errorf("returned vnic id = %v, want %q", got.Id, primary)
	}
	if calls > len(ids) {
		t.Errorf("GetVnic called %d times, want <= %d", calls, len(ids))
	}
}

// TestGetPrimaryVnicFromAttachments_ErrorsNoRepeat: some GetVnic calls error,
// none is primary. The fallback must not re-call attachments already fetched
// in the first pass.
func TestGetPrimaryVnicFromAttachments_ErrorsNoRepeat(t *testing.T) {
	ids := []string{"vnic-a", "vnic-b", "vnic-c"}
	attachments := []core.VnicAttachment{
		{VnicId: &ids[0]},
		{VnicId: &ids[1]},
		{VnicId: &ids[2]},
	}
	called := map[string]int{}
	got, err := getPrimaryVnicFromAttachments(context.Background(), attachments, "inst-1",
		func(ctx context.Context, vnicID *string) (core.Vnic, error) {
			called[*vnicID]++
			if *vnicID == "vnic-a" {
				return core.Vnic{}, errors.New("transient")
			}
			return core.Vnic{Id: vnicID}, nil // not primary
		})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got.Id == nil || *got.Id != "vnic-b" {
		t.Errorf("returned vnic id = %v, want vnic-b (first reachable)", got.Id)
	}
	for id, n := range called {
		if n > 1 {
			t.Errorf("GetVnic(%q) called %d times, want <= 1 (no repeat in fallback)", id, n)
		}
	}
}

// TestGetPrimaryVnicFromAttachments_NoneReachable: all GetVnic calls error ->
// error result, no panic.
func TestGetPrimaryVnicFromAttachments_NoneReachable(t *testing.T) {
	id := "vnic-a"
	attachments := []core.VnicAttachment{{VnicId: &id}}
	calls := 0
	_, err := getPrimaryVnicFromAttachments(context.Background(), attachments, "inst-1",
		func(ctx context.Context, vnicID *string) (core.Vnic, error) {
			calls++
			return core.Vnic{}, errors.New("unreachable")
		})
	if err == nil {
		t.Fatal("err = nil, want error")
	}
	if calls != 1 {
		t.Errorf("GetVnic called %d times, want 1", calls)
	}
}
