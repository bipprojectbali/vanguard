package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// engagements.go — HALAMAN baca Engagements / Check-ins (Modul 6 Customer
// Success, slice 6.5). Aksi tulis (create, status update) di engagements_save.go.
//
// Akses lewat dua sumbu orthogonal:
//   - F2 (Casbin bisnis): canViewEngagements — gerbang modul (Admin/Manager/CSM).
//   - F3 (ownership): EngagementsListFilterFor(dataScope) — baris mana yang tampil.
//     Admin/Manager (ScopeAll) lihat semua; CSM (ScopeOwn) lihat engagement desa
//     binarannya.
//   - RLS: h.q ber-tenant → mengisolasi workspace di bawah semua ini.

// engagementsDropdownLimit = batas opsi dropdown per-tipe di form engagement baru.
// Guardrail: tak ada full-scan tanpa batas.
const engagementsDropdownLimit = 200

// EngagementsList — GET /w/{workspace}/engagements. Daftar engagement + KPI
// header + filter tab. canViewEngagements=false → 403.
func (h *Handler) EngagementsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewEngagements(ctx) {
		h.renderEngagementsForbidden(w, r)
		return
	}

	dataScope := session.BusinessDataScope(ctx)
	canWrite := canWriteEngagements(ctx)
	uid := session.UserID(ctx)
	filter := db.EngagementsListFilterFor(dataScope)

	// Tab → filter parameter.
	tab := r.URL.Query().Get("tab")
	var filterStatus, filterType string
	switch tab {
	case "planned", "done", "skipped", "rescheduled":
		filterStatus = tab
	case "touch_point", "qbr", "onboarding_call", "escalation", "check_in":
		filterType = tab
	default:
		tab = "" // normalisasi nilai liar → "" (semua)
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	cursorAt, cursorID := pageCursor(r)

	// KPI header: cakupan scope sama persis ListEngagements.
	kpis, err := h.q(ctx).CountEngagementKPIs(ctx, db.CountEngagementKPIsParams{
		ScopeAll: filter.ScopeAll,
		IsOwn:    filter.IsOwn,
		Uid:      &uid,
	})
	if err != nil {
		h.Log.Error("engagements: count kpis", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	rows, err := h.q(ctx).ListEngagements(ctx, db.ListEngagementsParams{
		CursorScheduledAt: cursorAt,
		CursorID:          cursorID,
		ScopeAll:          filter.ScopeAll,
		IsOwn:             filter.IsOwn,
		Uid:               &uid,
		FilterStatus:      filterStatus,
		FilterType:        filterType,
		Search:            query,
		PageSize:          pageSize + 1,
	})
	if err != nil {
		h.Log.Error("engagements: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	shown, nextCursor := splitPage(rows, func(r db.ListEngagementsRow) (pgtype.Timestamptz, int64) {
		return r.ScheduledAt, r.ID
	})

	slug := slugFromRequest(r)
	items := make([]panel.EngagementRow, 0, len(shown))
	for _, row := range shown {
		items = append(items, engagementRowView(row, slug, appTZ))
	}

	base := wsPath(slug, "")
	h.renderWorkspaceShell(w, r, "Engagements", "/engagements", panel.EngagementsList(panel.EngagementsListView{
		Base:       base,
		KPIs:       engagementKPIView(kpis),
		Items:      items,
		Tab:        tab,
		Query:      query,
		CanWrite:   canWrite,
		NextCursor: nextCursor,
		Err:        engagementsErrMsg(r.URL.Query().Get("err")),
		Msg:        engagementsMsg(r.URL.Query().Get("ok")),
	}))
}
