package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
)

// billingSubscription — GET /tenants/:id/subscription
func billingSubscription(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid tenant ID")
			return
		}

		result, err := deps.BillingSvc.GetSubscriptionDetail(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, response.SuccessData(result))
	}
}

// billingCost — GET /tenants/:id/cost?type=current_month&start=&end=
func billingCost(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid tenant ID")
			return
		}

		queryType := c.DefaultQuery("type", "current_month")
		startDate := c.Query("start")
		endDate := c.Query("end")

		if queryType == "custom" && (startDate == "" || endDate == "") {
			response.Fail(c, http.StatusBadRequest, "Custom date range requires start and end parameters (format: 2006-01-02)")
			return
		}

		result, err := deps.BillingSvc.QueryCost(c.Request.Context(), id, queryType, startDate, endDate)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, response.SuccessData(result))
	}
}
