// Package httpapi -- network_mgmt.go: VCN, NAT gateway, and route table
// management handlers. These wrap OCI SDK functions for frontend consumption.
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"

	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/response"
)

// --- Response DTOs ---

type vcnInfoDTO struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	CidrBlock   string `json:"cidrBlock"`
	DnsLabel    string `json:"dnsLabel"`
	TimeCreated string `json:"timeCreated,omitempty"`
}

type natGatewayDTO struct {
	ID             string `json:"id"`
	DisplayName    string `json:"displayName"`
	VcnID          string `json:"vcnId"`
	LifecycleState string `json:"lifecycleState"`
}

type routeTableDTO struct {
	ID          string          `json:"id"`
	DisplayName string          `json:"displayName"`
	RouteRules  []routeRuleDTO  `json:"routeRules,omitempty"`
}

type routeRuleDTO struct {
	Destination     string `json:"destination"`
	DestinationType string `json:"destinationType"`
	NetworkEntityId string `json:"networkEntityId"`
}

// --- Helpers ---

func strPtr(s string) *string { return &s }

func ds(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// --- VCN ---

// vcnList -- GET /oci/vcn/list?tenantId=&compartmentId=
func vcnList(deps *Deps) gin.HandlerFunc {
	return func(ginCtx *gin.Context) {
		tenantID, err := strconv.ParseInt(ginCtx.Query("tenantId"), 10, 64)
		if err != nil || tenantID <= 0 {
			response.Fail(ginCtx, http.StatusBadRequest, "valid tenantId required")
			return
		}
		compartmentID := ginCtx.Query("compartmentId")
		if compartmentID == "" {
			response.Fail(ginCtx, http.StatusBadRequest, "compartmentId required")
			return
		}

		t, err := repo.New(deps.Store.Read).FindTenantByID(ginCtx.Request.Context(), tenantID)
		if err != nil {
			response.Fail(ginCtx, http.StatusNotFound, "tenant not found")
			return
		}
		creds := tenantToCreds(t)

		var vcns []vcnInfoDTO
		ctx := ginCtx.Request.Context()
		err = oci.WithProxy(ctx, deps.ProxyPool, creds, deps.MasterKey, func(clients oci.Clients) error {
			items, innerErr := oci.ListVcns(ctx, clients, compartmentID)
			if innerErr != nil {
				return innerErr
			}
			vcns = make([]vcnInfoDTO, 0, len(items))
			for _, v := range items {
				info := vcnInfoDTO{
					ID:          ds(v.Id),
					DisplayName: ds(v.DisplayName),
					CidrBlock:   ds(v.CidrBlock),
					DnsLabel:    ds(v.DnsLabel),
				}
				if v.TimeCreated != nil {
					info.TimeCreated = v.TimeCreated.Format("2006-01-02 15:04:05")
				}
				vcns = append(vcns, info)
			}
			return nil
		})
		if err != nil {
			response.Fail(ginCtx, http.StatusInternalServerError, "list vcns: "+err.Error())
			return
		}
		response.OK(ginCtx, response.SuccessData(vcns))
	}
}

// vcnConfigureIPv6 -- POST /oci/vcn/configure-ipv6 {tenantId, vcnId}
func vcnConfigureIPv6(deps *Deps) gin.HandlerFunc {
	return func(ginCtx *gin.Context) {
		var body struct {
			TenantID int64  `json:"tenantId"`
			VcnID    string `json:"vcnId"`
		}
		if err := ginCtx.ShouldBindJSON(&body); err != nil {
			response.Fail(ginCtx, http.StatusBadRequest, "invalid body")
			return
		}
		if body.TenantID <= 0 || body.VcnID == "" {
			response.Fail(ginCtx, http.StatusBadRequest, "tenantId and vcnId required")
			return
		}

		t, err := repo.New(deps.Store.Read).FindTenantByID(ginCtx.Request.Context(), body.TenantID)
		if err != nil {
			response.Fail(ginCtx, http.StatusNotFound, "tenant not found")
			return
		}
		creds := tenantToCreds(t)

		ctx := ginCtx.Request.Context()
		err = oci.WithProxy(ctx, deps.ProxyPool, creds, deps.MasterKey, func(clients oci.Clients) error {
			return oci.ConfigureIPv6SecurityRules(ctx, clients, body.VcnID)
		})
		if err != nil {
			response.Fail(ginCtx, http.StatusInternalServerError, "configure ipv6: "+err.Error())
			return
		}
		response.OK(ginCtx, response.SuccessMsg("IPv6 security rules configured"))
	}
}

// --- NAT Gateway ---

// natList -- GET /oci/nat/list?tenantId=&compartmentId=&vcnId=
func natList(deps *Deps) gin.HandlerFunc {
	return func(ginCtx *gin.Context) {
		tenantID, err := strconv.ParseInt(ginCtx.Query("tenantId"), 10, 64)
		if err != nil || tenantID <= 0 {
			response.Fail(ginCtx, http.StatusBadRequest, "valid tenantId required")
			return
		}
		compartmentID := ginCtx.Query("compartmentId")
		vcnID := ginCtx.Query("vcnId")
		if compartmentID == "" || vcnID == "" {
			response.Fail(ginCtx, http.StatusBadRequest, "compartmentId and vcnId required")
			return
		}

		t, err := repo.New(deps.Store.Read).FindTenantByID(ginCtx.Request.Context(), tenantID)
		if err != nil {
			response.Fail(ginCtx, http.StatusNotFound, "tenant not found")
			return
		}
		creds := tenantToCreds(t)

		var nats []natGatewayDTO
		ctx := ginCtx.Request.Context()
		err = oci.WithProxy(ctx, deps.ProxyPool, creds, deps.MasterKey, func(clients oci.Clients) error {
			var page *string
			for {
				resp, listErr := clients.Vcn.ListNatGateways(ctx, core.ListNatGatewaysRequest{
					CompartmentId: common.String(compartmentID),
					VcnId:         common.String(vcnID),
					Page:          page,
				})
				if listErr != nil {
					return listErr
				}
				for _, gw := range resp.Items {
					nats = append(nats, natGatewayDTO{
						ID:             ds(gw.Id),
						DisplayName:    ds(gw.DisplayName),
						VcnID:          ds(gw.VcnId),
						LifecycleState: string(gw.LifecycleState),
					})
				}
				if resp.OpcNextPage == nil {
					break
				}
				page = resp.OpcNextPage
			}
			return nil
		})
		if err != nil {
			response.Fail(ginCtx, http.StatusInternalServerError, "list nat gateways: "+err.Error())
			return
		}
		response.OK(ginCtx, response.SuccessData(nats))
	}
}

// natCreate -- POST /oci/nat/create {tenantId, compartmentId, vcnId, displayName}
func natCreate(deps *Deps) gin.HandlerFunc {
	return func(ginCtx *gin.Context) {
		var body struct {
			TenantID      int64  `json:"tenantId"`
			CompartmentID string `json:"compartmentId"`
			VcnID         string `json:"vcnId"`
			DisplayName   string `json:"displayName"`
		}
		if err := ginCtx.ShouldBindJSON(&body); err != nil {
			response.Fail(ginCtx, http.StatusBadRequest, "invalid body")
			return
		}
		if body.TenantID <= 0 || body.CompartmentID == "" || body.VcnID == "" || body.DisplayName == "" {
			response.Fail(ginCtx, http.StatusBadRequest, "all fields required")
			return
		}

		t, err := repo.New(deps.Store.Read).FindTenantByID(ginCtx.Request.Context(), body.TenantID)
		if err != nil {
			response.Fail(ginCtx, http.StatusNotFound, "tenant not found")
			return
		}
		creds := tenantToCreds(t)

		var natID, natName string
		ctx := ginCtx.Request.Context()
		err = oci.WithProxy(ctx, deps.ProxyPool, creds, deps.MasterKey, func(clients oci.Clients) error {
			gw, createErr := oci.CreateOrGetNatGateway(ctx, clients.Vcn, body.CompartmentID, body.VcnID, body.DisplayName)
			if createErr != nil {
				return createErr
			}
			natID = ds(gw.Id)
			natName = ds(gw.DisplayName)
			return nil
		})
		if err != nil {
			response.Fail(ginCtx, http.StatusInternalServerError, "create nat gateway: "+err.Error())
			return
		}
		response.OK(ginCtx, response.SuccessData(map[string]string{
			"id":          natID,
			"displayName": natName,
		}))
	}
}

// natDelete -- GET /oci/nat/delete?tenantId=&natGatewayId=
func natDelete(deps *Deps) gin.HandlerFunc {
	return func(ginCtx *gin.Context) {
		tenantID, err := strconv.ParseInt(ginCtx.Query("tenantId"), 10, 64)
		if err != nil || tenantID <= 0 {
			response.Fail(ginCtx, http.StatusBadRequest, "valid tenantId required")
			return
		}
		natGatewayID := ginCtx.Query("natGatewayId")
		if natGatewayID == "" {
			response.Fail(ginCtx, http.StatusBadRequest, "natGatewayId required")
			return
		}

		t, err := repo.New(deps.Store.Read).FindTenantByID(ginCtx.Request.Context(), tenantID)
		if err != nil {
			response.Fail(ginCtx, http.StatusNotFound, "tenant not found")
			return
		}
		creds := tenantToCreds(t)

		ctx := ginCtx.Request.Context()
		err = oci.WithProxy(ctx, deps.ProxyPool, creds, deps.MasterKey, func(clients oci.Clients) error {
			return oci.DeleteNatGateway(ctx, clients.Vcn, natGatewayID)
		})
		if err != nil {
			response.Fail(ginCtx, http.StatusInternalServerError, "delete nat gateway: "+err.Error())
			return
		}
		response.OK(ginCtx, response.Success())
	}
}

// --- Route Table ---

// routeTableList -- GET /oci/route-table/list?tenantId=&compartmentId=&vcnId=
func routeTableList(deps *Deps) gin.HandlerFunc {
	return func(ginCtx *gin.Context) {
		tenantID, err := strconv.ParseInt(ginCtx.Query("tenantId"), 10, 64)
		if err != nil || tenantID <= 0 {
			response.Fail(ginCtx, http.StatusBadRequest, "valid tenantId required")
			return
		}
		compartmentID := ginCtx.Query("compartmentId")
		vcnID := ginCtx.Query("vcnId")
		if compartmentID == "" || vcnID == "" {
			response.Fail(ginCtx, http.StatusBadRequest, "compartmentId and vcnId required")
			return
		}

		t, err := repo.New(deps.Store.Read).FindTenantByID(ginCtx.Request.Context(), tenantID)
		if err != nil {
			response.Fail(ginCtx, http.StatusNotFound, "tenant not found")
			return
		}
		creds := tenantToCreds(t)

		var rts []routeTableDTO
		ctx := ginCtx.Request.Context()
		err = oci.WithProxy(ctx, deps.ProxyPool, creds, deps.MasterKey, func(clients oci.Clients) error {
			var page *string
			for {
				resp, listErr := clients.Vcn.ListRouteTables(ctx, core.ListRouteTablesRequest{
					CompartmentId: common.String(compartmentID),
					VcnId:         common.String(vcnID),
					Page:          page,
				})
				if listErr != nil {
					return listErr
				}
				for _, rt := range resp.Items {
					dto := routeTableDTO{
						ID:          ds(rt.Id),
						DisplayName: ds(rt.DisplayName),
					}
					for _, rule := range rt.RouteRules {
						dto.RouteRules = append(dto.RouteRules, routeRuleDTO{
							Destination:     ds(rule.Destination),
							DestinationType: string(rule.DestinationType),
							NetworkEntityId: ds(rule.NetworkEntityId),
						})
					}
					rts = append(rts, dto)
				}
				if resp.OpcNextPage == nil {
					break
				}
				page = resp.OpcNextPage
			}
			return nil
		})
		if err != nil {
			response.Fail(ginCtx, http.StatusInternalServerError, "list route tables: "+err.Error())
			return
		}
		response.OK(ginCtx, response.SuccessData(rts))
	}
}

// routeTableCreate -- POST /oci/route-table/create {tenantId, compartmentId, vcnId, displayName, natGatewayId?}
func routeTableCreate(deps *Deps) gin.HandlerFunc {
	return func(ginCtx *gin.Context) {
		var body struct {
			TenantID      int64  `json:"tenantId"`
			CompartmentID string `json:"compartmentId"`
			VcnID         string `json:"vcnId"`
			DisplayName   string `json:"displayName"`
			NatGatewayID  string `json:"natGatewayId"`
		}
		if err := ginCtx.ShouldBindJSON(&body); err != nil {
			response.Fail(ginCtx, http.StatusBadRequest, "invalid body")
			return
		}
		if body.TenantID <= 0 || body.CompartmentID == "" || body.VcnID == "" || body.DisplayName == "" {
			response.Fail(ginCtx, http.StatusBadRequest, "required fields missing")
			return
		}

		t, err := repo.New(deps.Store.Read).FindTenantByID(ginCtx.Request.Context(), body.TenantID)
		if err != nil {
			response.Fail(ginCtx, http.StatusNotFound, "tenant not found")
			return
		}
		creds := tenantToCreds(t)

		var rtID, rtName string
		ctx := ginCtx.Request.Context()
		err = oci.WithProxy(ctx, deps.ProxyPool, creds, deps.MasterKey, func(clients oci.Clients) error {
			if body.NatGatewayID != "" {
				rt, createErr := oci.CreateOrGetNatRouteTable(ctx, clients.Vcn, body.CompartmentID, body.VcnID, body.NatGatewayID, body.DisplayName)
				if createErr != nil {
					return createErr
				}
				rtID = ds(rt.Id)
				rtName = ds(rt.DisplayName)
			} else {
				resp, createErr := clients.Vcn.CreateRouteTable(ctx, core.CreateRouteTableRequest{
					CreateRouteTableDetails: core.CreateRouteTableDetails{
						CompartmentId: common.String(body.CompartmentID),
						VcnId:         common.String(body.VcnID),
						DisplayName:   common.String(body.DisplayName),
						RouteRules:    []core.RouteRule{},
					},
				})
				if createErr != nil {
					return createErr
				}
				rtID = ds(resp.RouteTable.Id)
				rtName = ds(resp.RouteTable.DisplayName)
			}
			return nil
		})
		if err != nil {
			response.Fail(ginCtx, http.StatusInternalServerError, "create route table: "+err.Error())
			return
		}
		response.OK(ginCtx, response.SuccessData(map[string]string{
			"id":          rtID,
			"displayName": rtName,
		}))
	}
}

// routeTableDelete -- GET /oci/route-table/delete?tenantId=&routeTableId=
func routeTableDelete(deps *Deps) gin.HandlerFunc {
	return func(ginCtx *gin.Context) {
		tenantID, err := strconv.ParseInt(ginCtx.Query("tenantId"), 10, 64)
		if err != nil || tenantID <= 0 {
			response.Fail(ginCtx, http.StatusBadRequest, "valid tenantId required")
			return
		}
		routeTableID := ginCtx.Query("routeTableId")
		if routeTableID == "" {
			response.Fail(ginCtx, http.StatusBadRequest, "routeTableId required")
			return
		}

		t, err := repo.New(deps.Store.Read).FindTenantByID(ginCtx.Request.Context(), tenantID)
		if err != nil {
			response.Fail(ginCtx, http.StatusNotFound, "tenant not found")
			return
		}
		creds := tenantToCreds(t)

		ctx := ginCtx.Request.Context()
		err = oci.WithProxy(ctx, deps.ProxyPool, creds, deps.MasterKey, func(clients oci.Clients) error {
			return oci.DeleteRouteTable(ctx, clients.Vcn, routeTableID)
		})
		if err != nil {
			response.Fail(ginCtx, http.StatusInternalServerError, "delete route table: "+err.Error())
			return
		}
		response.OK(ginCtx, response.Success())
	}
}

// routeTableReset -- POST /oci/route-table/reset {tenantId, instanceId, compartmentId}
func routeTableReset(deps *Deps) gin.HandlerFunc {
	return func(ginCtx *gin.Context) {
		var body struct {
			TenantID      int64  `json:"tenantId"`
			InstanceID    string `json:"instanceId"`
			CompartmentID string `json:"compartmentId"`
		}
		if err := ginCtx.ShouldBindJSON(&body); err != nil {
			response.Fail(ginCtx, http.StatusBadRequest, "invalid body")
			return
		}
		if body.TenantID <= 0 || body.InstanceID == "" || body.CompartmentID == "" {
			response.Fail(ginCtx, http.StatusBadRequest, "all fields required")
			return
		}

		t, err := repo.New(deps.Store.Read).FindTenantByID(ginCtx.Request.Context(), body.TenantID)
		if err != nil {
			response.Fail(ginCtx, http.StatusNotFound, "tenant not found")
			return
		}
		creds := tenantToCreds(t)

		ctx := ginCtx.Request.Context()
		err = oci.WithProxy(ctx, deps.ProxyPool, creds, deps.MasterKey, func(clients oci.Clients) error {
			return oci.ResetVnicToDefaultRouteTable(ctx, clients, body.InstanceID, body.CompartmentID)
		})
		if err != nil {
			response.Fail(ginCtx, http.StatusInternalServerError, "reset route table: "+err.Error())
			return
		}
		response.OK(ginCtx, response.SuccessMsg("route table reset to default"))
	}
}
