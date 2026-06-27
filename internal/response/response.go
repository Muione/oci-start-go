// Package response defines the unified JSON envelope (parity with Java
// oci-common/param/ApiResponse: {success, message, data, code} — NO timestamp).
// Shared by auth middleware and httpapi handlers to avoid an import cycle.
// See SPEC §6.3.
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Code    int         `json:"code"`
}

func Success() ApiResponse                  { return ApiResponse{Success: true, Message: "success", Code: 200} }
func SuccessData(data any) ApiResponse      { return ApiResponse{Success: true, Message: "success", Data: data, Code: 200} }
func SuccessMsg(msg string) ApiResponse     { return ApiResponse{Success: true, Message: msg, Code: 200} }
func SuccessMsgData(msg string, data any) ApiResponse {
	return ApiResponse{Success: true, Message: msg, Data: data, Code: 200}
}
func Error(msg string) ApiResponse            { return ApiResponse{Success: false, Message: msg, Code: 500} }
func ErrorWithCode(msg string, code int) ApiResponse {
	return ApiResponse{Success: false, Message: msg, Code: code}
}

// OK writes a 200 with the given ApiResponse.
func OK(c *gin.Context, r ApiResponse) { c.JSON(http.StatusOK, r) }

// Fail writes httpStatus with {success:false, message, code:httpStatus} and
// ABORTS the chain (so middleware rejections don't fall through to the handler).
func Fail(c *gin.Context, httpStatus int, msg string) {
	c.AbortWithStatusJSON(httpStatus, ApiResponse{Success: false, Message: msg, Code: httpStatus})
}
