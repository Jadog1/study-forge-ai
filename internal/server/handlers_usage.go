package server

import (
	"net/http"
	"time"

	"github.com/studyforge/study-agent/internal/state"
)

func (s *Server) handleUsageTotals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	filter := state.UsageFilter{}
	q := r.URL.Query()

	if after := q.Get("after"); after != "" {
		t, err := time.Parse(time.RFC3339, after)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "invalid after time: "+err.Error())
			return
		}
		filter.CreatedAfter = &t
	}
	if before := q.Get("before"); before != "" {
		t, err := time.Parse(time.RFC3339, before)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "invalid before time: "+err.Error())
			return
		}
		filter.CreatedBefore = &t
	}

	cfg := s.Config()
	totals, err := s.Store().Usage().LoadUsageTotalsWithPricing(cfg, filter)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "load usage: "+err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, totals)
}

func (s *Server) handleUsageLedger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ledger, err := s.Store().Usage().LoadUsageLedger()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "load ledger: "+err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, ledger)
}
