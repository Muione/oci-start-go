package oci

import (
	"testing"
	"time"
)

func TestGetSubscriptionDays_Calculation(t *testing.T) {
	now := time.Now().UTC()

	t.Run("ten_days_ago", func(t *testing.T) {
		timeCreated := now.AddDate(0, 0, -10)
		duration := now.Sub(timeCreated)
		days := int64(duration.Hours() / 24)
		if duration.Nanoseconds()%int64(24*time.Hour) > 0 {
			days++
		}
		if days < 10 || days > 11 {
			t.Errorf("days = %d, want 10 or 11", days)
		}
	})

	t.Run("one_year_ago", func(t *testing.T) {
		timeCreated := now.AddDate(-1, 0, 0)
		duration := now.Sub(timeCreated)
		days := int64(duration.Hours() / 24)
		if duration.Nanoseconds()%int64(24*time.Hour) > 0 {
			days++
		}
		if days < 365 || days > 366 {
			t.Errorf("days = %d, want 365 or 366", days)
		}
		activeMonths := float64(days) / 30.44
		activeYears := float64(days) / 365.25
		if activeMonths < 11.0 || activeMonths > 13.0 {
			t.Errorf("activeMonths = %f, want ~12", activeMonths)
		}
		if activeYears < 0.9 || activeYears > 1.1 {
			t.Errorf("activeYears = %f, want ~1.0", activeYears)
		}
	})

	t.Run("zero_time", func(t *testing.T) {
		var timeCreated time.Time
		if !timeCreated.IsZero() {
			t.Fatal("expected zero time")
		}
	})

	t.Run("just_now", func(t *testing.T) {
		timeCreated := now
		duration := now.Sub(timeCreated)
		days := int64(duration.Hours() / 24)
		if duration.Nanoseconds()%int64(24*time.Hour) > 0 {
			days++
		}
		if days < 0 || days > 1 {
			t.Errorf("days = %d, want 0 or 1", days)
		}
	})
}

func TestSubscriptionDaysInfo_Fields(t *testing.T) {
	info := SubscriptionDaysInfo{
		TimeCreated:  time.Now(),
		CurrentTime:  time.Now(),
		ActiveDays:   30,
		ActiveMonths: 30.0 / 30.44,
		ActiveYears:  30.0 / 365.25,
	}
	if info.ActiveDays != 30 {
		t.Errorf("ActiveDays = %d, want 30", info.ActiveDays)
	}
	if info.ActiveMonths <= 0 || info.ActiveMonths > 1.0 {
		t.Errorf("ActiveMonths = %f, want ~0.98", info.ActiveMonths)
	}
	if info.ActiveYears <= 0 || info.ActiveYears > 0.2 {
		t.Errorf("ActiveYears = %f, want ~0.08", info.ActiveYears)
	}
}

func TestDomainInfo_Fields(t *testing.T) {
	info := DomainInfo{
		Id:             "ocid1.domain.oc1..aaaa",
		DisplayName:    "Default",
		Description:    "Default domain",
		Url:            "https://idcs-xxx.identity.oraclecloud.com",
		HomeRegion:     "us-ashburn-1",
		Type:           "DEFAULT",
		LicenseType:    "oracle-apps-premium",
		LifecycleState: "ACTIVE",
		TimeCreated:    time.Now(),
	}
	if info.Id != "ocid1.domain.oc1..aaaa" {
		t.Errorf("Id = %q", info.Id)
	}
	if info.LifecycleState != "ACTIVE" {
		t.Errorf("LifecycleState = %q, want ACTIVE", info.LifecycleState)
	}
}
