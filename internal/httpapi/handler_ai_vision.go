// Package httpapi -- handler_ai_vision.go: Phase 14.3 AI Vision management handlers.
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/response"
)

// aiVisionAnalyzeImage -- POST /oci/aivision/image/analyze
func aiVisionAnalyzeImage(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID int64 `json:"tenantId"`
			oci.AnalyzeImageInput
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		data, err := deps.AiVisionSvc.AnalyzeImage(c.Request.Context(), body.TenantID, body.AnalyzeImageInput)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}

// aiVisionAnalyzeDocument -- POST /oci/aivision/document/analyze
func aiVisionAnalyzeDocument(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID int64 `json:"tenantId"`
			oci.AnalyzeDocumentInput
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		data, err := deps.AiVisionSvc.AnalyzeDocument(c.Request.Context(), body.TenantID, body.AnalyzeDocumentInput)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}

// aiVisionCreateVideoJob -- POST /oci/aivision/video/create
func aiVisionCreateVideoJob(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID int64 `json:"tenantId"`
			oci.AnalyzeVideoInput
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		data, err := deps.AiVisionSvc.CreateVideoJob(c.Request.Context(), body.TenantID, body.AnalyzeVideoInput)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}

// aiVisionGetVideoJob -- GET /oci/aivision/video/status?tenantId=&videoJobId=
func aiVisionGetVideoJob(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, _ := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		videoJobID := c.Query("videoJobId")
		if tenantID == 0 || videoJobID == "" {
			response.Fail(c, http.StatusBadRequest, "tenantId and videoJobId required")
			return
		}
		data, err := deps.AiVisionSvc.GetVideoJob(c.Request.Context(), tenantID, videoJobID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}

// aiVisionCancelVideoJob -- POST /oci/aivision/video/cancel
func aiVisionCancelVideoJob(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID   int64  `json:"tenantId"`
			VideoJobID string `json:"videoJobId"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if err := deps.AiVisionSvc.CancelVideoJob(c.Request.Context(), body.TenantID, body.VideoJobID); err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}
