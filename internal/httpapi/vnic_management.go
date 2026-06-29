// Package httpapi — vnic_management.go: Phase 11.2 VNIC Management endpoints.
// 10 handlers under /oci/vnic for batch VNIC create/delete, IPv6 management,
// IP switch, and load balancer configuration. Parity with Java
// VnicManagementController.
package httpapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
)

// --- Request structs ---

type vnicCreateReq struct {
	InstanceID       string `json:"instanceId"`
	SubnetID         string `json:"subnetId"`
	VnicCount        int    `json:"vnicCount"`
	Ipv6CountPerVnic int    `json:"ipv6CountPerVnic"`
}

type vnicDeleteReq struct {
	InstanceID string `json:"instanceId"`
	VnicID     string `json:"vnicId"`
}

type vnicCreateIpv6Req struct {
	VnicID     string `json:"vnicId"`
	Ipv6Count  int    `json:"ipv6Count"`
	InstanceID string `json:"instanceId"`
}

type vnicDeleteIpv6Req struct {
	Ipv6Address string `json:"ipv6Address"`
	VnicID      string `json:"vnicId"`
	InstanceID  string `json:"instanceId"`
}

type vnicDeleteAllReq struct {
	InstanceID string `json:"instanceId"`
}

type vnicChangeSpecIpReq struct {
	InstanceID string   `json:"instanceId"`
	VnicID     string   `json:"vnicId"`
	CidrRanges []string `json:"cidrRanges"`
}

type vnicLBReq struct {
	InstanceID string `json:"instanceId"`
}

// --- Handlers ---

// vnicLoadData — GET /oci/vnic/loadData?instanceId=
func vnicLoadData(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		instanceID := c.Query("instanceId")
		if instanceID == "" {
			response.Fail(c, http.StatusBadRequest, "参数不完整")
			return
		}
		result, err := deps.VnicMgmtSvc.LoadData(c.Request.Context(), instanceID, true)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "数据加载失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsgData("数据加载成功", result))
	}
}

// vnicCreate — POST /oci/vnic/create
func vnicCreate(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req vnicCreateReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "参数不完整")
			return
		}
		if req.InstanceID == "" || req.SubnetID == "" {
			response.Fail(c, http.StatusBadRequest, "参数不完整")
			return
		}
		result, err := deps.VnicMgmtSvc.CreateVnics(c.Request.Context(), req.InstanceID, req.SubnetID, req.VnicCount, req.Ipv6CountPerVnic)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		if result.AllSuccessful {
			response.OK(c, response.SuccessMsg(result.Summary))
		} else {
			// Partial failure: HTTP 200 but success=false in body.
			response.OK(c, response.Error(result.Summary))
		}
	}
}

// vnicDelete — POST /oci/vnic/delete
func vnicDelete(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req vnicDeleteReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "参数不完整")
			return
		}
		if req.InstanceID == "" || req.VnicID == "" {
			response.Fail(c, http.StatusBadRequest, "参数不完整")
			return
		}
		if err := deps.VnicMgmtSvc.DeleteVnic(c.Request.Context(), req.InstanceID, req.VnicID); err != nil {
			response.Fail(c, http.StatusInternalServerError, "VNIC删除失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("VNIC删除成功"))
	}
}

// vnicCreateIpv6 — POST /oci/vnic/createIpv6
func vnicCreateIpv6(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req vnicCreateIpv6Req
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "参数不完整")
			return
		}
		if req.VnicID == "" || req.InstanceID == "" {
			response.Fail(c, http.StatusBadRequest, "参数不完整")
			return
		}
		results, err := deps.VnicMgmtSvc.CreateIpv6(c.Request.Context(), req.InstanceID, req.VnicID, req.Ipv6Count)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "IPv6创建失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsgData("IPv6地址创建完成", results))
	}
}

// vnicDeleteIpv6 — POST /oci/vnic/deleteIpv6
func vnicDeleteIpv6(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req vnicDeleteIpv6Req
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "参数不完整")
			return
		}
		if req.Ipv6Address == "" || req.VnicID == "" || req.InstanceID == "" {
			response.Fail(c, http.StatusBadRequest, "参数不完整")
			return
		}
		if err := deps.VnicMgmtSvc.DeleteIpv6(c.Request.Context(), req.InstanceID, req.VnicID, req.Ipv6Address); err != nil {
			response.Fail(c, http.StatusInternalServerError, "IPv6删除失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("IPv6地址删除成功"))
	}
}

// vnicDeleteAllSecondary — POST /oci/vnic/deleteAllSecondary
func vnicDeleteAllSecondary(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req vnicDeleteAllReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "参数不完整")
			return
		}
		if req.InstanceID == "" {
			response.Fail(c, http.StatusBadRequest, "参数不完整")
			return
		}
		resultMap, err := deps.VnicMgmtSvc.DeleteAllSecondary(c.Request.Context(), req.InstanceID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "删除失败: "+err.Error())
			return
		}
		// Count successes.
		successCount := 0
		for _, ok := range resultMap {
			if ok {
				successCount++
			}
		}
		msg := "辅助VNIC删除完成"
		if len(resultMap) > 0 {
			msg = "辅助VNIC删除完成 - 成功: " + fmt.Sprintf("%d/%d", successCount, len(resultMap))
		}
		response.OK(c, response.SuccessMsgData(msg, resultMap))
	}
}

// vnicRefresh — GET /oci/vnic/refresh?instanceId=
func vnicRefresh(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		instanceID := c.Query("instanceId")
		if instanceID == "" {
			response.Fail(c, http.StatusBadRequest, "参数不完整")
			return
		}
		result, err := deps.VnicMgmtSvc.RefreshVnicInfo(c.Request.Context(), instanceID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "刷新失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsgData("数据加载成功", result))
	}
}

// vnicChangeSpecIp — POST /oci/vnic/changeSpecIp
func vnicChangeSpecIp(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req vnicChangeSpecIpReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "参数不完整")
			return
		}
		if req.InstanceID == "" || req.VnicID == "" || len(req.CidrRanges) == 0 {
			response.Fail(c, http.StatusBadRequest, "参数不完整")
			return
		}
		result, err := deps.VnicMgmtSvc.ChangeSpecIp(c.Request.Context(), req.InstanceID, req.VnicID, req.CidrRanges)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"status": "error", "message": "IP切换失败: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "IP切换成功", "details": result})
	}
}

// vnicConfigureLB — POST /oci/vnic/network/configureLoadBalancer
func vnicConfigureLB(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req vnicLBReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "参数不完整")
			return
		}
		if req.InstanceID == "" {
			response.Fail(c, http.StatusBadRequest, "参数不完整")
			return
		}
		result, err := deps.VnicMgmtSvc.ConfigureLoadBalancer(c.Request.Context(), req.InstanceID)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		response.OK(c, response.SuccessMsgData("实例网络配置完成", result))
	}
}

// vnicRestoreNetwork — POST /oci/vnic/network/restoreNetwork
func vnicRestoreNetwork(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req vnicLBReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "参数不完整")
			return
		}
		if req.InstanceID == "" {
			response.Fail(c, http.StatusBadRequest, "参数不完整")
			return
		}
		if err := deps.VnicMgmtSvc.RestoreNetwork(c.Request.Context(), req.InstanceID); err != nil {
			response.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("网络配置已成功还原到原始状态"))
	}
}
