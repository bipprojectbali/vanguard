package handler

import (
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// HealthScoreList — GET /health-scores.
// Workspace-level listing health score semua desa dalam scope role.
// Gate: crm:health read (canViewHealthScore). F3 ownership via flag boolean.
func (h *Handler) HealthScoreList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !canViewHealthScore(ctx) {
		h.renderHealthScoreForbidden(w, r)
		return
	}

	slug := slugFromRequest(r)

	// Filter ownership — pola AccountsListFilter.
	lp := healthScoreListParams(ctx)
	lp.CursorCreatedAt, lp.CursorID = pageCursor(r)
	lp.PageSize = pageSize + 1

	// Filter status dari tab (?tab=sehat|berisiko|kritis). filterStatus disimpan
	// terpisah (string) karena lp.FilterStatus di-generate sqlc sbg interface{}
	// (query dasar ListHealthScores tanpa cast ::text) — 7 query sort BARU
	// (BL-157i) tulis predikat DENGAN ::text sehingga field Params-nya string.
	tab := r.URL.Query().Get("tab")
	filterStatus := healthTabToStatus(tab)
	lp.FilterStatus = filterStatus

	// BL-114: segmen populasi (?segment=active|churned). Default "active" = desa
	// pelanggan berlangganan hidup; "churned" = eks-pelanggan (punya langganan,
	// tak ada yang hidup). Prospek tanpa langganan tak muncul di segmen mana pun.
	segment := healthSegment(r.URL.Query().Get("segment"))
	lp.Segment = segment

	// Pencarian bebas (BL-6) — menyaring pada nama desa yang tampil.
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	lp.Search = query

	kpis, err := h.q(ctx).CountHealthScoreKPIs(ctx, healthScoreKPIParams(lp))
	if err != nil {
		h.Log.Error("health-scores: count kpis", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	uid := session.UserID(ctx)

	// sort/dir (BL-157i): whitelist 7 kolom sortable (healthScoreSortableColumns).
	// Status TAK sortable — tab sudah jadi sumbu kategorik & sejak BL-24
	// health_status derivasi overall_health_score (sort terpisah nyaris duplikat
	// sort "score").
	sortCol := r.URL.Query().Get("sort")
	if !healthScoreSortableColumns[sortCol] {
		sortCol = ""
	}
	dir := r.URL.Query().Get("dir")
	if dir != "asc" && dir != "desc" {
		dir = "asc"
	}

	var items []panel.HealthScoreRowView
	var nextCursor string
	switch sortCol {
	case "village":
		cursorVal, cursorSortID, hasCursor := pageCursorText(r)
		rows, err := h.q(ctx).ListHealthScoresSortByVillage(ctx, db.ListHealthScoresSortByVillageParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     lp.ScopeAll,
			IsCsm:        lp.IsCsm,
			Uid:          &uid,
			IsSales:      lp.IsSales,
			Segment:      segment,
			FilterStatus: filterStatus,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("health-scores: list sort village", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPageText(rows, func(s db.ListHealthScoresSortByVillageRow) (string, int64) {
			return s.AccountName, s.ID
		})
		nextCursor = nc
		items = make([]panel.HealthScoreRowView, 0, len(shown))
		for _, s := range shown {
			items = append(items, healthRowToView(healthListRowFromVillageSort(s), slug, appTZ, uid))
		}
	case "score":
		cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		var cursorVal int16
		if hasCursor && !isNull {
			n, perr := strconv.ParseInt(cursorRaw, 10, 16)
			if perr != nil {
				hasCursor = false
			} else {
				cursorVal = int16(n)
			}
		}
		rows, err := h.q(ctx).ListHealthScoresSortByScore(ctx, db.ListHealthScoresSortByScoreParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     lp.ScopeAll,
			IsCsm:        lp.IsCsm,
			Uid:          &uid,
			IsSales:      lp.IsSales,
			Segment:      segment,
			FilterStatus: filterStatus,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("health-scores: list sort score", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPageTextNullable(rows, func(s db.ListHealthScoresSortByScoreRow) (string, int64, bool) {
			if s.OverallHealthScore == nil {
				return "", s.ID, true
			}
			return strconv.FormatInt(int64(*s.OverallHealthScore), 10), s.ID, false
		})
		nextCursor = nc
		items = make([]panel.HealthScoreRowView, 0, len(shown))
		for _, s := range shown {
			items = append(items, healthRowToView(healthListRowFromScoreSort(s), slug, appTZ, uid))
		}
	case "adoption":
		cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		var cursorVal int16
		if hasCursor && !isNull {
			n, perr := strconv.ParseInt(cursorRaw, 10, 16)
			if perr != nil {
				hasCursor = false
			} else {
				cursorVal = int16(n)
			}
		}
		rows, err := h.q(ctx).ListHealthScoresSortByAdoption(ctx, db.ListHealthScoresSortByAdoptionParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     lp.ScopeAll,
			IsCsm:        lp.IsCsm,
			Uid:          &uid,
			IsSales:      lp.IsSales,
			Segment:      segment,
			FilterStatus: filterStatus,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("health-scores: list sort adoption", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPageTextNullable(rows, func(s db.ListHealthScoresSortByAdoptionRow) (string, int64, bool) {
			if s.AdoptionScore == nil {
				return "", s.ID, true
			}
			return strconv.FormatInt(int64(*s.AdoptionScore), 10), s.ID, false
		})
		nextCursor = nc
		items = make([]panel.HealthScoreRowView, 0, len(shown))
		for _, s := range shown {
			items = append(items, healthRowToView(healthListRowFromAdoptionSort(s), slug, appTZ, uid))
		}
	case "engagement":
		cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		var cursorVal int16
		if hasCursor && !isNull {
			n, perr := strconv.ParseInt(cursorRaw, 10, 16)
			if perr != nil {
				hasCursor = false
			} else {
				cursorVal = int16(n)
			}
		}
		rows, err := h.q(ctx).ListHealthScoresSortByEngagement(ctx, db.ListHealthScoresSortByEngagementParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     lp.ScopeAll,
			IsCsm:        lp.IsCsm,
			Uid:          &uid,
			IsSales:      lp.IsSales,
			Segment:      segment,
			FilterStatus: filterStatus,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("health-scores: list sort engagement", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPageTextNullable(rows, func(s db.ListHealthScoresSortByEngagementRow) (string, int64, bool) {
			if s.EngagementScore == nil {
				return "", s.ID, true
			}
			return strconv.FormatInt(int64(*s.EngagementScore), 10), s.ID, false
		})
		nextCursor = nc
		items = make([]panel.HealthScoreRowView, 0, len(shown))
		for _, s := range shown {
			items = append(items, healthRowToView(healthListRowFromEngagementSort(s), slug, appTZ, uid))
		}
	case "support":
		cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		var cursorVal int16
		if hasCursor && !isNull {
			n, perr := strconv.ParseInt(cursorRaw, 10, 16)
			if perr != nil {
				hasCursor = false
			} else {
				cursorVal = int16(n)
			}
		}
		rows, err := h.q(ctx).ListHealthScoresSortBySupport(ctx, db.ListHealthScoresSortBySupportParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     lp.ScopeAll,
			IsCsm:        lp.IsCsm,
			Uid:          &uid,
			IsSales:      lp.IsSales,
			Segment:      segment,
			FilterStatus: filterStatus,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("health-scores: list sort support", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPageTextNullable(rows, func(s db.ListHealthScoresSortBySupportRow) (string, int64, bool) {
			if s.SupportScore == nil {
				return "", s.ID, true
			}
			return strconv.FormatInt(int64(*s.SupportScore), 10), s.ID, false
		})
		nextCursor = nc
		items = make([]panel.HealthScoreRowView, 0, len(shown))
		for _, s := range shown {
			items = append(items, healthRowToView(healthListRowFromSupportSort(s), slug, appTZ, uid))
		}
	case "trend":
		cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		rows, err := h.q(ctx).ListHealthScoresSortByTrend(ctx, db.ListHealthScoresSortByTrendParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     lp.ScopeAll,
			IsCsm:        lp.IsCsm,
			Uid:          &uid,
			IsSales:      lp.IsSales,
			Segment:      segment,
			FilterStatus: filterStatus,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("health-scores: list sort trend", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPageTextNullable(rows, func(s db.ListHealthScoresSortByTrendRow) (string, int64, bool) {
			if s.ScoreTrend == nil {
				return "", s.ID, true
			}
			return *s.ScoreTrend, s.ID, false
		})
		nextCursor = nc
		items = make([]panel.HealthScoreRowView, 0, len(shown))
		for _, s := range shown {
			items = append(items, healthRowToView(healthListRowFromTrendSort(s), slug, appTZ, uid))
		}
	case "renewal":
		cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		// cursor Jatuh Tempo kanonik = ISO tanggal apa adanya (optDate,
		// sales_format.go), mirror kolom Close Date Deals/Renewal Date Renewals.
		cursorVal, code := optDate(cursorRaw)
		if code != "" {
			hasCursor = false
		}
		rows, err := h.q(ctx).ListHealthScoresSortByRenewal(ctx, db.ListHealthScoresSortByRenewalParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     lp.ScopeAll,
			IsCsm:        lp.IsCsm,
			Uid:          &uid,
			IsSales:      lp.IsSales,
			Segment:      segment,
			FilterStatus: filterStatus,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("health-scores: list sort renewal", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPageTextNullable(rows, func(s db.ListHealthScoresSortByRenewalRow) (string, int64, bool) {
			if !s.RenewalEndDate.Valid {
				return "", s.ID, true
			}
			return dateStr(s.RenewalEndDate), s.ID, false
		})
		nextCursor = nc
		items = make([]panel.HealthScoreRowView, 0, len(shown))
		for _, s := range shown {
			items = append(items, healthRowToView(healthListRowFromRenewalSort(s), slug, appTZ, uid))
		}
	default:
		rows, err := h.q(ctx).ListHealthScores(ctx, lp)
		if err != nil {
			h.Log.Error("health-scores: list", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPage(rows, func(r db.ListHealthScoresRow) (pgtype.Timestamptz, int64) {
			return r.CreatedAt, r.ID
		})
		nextCursor = nc
		items = make([]panel.HealthScoreRowView, 0, len(shown))
		for _, row := range shown {
			items = append(items, healthRowToView(healthListRowFromDefault(row), slug, appTZ, uid))
		}
	}
	if items == nil {
		items = []panel.HealthScoreRowView{}
	}

	kpiView, panels := h.healthKPIsToView(kpis)

	h.renderWorkspaceShell(w, r, "Customer Health Score", "/health-scores", panel.HealthScoreList(panel.HealthScoreListView{
		Base:          wsPath(slug, ""),
		ActiveTab:     tab,
		Segment:       segment,
		Query:         query,
		NextCursor:    nextCursor,
		After:         r.URL.Query().Get("after"),
		Trail:         pageTrail(r),
		Sort:          sortCol,
		Dir:           dir,
		KPIs:          kpiView,
		Panels:        panels,
		TableSubtitle: healthTableSubtitle(kpis.Total, sortCol),
		Rows:          items,
	}))
}

// healthScoreSortableColumns = whitelist kolom yang boleh diminta lewat ?sort=
// (BL-157i: 7 dari 9 kolom tabel Health Score). Status & kolom aksi SENGAJA
// absen (lihat komentar di HealthScoreList). ?sort= di luar daftar ini
// diperlakukan seolah absen (jatuh ke default created_at DESC), TAK error.
var healthScoreSortableColumns = map[string]bool{
	"village":    true,
	"score":      true,
	"adoption":   true,
	"engagement": true,
	"support":    true,
	"trend":      true,
	"renewal":    true,
}

// healthSegment menormalkan ?segment= ke nilai SQL sah. Apa pun selain "churned"
// → "active" (default aman: nilai janggal jatuh ke populasi pelanggan aktif, tak
// pernah membuka data churned tanpa diminta eksplisit).
func healthSegment(seg string) string {
	if seg == "churned" {
		return "churned"
	}
	return "active"
}

// healthTabToStatus memetakan nilai query-param ?tab= ke health_status DB.
// Tab kosong / "semua" → "" (tidak difilter).
func healthTabToStatus(tab string) string {
	switch tab {
	case "sehat":
		return "Healthy"
	case "berisiko":
		return "At-Risk"
	case "kritis":
		return "Critical"
	default:
		return ""
	}
}
