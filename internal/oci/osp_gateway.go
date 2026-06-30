// Package oci — osp_gateway.go: OSP Gateway subscription details (Phase B).
// Parity with Java OspGatewayUtils: list subscriptions, get subscription info,
// discover home region from subscribed regions.
package oci

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/ospgateway"
)

// SubscriptionInfo holds OSP Gateway subscription details.
type SubscriptionInfo struct {
	Id                     string          `json:"id"`
	PlanType               string          `json:"planType"`
	AccountType            string          `json:"accountType"`
	TimeStart              *common.SDKTime `json:"timeStart"`
	CurrencyCode           string          `json:"currencyCode"`
	IsIntentToPay          bool            `json:"isIntentToPay"`
	SubscriptionPlanNumber string          `json:"subscriptionPlanNumber"`
	UpgradeState           string          `json:"upgradeState"`
	EmailAddress           string          `json:"emailAddress"`
	CompanyName            string          `json:"companyName"`
	Country                string          `json:"country"`
	LanguageCode           string          `json:"languageCode"`
}

// GetHomeRegionName discovers the home region name from subscribed regions.
func GetHomeRegionName(ctx context.Context, c Clients, tenancyOCID string) (string, error) {
	subs, err := ListSubscribedRegions(ctx, c, tenancyOCID)
	if err != nil {
		return "", fmt.Errorf("list subscribed regions: %w", err)
	}
	for _, s := range subs {
		if s.IsHomeRegion {
			return s.RegionName, nil
		}
	}
	return "", fmt.Errorf("home region not found in subscribed regions")
}

// GetSubscriptionInfo retrieves the OSP Gateway subscription details for a tenancy.
func GetSubscriptionInfo(ctx context.Context, c Clients, tenancyOCID string) (*SubscriptionInfo, error) {
	homeRegion, err := GetHomeRegionName(ctx, c, tenancyOCID)
	if err != nil {
		return nil, fmt.Errorf("get home region: %w", err)
	}

	// List subscriptions to find the subscription ID.
	listResp, err := c.OspGateway.ListSubscriptions(ctx, ospgateway.ListSubscriptionsRequest{
		CompartmentId: common.String(tenancyOCID),
		OspHomeRegion: common.String(homeRegion),
	})
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	if len(listResp.Items) == 0 {
		return nil, fmt.Errorf("no subscriptions found for tenancy %s", tenancyOCID)
	}

	subscriptionId := derefStr(listResp.Items[0].Id)

	// Get full subscription details.
	getResp, err := c.OspGateway.GetSubscription(ctx, ospgateway.GetSubscriptionRequest{
		CompartmentId:  common.String(tenancyOCID),
		SubscriptionId: common.String(subscriptionId),
		OspHomeRegion:  common.String(homeRegion),
	})
	if err != nil {
		return nil, fmt.Errorf("get subscription: %w", err)
	}

	s := getResp.Subscription
	info := &SubscriptionInfo{
		Id:                     derefStr(s.Id),
		PlanType:               string(s.PlanType),
		AccountType:            string(s.AccountType),
		TimeStart:              s.TimeStart,
		CurrencyCode:           derefStr(s.CurrencyCode),
		IsIntentToPay:          derefBool(s.IsIntentToPay),
		SubscriptionPlanNumber: derefStr(s.SubscriptionPlanNumber),
		UpgradeState:           string(s.UpgradeState),
		LanguageCode:           derefStr(s.LanguageCode),
	}
	if s.BillingAddress != nil {
		info.EmailAddress = derefStr(s.BillingAddress.EmailAddress)
		info.CompanyName = derefStr(s.BillingAddress.CompanyName)
		info.Country = derefStr(s.BillingAddress.Country)
	}
	return info, nil
}

// derefBool returns the bool value pointed to by v, or false if nil.
func derefBool(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}
