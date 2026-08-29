package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/aleshka5/auth-service/internal/users"
)

func (s *Server) handleAdminList(w http.ResponseWriter, r *http.Request) {
	list, err := s.users.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}
	out := make([]userJSON, 0, len(list))
	for i := range list {
		out = append(out, toUserJSON(&list[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

func (s *Server) handleAdminPatch(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Roles        *string `json:"roles"`
		StorageRoles *string `json:"storage_roles"`
		ResumeRoles  *string `json:"resume_roles"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	var patch users.RolePatch
	if body.Roles != nil {
		role, err := users.ParseRole(*body.Roles)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		patch.Roles = &role
	}
	if body.StorageRoles != nil {
		role, err := users.ParseServiceRole(*body.StorageRoles)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		patch.StorageRoles = &role
	}
	if body.ResumeRoles != nil {
		role, err := users.ParseServiceRole(*body.ResumeRoles)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		patch.ResumeRoles = &role
	}
	u, err := s.users.UpdateRoles(r.Context(), id, patch)
	if errors.Is(err, users.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if errors.Is(err, users.ErrLastAdmin) {
		writeError(w, http.StatusConflict, "cannot demote or block the last admin")
		return
	}
	if err != nil {
		slog.Error("admin_role_update_failed", "err", err, "user_id", id.String())
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}
	writeJSON(w, http.StatusOK, toUserJSON(u))
}
