package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// success_plans.go — HALAMAN baca Success Plans (Modul 6 Customer Success,
// slice 6.3). Aksi tulis (create, update) di success_plans_save.go.
//
// Akses lewat dua sumbu orthogonal:
//   - F2 (Casbin bisnis): canViewSuccessPlans — gerbang modul (Admin/Manager/CSM).
//   - F3 (ownership): SuccessPlansListFilterFor(dataScope) — baris mana yang tampil.
//     Admin/Manager (ScopeAll) lihat semua; CSM (ScopeOwn) lihat plan desa
//     binaannya atau plan yang ia miliki.
//   - RLS: h.q ber-tenant → mengisolasi workspace di bawah semua ini.

// successPlansDropdownLimit = batas opsi dropdown per-tipe di form plan baru.
const successPlansDropdownLimit = 200

// successPlanStatusValues = urutan status sesuai spec 6.3 (lifecycle plan).
var successPlanStatusValues = []string{"Draft", "Active", "Achieved", "At-Risk", "Cancelled"}

// successPlansTabs = daftar tab filter halaman daftar. Key "" = semua.
var successPlansTabs = []panel.SuccessPlanTab{
	{Key: "", Label: "Semua"},
	{Key: "Draft", Label: "Draft"},
	{Key: "Active", Label: "Active"},
	{Key: "Achieved", Label: "Achieved"},
	{Key: "At-Risk", Label: "At-Risk"},
	{Key: "Cancelled", Label: "Cancelled"},
}

// SuccessPlansList — GET /w/{workspace}/success-plans. Daftar success plan +
// filter tab per status. canViewSuccessPlans=false → 403.
func (h *Handler) SuccessPlansList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewSuccessPlans(ctx) {
		h.renderSuccessPlansForbidden(w, r)
		return
	}

	dataScope := session.BusinessDataScope(ctx)
	canWrite := canWriteSuccessPlans(ctx)
	uid := session.UserID(ctx)
	filter := db.SuccessPlansListFilterFor(dataScope)

	// Tab → filter parameter.
	tab := r.URL.Query().Get("tab")
	var filterStatus string
	switch tab {
	case "Draft", "Active", "Achieved", "At-Risk", "Cancelled":
		filterStatus = tab
	default:
		tab = "" // normalisasi nilai liar → "" (semua)
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	cursorAt, cursorID := pageCursor(r)

	rows, err := h.q(ctx).ListSuccessPlans(ctx, db.ListSuccessPlansParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        filter.ScopeAll,
		IsOwn:           filter.IsOwn,
		Uid:             &uid,
		FilterStatus:    filterStatus,
		Search:          query,
		PageSize:        pageSize + 1,
	})
	if err != nil {
		h.Log.Error("success_plans: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	shown, nextCursor := splitPage(rows, func(r db.ListSuccessPlansRow) (pgtype.Timestamptz, int64) {
		return r.CreatedAt, r.ID
	})

	slug := slugFromRequest(r)
	items := make([]panel.SuccessPlanRow, 0, len(shown))
	for _, row := range shown {
		items = append(items, successPlanRowView(row, slug))
	}

	base := wsPath(slug, "")
	h.renderWorkspaceShell(w, r, "Success Plans", "/success-plans", panel.SuccessPlansList(panel.SuccessPlansListView{
		Base:       base,
		Tab:        tab,
		Query:      query,
		Tabs:       successPlansTabs,
		Items:      items,
		CanWrite:   canWrite,
		NextCursor: nextCursor,
		After:      r.URL.Query().Get("after"),
		Trail:      pageTrail(r),
		Err:        successPlansErrMsg(r.URL.Query().Get("err")),
		Msg:        successPlansMsg(r.URL.Query().Get("ok")),
	}))
}
