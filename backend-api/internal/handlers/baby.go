package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/andreistefanciprian/yauli/backend-api/internal/authctx"
	"github.com/andreistefanciprian/yauli/backend-api/internal/store"
)

// defaultBabyTimezone matches babies.timezone's DB default (0001_init.sql) —
// applied here rather than left to the DB default so an explicit empty
// string in the request doesn't defeat it.
const defaultBabyTimezone = "Australia/Adelaide"

// requireClaims returns the caller's claims, writing a 401 and returning
// ok=false if the request carried none.
func requireClaims(w http.ResponseWriter, r *http.Request) (authctx.Claims, bool) {
	claims, ok := authctx.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
	}
	return claims, ok
}

// GetCurrentBaby returns the caller's family's "current" baby. A family is
// never named or shown in the UI, so this is the only baby-scoped read the
// frontend needs before it has a specific baby id to work with. It trusts
// claims.FamilyID as decoded from the JWT rather than re-querying family
// membership, so it only sees a family created by CreateBaby once the caller
// holds a token re-minted after that (auth-service's attach-family, a later
// PR) - a stale token from before the family existed still reports 404 here.
func (h *Handlers) GetCurrentBaby(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	if claims.FamilyID == nil {
		writeError(w, http.StatusNotFound, "baby not found")
		return
	}

	baby, err := h.Store.GetCurrentBaby(r.Context(), *claims.FamilyID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "baby not found")
		return
	}
	if err != nil {
		log.Printf("get current baby: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load baby")
		return
	}

	writeJSON(w, http.StatusOK, baby)
}

type createBabyRequest struct {
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
}

// CreateBaby adds a baby for the caller. A user with no existing family
// membership gets one created implicitly, as this baby's owner, in the same
// call — family is a backend-only tenancy boundary, never a concept the UI
// asks the user about (see the PR plan). A user who already belongs to a
// family just gets a sibling baby added to it.
func (h *Handlers) CreateBaby(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}

	var req createBabyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Timezone == "" {
		req.Timezone = defaultBabyTimezone
	}

	membership, err := h.FamilyStore.GetFamilyMembership(r.Context(), claims.UserID)
	if err != nil {
		log.Printf("get family membership for create baby: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load family membership")
		return
	}

	familyID := membership.FamilyID
	switch {
	case familyID == nil:
		// familyName is never shown to users - it only exists to satisfy
		// families.name NOT NULL.
		newFamilyID, err := h.FamilyStore.CreateFamilyWithOwner(r.Context(), claims.UserID, fmt.Sprintf("family-%s", claims.UserID))
		if errors.Is(err, store.ErrActiveMembershipExists) {
			// Lost a race with a concurrent CreateBaby call for the same
			// brand-new user (e.g. a double-submitted "add your baby" form)
			// - re-fetch rather than fail, since the other call already
			// created the family.
			membership, err = h.FamilyStore.GetFamilyMembership(r.Context(), claims.UserID)
			if err != nil {
				log.Printf("get family membership after create-family race: %v", err)
				writeError(w, http.StatusInternalServerError, "failed to load family membership")
				return
			}
			if membership.FamilyID == nil {
				writeError(w, http.StatusInternalServerError, "failed to create family")
				return
			}
			familyID = membership.FamilyID
		} else if err != nil {
			log.Printf("create family for new baby: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to create family")
			return
		} else {
			familyID = &newFamilyID
		}
	case membership.Status == store.MembershipStatusInvited:
		// The caller's only membership is a pending invite they haven't
		// logged in to accept yet (ActivateInvitedMembership normally runs
		// at login) - activate it now rather than let them write into a
		// family they're not formally an active member of.
		if err := h.FamilyStore.ActivateInvitedMembership(r.Context(), claims.UserID, *familyID); err != nil {
			log.Printf("activate invited membership for create baby: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to activate family membership")
			return
		}
	}

	baby, err := h.Store.CreateBaby(r.Context(), *familyID, req.Name, req.Timezone)
	if err != nil {
		log.Printf("create baby: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create baby")
		return
	}

	writeJSON(w, http.StatusCreated, baby)
}
