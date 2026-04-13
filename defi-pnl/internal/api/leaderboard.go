package api

import (
	"encoding/json"
	"net/http"
	"time"

	"defi-pnl/internal/storage"
)

const leaderboardTopN = 10

// --- Row types ---

type leaderboardRowJSON struct {
	Rank      int    `json:"rank"`
	TxAddress string `json:"tx_address"`
	TxType    string `json:"tx_type,omitempty"` // U = origin aggregate, B = sender aggregate (omitted when implied by URL)
	Volume    string `json:"volume"`
	Date      string `json:"date,omitempty"` // all-time: peak single-day
}

// Combined daily / all-time (backward compatible)
type leaderboardDailyResponse struct {
	Date  string               `json:"date"`
	Users []leaderboardRowJSON `json:"users"`
	Bots  []leaderboardRowJSON `json:"bots"`
	Limit int                  `json:"limit"`
	Empty bool                 `json:"empty,omitempty"`
	Error string               `json:"error,omitempty"`
}

type leaderboardAllTimeResponse struct {
	Users []leaderboardRowJSON `json:"users"`
	Bots  []leaderboardRowJSON `json:"bots"`
	Limit int                  `json:"limit"`
	Empty bool                 `json:"empty,omitempty"`
	Error string               `json:"error,omitempty"`
}

// Single-scope response: one top-10 list (users OR bots).
type leaderboardScopeResponse struct {
	Scope string               `json:"scope"`          // "users" | "bots"
	Kind  string               `json:"kind,omitempty"` // "all_time" | "daily"
	Date  string               `json:"date,omitempty"` // daily only YYYY-MM-DD
	Top   []leaderboardRowJSON `json:"top"`
	Limit int                  `json:"limit"`
	Empty bool                 `json:"empty,omitempty"`
	Error string               `json:"error,omitempty"`
}

// --- Daily combined ---

func GetLeaderboardDaily(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	day, errResp := parseRequiredDate(r)
	if errResp != "" {
		writeDailyErr(w, errResp)
		return
	}
	users, err := storage.GetDailyLeaderboardTop(day, "U", leaderboardTopN)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	bots, err := storage.GetDailyLeaderboardTop(day, "B", leaderboardTopN)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	usersJSON := mapRows(users, "U")
	botsJSON := mapRows(bots, "B")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(leaderboardDailyResponse{
		Date:  day.Format("2006-01-02"),
		Users: usersJSON,
		Bots:  botsJSON,
		Limit: leaderboardTopN,
		Empty: len(usersJSON) == 0 && len(botsJSON) == 0,
	})
}

// GetLeaderboardDailyUsers returns top 10 rows for tx_type U (Swap.origin aggregate) for the given day.
func GetLeaderboardDailyUsers(w http.ResponseWriter, r *http.Request) {
	leaderboardDailyScope(w, r, "users", "U", "daily")
}

// GetLeaderboardDailyBots returns top 10 rows for tx_type B (Swap.sender aggregate) for the given day.
func GetLeaderboardDailyBots(w http.ResponseWriter, r *http.Request) {
	leaderboardDailyScope(w, r, "bots", "B", "daily")
}

func leaderboardDailyScope(w http.ResponseWriter, r *http.Request, scope, txType, kind string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	day, errResp := parseRequiredDate(r)
	if errResp != "" {
		writeScopeErr(w, errResp)
		return
	}
	rows, err := storage.GetDailyLeaderboardTop(day, txType, leaderboardTopN)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	top := mapRowsSingleScope(rows)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(leaderboardScopeResponse{
		Scope: scope,
		Kind:  kind,
		Date:  day.Format("2006-01-02"),
		Top:   top,
		Limit: leaderboardTopN,
		Empty: len(top) == 0,
	})
}

// --- All-time combined ---

func GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rowsU, err := storage.GetAllTimeLeaderboardTopByType("U", leaderboardTopN)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rowsB, err := storage.GetAllTimeLeaderboardTopByType("B", leaderboardTopN)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	usersJSON := mapAllTimeRows(rowsU)
	botsJSON := mapAllTimeRows(rowsB)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(leaderboardAllTimeResponse{
		Users: usersJSON,
		Bots:  botsJSON,
		Limit: leaderboardTopN,
		Empty: len(usersJSON) == 0 && len(botsJSON) == 0,
	})
}

// GetLeaderboardUsers returns all-time top 10 for tx_type U (origin aggregate) only.
func GetLeaderboardUsers(w http.ResponseWriter, r *http.Request) {
	leaderboardAllTimeScope(w, r, "users", "U")
}

// GetLeaderboardBots returns all-time top 10 for tx_type B (sender aggregate) only.
func GetLeaderboardBots(w http.ResponseWriter, r *http.Request) {
	leaderboardAllTimeScope(w, r, "bots", "B")
}

func leaderboardAllTimeScope(w http.ResponseWriter, r *http.Request, scope, txType string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rows, err := storage.GetAllTimeLeaderboardTopByType(txType, leaderboardTopN)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	top := make([]leaderboardRowJSON, 0, len(rows))
	for i, row := range rows {
		top = append(top, leaderboardRowJSON{
			Rank:      i + 1,
			TxAddress: row.TxAddress,
			Volume:    formatDecimalUS(row.TotalVolume, 2),
			Date:      row.PeakDate.Format("2006-01-02"),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(leaderboardScopeResponse{
		Scope: scope,
		Kind:  "all_time",
		Top:   top,
		Limit: leaderboardTopN,
		Empty: len(top) == 0,
	})
}

func parseRequiredDate(r *http.Request) (time.Time, string) {
	ds := r.URL.Query().Get("date")
	if ds == "" {
		return time.Time{}, "missing required query: date=YYYY-MM-DD"
	}
	day, err := time.ParseInLocation("2006-01-02", ds, time.Local)
	if err != nil {
		return time.Time{}, "invalid date; use YYYY-MM-DD"
	}
	return day, ""
}

func writeDailyErr(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(leaderboardDailyResponse{Error: msg})
}

func writeScopeErr(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(leaderboardScopeResponse{Error: msg})
}

func mapRows(entries []storage.LeaderboardEntry, txType string) []leaderboardRowJSON {
	out := make([]leaderboardRowJSON, 0, len(entries))
	for i, e := range entries {
		tt := e.TxType
		if tt == "" {
			tt = txType
		}
		out = append(out, leaderboardRowJSON{
			Rank:      i + 1,
			TxAddress: e.TxAddress,
			TxType:    tt,
			Volume:    formatDecimalUS(e.Volume, 2),
		})
	}
	return out
}

func mapRowsSingleScope(entries []storage.LeaderboardEntry) []leaderboardRowJSON {
	out := make([]leaderboardRowJSON, 0, len(entries))
	for i, e := range entries {
		out = append(out, leaderboardRowJSON{
			Rank:      i + 1,
			TxAddress: e.TxAddress,
			Volume:    formatDecimalUS(e.Volume, 2),
		})
	}
	return out
}

func mapAllTimeRows(rows []storage.AllTimeLeaderboardRow) []leaderboardRowJSON {
	out := make([]leaderboardRowJSON, 0, len(rows))
	for i, row := range rows {
		out = append(out, leaderboardRowJSON{
			Rank:      i + 1,
			TxAddress: row.TxAddress,
			TxType:    row.TxType,
			Volume:    formatDecimalUS(row.TotalVolume, 2),
			Date:      row.PeakDate.Format("2006-01-02"),
		})
	}
	return out
}
