package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/andreistefanciprian/yauli/backend-api/internal/store"
)

type inviteHelperRequest struct {
	Email string `json:"email"`
}

// InviteHelper invites another person to help with a baby. The route is
// baby-scoped because "family" is an implementation detail: callers name the
// baby they want help with, and the handler resolves that baby's family
// internally before checking ownership.
func (h *Handlers) InviteHelper(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}

	babyID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid baby id")
		return
	}

	var req inviteHelperRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}

	baby, err := h.Store.GetBaby(r.Context(), babyID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "baby not found")
		return
	}
	if err != nil {
		log.Printf("get baby for invite: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load baby")
		return
	}

	membership, err := h.FamilyStore.GetFamilyMembershipForFamily(r.Context(), claims.UserID, baby.FamilyID)
	if err != nil {
		log.Printf("get membership for invite: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load membership")
		return
	}
	if !membership.Found || membership.Role != store.MembershipRoleOwner || membership.Status != store.MembershipStatusActive {
		writeError(w, http.StatusForbidden, "only the owner can invite helpers")
		return
	}

	if err := h.FamilyStore.CreateInvite(r.Context(), baby.FamilyID, email); err != nil {
		log.Printf("create baby-scoped invite: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to invite helper")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "invited"})
}
