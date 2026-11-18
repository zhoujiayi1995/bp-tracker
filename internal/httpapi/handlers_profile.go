package httpapi

import (
	"context"
	"net/http"

	"bptracker/internal/auth"
	"bptracker/internal/authz"
	"bptracker/internal/profile"
)

type profileDTO struct {
	ID             int64  `json:"id"`
	DisplayName    string `json:"displayName"`
	Gender         string `json:"gender"`
	BirthYear      *int   `json:"birthYear"`
	MedicalHistory string `json:"medicalHistory"`
	Medications    string `json:"medications"`
	IsSelf         bool   `json:"isSelf"`
	IsManaged      bool   `json:"isManaged"`
	ManagerName    string `json:"managerName,omitempty"`
	CanWrite       bool   `json:"canWrite"`
}

type profileRequest struct {
	DisplayName    string `json:"displayName"`
	Gender         string `json:"gender"`
	BirthYear      *int   `json:"birthYear"`
	MedicalHistory string `json:"medicalHistory"`
	Medications    string `json:"medications"`
}

func (req profileRequest) fields() profile.Fields {
	f := profile.Fields{
		DisplayName:    req.DisplayName,
		Gender:         req.Gender,
		MedicalHistory: req.MedicalHistory,
		Medications:    req.Medications,
	}
	if req.BirthYear != nil {
		f.BirthYear = *req.BirthYear
	}
	return f
}

func toProfileDTO(p profile.Profile, u auth.User) profileDTO {
	dto := profileDTO{
		ID:             p.ID,
		DisplayName:    p.DisplayName,
		Gender:         p.Gender,
		MedicalHistory: p.MedicalHistory,
		Medications:    p.Medications,
		IsSelf:         p.UserID == u.ID,
		IsManaged:      p.IsManaged(),
		CanWrite:       authz.CanWrite(u.Subject(), p.Target()),
	}
	if p.IsManaged() {
		dto.ManagerName = p.ManagerName
	}
	if p.BirthYear != 0 {
		by := p.BirthYear
		dto.BirthYear = &by
	}
	return dto
}

func (s *Server) listProfiles(w http.ResponseWriter, r *http.Request, u auth.User) error {
	list, err := profile.ListVisible(r.Context(), s.db, u.Subject())
	if err != nil {
		return err
	}
	out := make([]profileDTO, 0, len(list))
	for _, p := range list {
		out = append(out, toProfileDTO(p, u))
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) createProfile(w http.ResponseWriter, r *http.Request, u auth.User) error {
	var req profileRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return err
	}
	id, err := profile.CreateManaged(r.Context(), s.db, u.ID, req.fields(), s.now())
	if err != nil {
		return err
	}
	p, err := profile.Get(r.Context(), s.db, id)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, toProfileDTO(p, u))
	return nil
}

func (s *Server) updateProfile(w http.ResponseWriter, r *http.Request, u auth.User) error {
	p, err := s.profileForWrite(r, u)
	if err != nil {
		return err
	}
	var req profileRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return err
	}
	if err := profile.Update(r.Context(), s.db, p.ID, req.fields(), s.now()); err != nil {
		return err
	}
	updated, err := profile.Get(r.Context(), s.db, p.ID)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, toProfileDTO(updated, u))
	return nil
}

func (s *Server) deleteProfile(w http.ResponseWriter, r *http.Request, u auth.User) error {
	p, err := s.profileForWrite(r, u)
	if err != nil {
		return err
	}
	// Self profiles live and die with their account.
	if !p.IsManaged() {
		return errForbidden
	}
	if err := profile.Delete(r.Context(), s.db, p.ID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) profileForRead(r *http.Request, u auth.User) (profile.Profile, error) {
	id, err := pathID(r)
	if err != nil {
		return profile.Profile{}, err
	}
	return s.loadProfile(r.Context(), u, id, authz.CanRead)
}

func (s *Server) profileForWrite(r *http.Request, u auth.User) (profile.Profile, error) {
	id, err := pathID(r)
	if err != nil {
		return profile.Profile{}, err
	}
	return s.loadProfile(r.Context(), u, id, authz.CanWrite)
}

func (s *Server) loadProfile(ctx context.Context, u auth.User, id int64, allowed func(authz.Subject, authz.Target) bool) (profile.Profile, error) {
	p, err := profile.Get(ctx, s.db, id)
	if err != nil {
		return profile.Profile{}, err
	}
	if !allowed(u.Subject(), p.Target()) {
		return profile.Profile{}, errForbidden
	}
	return p, nil
}
