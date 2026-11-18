package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"bptracker/internal/auth"
	"bptracker/internal/db"
	"bptracker/internal/family"
)

type familyDTO struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	InviteCode string `json:"inviteCode"`
	CreatedBy  int64  `json:"createdBy"`
}

type memberDTO struct {
	UserID      int64  `json:"userId"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	IsMe        bool   `json:"isMe"`
}

type familyResponse struct {
	Family  *familyDTO  `json:"family"`
	Members []memberDTO `json:"members"`
}

func (s *Server) getFamily(w http.ResponseWriter, r *http.Request, u auth.User) error {
	resp, err := s.familyView(r.Context(), u.FamilyID, u.ID)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, resp)
	return nil
}

func (s *Server) createFamily(w http.ResponseWriter, r *http.Request, u auth.User) error {
	var req struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		return err
	}
	var f family.Family
	err := db.WithTx(r.Context(), s.db, func(tx *sql.Tx) error {
		var err error
		f, err = family.Create(r.Context(), tx, u.ID, req.Name, s.now())
		return err
	})
	if err != nil {
		return err
	}
	return s.writeFamily(w, r, f.ID, u.ID, http.StatusCreated)
}

func (s *Server) joinFamily(w http.ResponseWriter, r *http.Request, u auth.User) error {
	var req struct {
		InviteCode string `json:"inviteCode"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		return err
	}
	key := "join|" + strconv.FormatInt(u.ID, 10)
	if ok, wait := s.joinLimiter.Allow(key); !ok {
		return tooManyAttempts(wait.Seconds())
	}
	var f family.Family
	err := db.WithTx(r.Context(), s.db, func(tx *sql.Tx) error {
		var err error
		f, err = family.Join(r.Context(), tx, u.ID, req.InviteCode)
		return err
	})
	if errors.Is(err, family.ErrInvalidInviteCode) {
		s.joinLimiter.Fail(key)
	}
	if err != nil {
		return err
	}
	s.joinLimiter.Reset(key)
	return s.writeFamily(w, r, f.ID, u.ID, http.StatusOK)
}

func (s *Server) leaveFamily(w http.ResponseWriter, r *http.Request, u auth.User) error {
	err := db.WithTx(r.Context(), s.db, func(tx *sql.Tx) error {
		return family.Leave(r.Context(), tx, u.ID)
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) resetInviteCode(w http.ResponseWriter, r *http.Request, u auth.User) error {
	err := db.WithTx(r.Context(), s.db, func(tx *sql.Tx) error {
		_, err := family.ResetInviteCode(r.Context(), tx, u.ID)
		return err
	})
	if err != nil {
		return err
	}
	return s.writeFamily(w, r, u.FamilyID, u.ID, http.StatusOK)
}

func (s *Server) writeFamily(w http.ResponseWriter, r *http.Request, familyID, meID int64, status int) error {
	resp, err := s.familyView(r.Context(), familyID, meID)
	if err != nil {
		return err
	}
	writeJSON(w, status, resp)
	return nil
}

func (s *Server) familyView(ctx context.Context, familyID, meID int64) (familyResponse, error) {
	resp := familyResponse{Members: []memberDTO{}}
	if familyID == 0 {
		return resp, nil
	}
	f, err := family.Get(ctx, s.db, familyID)
	if err != nil {
		return familyResponse{}, err
	}
	members, err := family.Members(ctx, s.db, familyID)
	if err != nil {
		return familyResponse{}, err
	}
	resp.Family = &familyDTO{ID: f.ID, Name: f.Name, InviteCode: f.InviteCode, CreatedBy: f.CreatedBy}
	for _, m := range members {
		resp.Members = append(resp.Members, memberDTO{
			UserID: m.UserID, Username: m.Username, DisplayName: m.DisplayName, IsMe: m.UserID == meID,
		})
	}
	return resp, nil
}
