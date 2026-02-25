package api

import (
	"net/http"

	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/sdk"
)

// --- Read handlers ---

func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	filter := parseQueryFilter(r)
	result, err := s.client.Browse(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toBrowseResponse(result))
}

func (s *Server) handleDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := s.client.Detail(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toDetailResponse(result))
}

func (s *Server) handleDashboard(w http.ResponseWriter, _ *http.Request) {
	data, err := s.client.Dashboard()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toDashboardResponse(data))
}

func (s *Server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, ConfigResponse{
		RigHandle: s.client.RigHandle(),
		Mode:      s.client.Mode(),
	})
}

// --- Mutation handlers ---

func (s *Server) handlePost(w http.ResponseWriter, r *http.Request) {
	var req PostRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	result, err := s.client.Post(sdk.PostInput{
		Title:       req.Title,
		Description: req.Description,
		Project:     req.Project,
		Type:        req.Type,
		Priority:    req.Priority,
		EffortLevel: req.EffortLevel,
		Tags:        req.Tags,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toMutationResponse(result))
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req UpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	fields := &commons.WantedUpdate{
		Title:       req.Title,
		Description: req.Description,
		Project:     req.Project,
		Type:        req.Type,
		Priority:    -1,
		EffortLevel: req.EffortLevel,
		Tags:        req.Tags,
		TagsSet:     req.TagsSet,
	}
	if req.Priority != nil {
		fields.Priority = *req.Priority
	}
	result, err := s.client.Update(id, fields)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toMutationResponse(result))
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := s.client.Delete(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toMutationResponse(result))
}

func (s *Server) handleClaim(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := s.client.Claim(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toMutationResponse(result))
}

func (s *Server) handleUnclaim(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := s.client.Unclaim(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toMutationResponse(result))
}

func (s *Server) handleDone(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req DoneRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Evidence == "" {
		writeError(w, http.StatusBadRequest, "evidence is required")
		return
	}
	result, err := s.client.Done(id, req.Evidence)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toMutationResponse(result))
}

func (s *Server) handleAccept(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req AcceptRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	result, err := s.client.Accept(id, sdk.AcceptInput{
		Quality:     req.Quality,
		Reliability: req.Reliability,
		Severity:    req.Severity,
		SkillTags:   req.SkillTags,
		Message:     req.Message,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toMutationResponse(result))
}

func (s *Server) handleReject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req RejectRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	result, err := s.client.Reject(id, req.Reason)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toMutationResponse(result))
}

func (s *Server) handleClose(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := s.client.Close(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toMutationResponse(result))
}

// --- Branch handlers ---

func (s *Server) handleApplyBranch(w http.ResponseWriter, r *http.Request) {
	branch := r.PathValue("branch")
	if err := s.client.ApplyBranch(branch); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "applied"})
}

func (s *Server) handleDiscardBranch(w http.ResponseWriter, r *http.Request) {
	branch := r.PathValue("branch")
	if err := s.client.DiscardBranch(branch); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "discarded"})
}

func (s *Server) handleSubmitPR(w http.ResponseWriter, r *http.Request) {
	branch := r.PathValue("branch")
	url, err := s.client.SubmitPR(branch)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, PRResponse{URL: url})
}

func (s *Server) handleBranchDiff(w http.ResponseWriter, r *http.Request) {
	branch := r.PathValue("branch")
	diff, err := s.client.BranchDiff(branch)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, DiffResponse{Diff: diff})
}

// --- Settings handlers ---

func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	var req SettingsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Mode != "wild-west" && req.Mode != "pr" {
		writeError(w, http.StatusBadRequest, "mode must be \"wild-west\" or \"pr\"")
		return
	}
	if err := s.client.SaveSettings(req.Mode, req.Signing); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (s *Server) handleSync(w http.ResponseWriter, _ *http.Request) {
	if err := s.client.Sync(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "synced"})
}
