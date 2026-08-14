package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
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

	cursorAt, cursorID := pageCursor(r)

	rows, err := h.q(ctx).ListSuccessPlans(ctx, db.ListSuccessPlansParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        filter.ScopeAll,
		IsOwn:           filter.IsOwn,
		Uid:             &uid,
		FilterStatus:    filterStatus,
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
		Tabs:       successPlansTabs,
		Items:      items,
		CanWrite:   canWrite,
		NextCursor: nextCursor,
		Err:        successPlansErrMsg(r.URL.Query().Get("err")),
		Msg:        successPlansMsg(r.URL.Query().Get("ok")),
	}))
}

// SuccessPlanNew — GET /w/{workspace}/success-plans/new. Form kosong untuk
// plan baru. Hanya canWriteSuccessPlans → Admin, Manager, CSM.
func (h *Handler) SuccessPlanNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canWriteSuccessPlans(ctx) {
		h.renderSuccessPlansForbidden(w, r)
		return
	}

	slug := slugFromRequest(r)
	base := wsPath(slug, "")
	dataScope := session.BusinessDataScope(ctx)
	uid := session.UserID(ctx)
	filter := db.SuccessPlansListFilterFor(dataScope)

	// Accounts dropdown: scope user sendiri (CSM hanya melihat desanya).
	at, cid := firstPageCursor()
	accts, err := h.q(ctx).ListAccounts(ctx, db.ListAccountsParams{
		CursorCreatedAt: at, CursorID: cid,
		ScopeAll: filter.ScopeAll,
		IsSales:  filter.IsOwn,
		IsCsm:    filter.IsOwn,
		Uid:      &uid,
		PageSize: successPlansDropdownLimit,
	})
	if err != nil {
		h.Log.Error("success_plans: list accounts for form", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	accountOpts := make([]panel.SuccessPlanAccountOption, 0, len(accts))
	for _, a := range accts {
		accountOpts = append(accountOpts, panel.SuccessPlanAccountOption{ID: a.ID, Name: a.VillageName})
	}

	// Members dropdown: owner_csm (opsional).
	memberOpts, err := h.successPlanMemberOptions(ctx)
	if err != nil {
		h.Log.Error("success_plans: list members for form", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.renderWorkspaceShell(w, r, "Buat Success Plan", "/success-plans", panel.SuccessPlanForm(panel.SuccessPlanFormView{
		Base:     base,
		Action:   base + "/success-plans",
		Err:      successPlansErrMsg(r.URL.Query().Get("err")),
		Accounts: accountOpts,
		Members:  memberOpts,
		Statuses: successPlanStatusValues,
	}))
}

// SuccessPlanEdit — GET /w/{workspace}/success-plans/{id}/edit. Form isi untuk
// edit plan. F3 ditegakkan via loadSuccessPlan.
func (h *Handler) SuccessPlanEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canWriteSuccessPlans(ctx) {
		h.renderSuccessPlansForbidden(w, r)
		return
	}

	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}

	sp, acct, ok := h.loadSuccessPlan(w, r, id)
	if !ok {
		return
	}

	slug := slugFromRequest(r)
	base := wsPath(slug, "")

	memberOpts, err := h.successPlanMemberOptions(ctx)
	if err != nil {
		h.Log.Error("success_plans: list members for edit form", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.renderWorkspaceShell(w, r, "Edit Success Plan", "/success-plans", panel.SuccessPlanForm(panel.SuccessPlanFormView{
		Base:     base,
		Action:   base + "/success-plans/" + strconv.FormatInt(id, 10),
		Err:      successPlansErrMsg(r.URL.Query().Get("err")),
		Members:  memberOpts,
		Statuses: successPlanStatusValues,

		// Data akun (read-only di form edit — tampilkan saja, tanpa dropdown desa).
		AccountName: acct.VillageName,

		// Nilai saat ini.
		CurrentPlanName:      sp.PlanName,
		CurrentObjective:     deref(sp.Objective),
		CurrentSuccessMetric: deref(sp.SuccessMetric),
		CurrentTargetDate:    dateStr(sp.TargetDate),
		CurrentStatus:        sp.PlanStatus,
		CurrentProgress:      int(sp.Progress),
		CurrentOwnerCsmID:    sp.OwnerCsm,
	}))
}

// loadSuccessPlan memuat satu success plan + akun terkait dan menegakkan F3.
// Mengembalikan (plan, akun, true) jika berhasil; menulis respons dan
// mengembalikan false jika gagal.
func (h *Handler) loadSuccessPlan(w http.ResponseWriter, r *http.Request, id int64) (db.GetSuccessPlanRow, db.Account, bool) {
	ctx := r.Context()

	sp, err := h.q(ctx).GetSuccessPlan(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return db.GetSuccessPlanRow{}, db.Account{}, false
		}
		h.Log.Error("success_plans: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.GetSuccessPlanRow{}, db.Account{}, false
	}

	dataScope := session.BusinessDataScope(ctx)
	filter := db.SuccessPlansListFilterFor(dataScope)
	uid := session.UserID(ctx)

	// Muat akun: untuk VillageName (dipakai form edit) dan F3 ownership check.
	acct, err := h.q(ctx).GetAccount(ctx, sp.AccountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return db.GetSuccessPlanRow{}, db.Account{}, false
		}
		h.Log.Error("success_plans: get account", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.GetSuccessPlanRow{}, db.Account{}, false
	}

	if !filter.ScopeAll {
		if !filter.IsOwn {
			http.NotFound(w, r)
			return db.GetSuccessPlanRow{}, db.Account{}, false
		}
		if !filter.Allows(uid, acct.AccountOwner, acct.AssignedCsm, acct.BackupCsm, sp.OwnerCsm) {
			http.NotFound(w, r)
			return db.GetSuccessPlanRow{}, db.Account{}, false
		}
	}
	return sp, acct, true
}

// successPlanMemberOptions merakit slice option anggota untuk dropdown owner_csm.
// Pola sama dengan csRenewalMemberOptions.
func (h *Handler) successPlanMemberOptions(ctx context.Context) ([]panel.SuccessPlanMemberOption, error) {
	memberRows, err := h.q(ctx).ListMembersByTenant(ctx, session.TenantID(ctx))
	if err != nil {
		return nil, err
	}
	opts := make([]panel.SuccessPlanMemberOption, 0, len(memberRows))
	for _, m := range memberRows {
		name := m.Email
		if m.Name != nil && *m.Name != "" {
			name = *m.Name
		}
		opts = append(opts, panel.SuccessPlanMemberOption{ID: m.UserID, Name: name})
	}
	return opts, nil
}

// successPlanRowView memetakan satu baris ListSuccessPlansRow → SuccessPlanRow view.
func successPlanRowView(r db.ListSuccessPlansRow, slug string) panel.SuccessPlanRow {
	return panel.SuccessPlanRow{
		ID:          r.ID,
		AccountName: r.AccountName,
		PlanName:    r.PlanName,
		Objective:   deref(r.Objective),
		StatusLabel: r.PlanStatus,
		StatusBadge: successPlanStatusBadge(r.PlanStatus),
		Progress:    int(r.Progress),
		OwnerName:   deref(r.OwnerName),
		TargetDate:  dateStr(r.TargetDate),
		HrefEdit:    wsPath(slug, "/success-plans/"+strconv.FormatInt(r.ID, 10)+"/edit"),
	}
}

// successPlanStatusBadge memetakan status → kelas badge daisyUI.
func successPlanStatusBadge(s string) string {
	switch s {
	case "Draft":
		return "badge-ghost"
	case "Active":
		return "badge-info"
	case "Achieved":
		return "badge-success"
	case "At-Risk":
		return "badge-warning"
	case "Cancelled":
		return "badge-neutral"
	default:
		return "badge-ghost"
	}
}
