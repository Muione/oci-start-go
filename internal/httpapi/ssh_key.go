// Package httpapi — ssh_key.go: CRUD for stored SSH private keys (encrypted at
// rest by service.SSHKeyService). The key content is sent by the client ONLY on
// create; list returns id/label/fingerprint (no content). The WS SSH handler
// resolves a key by id at connect time so the material stays server-side.
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
)

// sshKeysList returns all stored SSH keys (id/label/fingerprint; no content).
func sshKeysList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.SSHKeySvc == nil {
			response.Fail(c, http.StatusInternalServerError, "ssh key service not configured")
			return
		}
		views, err := deps.SSHKeySvc.List(c.Request.Context())
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.SuccessData(views))
	}
}

// sshKeyCreate stores a labeled private key (encrypted). The content is
// validated + encrypted server-side; it is never returned again.
func sshKeyCreate(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.SSHKeySvc == nil {
			response.Fail(c, http.StatusInternalServerError, "ssh key service not configured")
			return
		}
		var body struct {
			Label      string `json:"label"`
			Content    string `json:"content"`
			Passphrase string `json:"passphrase"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
		if body.Label == "" || body.Content == "" {
			response.Fail(c, http.StatusBadRequest, "label and content required")
			return
		}
		id, err := deps.SSHKeySvc.Create(c.Request.Context(), body.Label, body.Content, body.Passphrase)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.SuccessData(map[string]any{"id": id}))
	}
}

// sshKeyDelete removes a stored key by id.
func sshKeyDelete(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.SSHKeySvc == nil {
			response.Fail(c, http.StatusInternalServerError, "ssh key service not configured")
			return
		}
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			response.Fail(c, http.StatusBadRequest, "invalid id")
			return
		}
		if err := deps.SSHKeySvc.Delete(c.Request.Context(), id); err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}
