package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// cs_renewals.go — halaman Renewal Management CS (Modul 6 slice 6.6).
//
// Keputusan "Renewal Dua-Rumah": data renewal (end_date, renewal_status, dsb.)
// tetap milik tabel subscriptions; halaman ini MENAMPILKAN data itu dan
// memberikan form untuk mengisi AKSI CS (renewal_stage, renewal_risk,
// renewal_action_plan, renewal_next_action_date, renewal_owner).
//
// Gate F2: canViewCSRenewals (crm:renewal_mgmt read). F3 ownership lewat
// CSRenewalsListFilterFor — scope akun (assigned_csm/backup_csm/account_owner).

// CSRenewalsList — GET /w/{slug}/renewal-management.
// Daftar langganan ber-renewal-date dalam scope CSM, dengan tab filter stage.
func (h *Handler) CSRenewalsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewCSRenewals(ctx) {
		h.renderCSRenewalsForbidden(w, r)
		return
	}

	canWrite := canWriteCSRenewals(ctx)
	slug := slugFromRequest(r)
	tab := r.URL.Query().Get("tab")

	filter := db.CSRenewalsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)

	cursorAt, cursorID := pageCursor(r)
	rows, err := h.q(ctx).ListCSRenewals(ctx, db.ListCSRenewalsParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        filter.ScopeAll,
		IsOwn:           filter.IsOwn,
		Uid:             &uid,
		FilterStage:     tab,
		PageSize:        pageSize + 1,
	})
	if err != nil {
		h.Log.Error("cs-renewals: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	shown, nextCursor := splitPage(rows, func(r db.ListCSRenewalsRow) (pgtype.Timestamptz, int64) {
		return r.CreatedAt, r.ID
	})

	items := make([]panel.CSRenewalRow, 0, len(shown))
	for _, row := range shown {
		items = append(items, csRenewalRowView(row, slug))
	}

	h.renderWorkspaceShell(w, r, "Renewal Management", "/renewal-management",
		panel.CSRenewalsList(panel.CSRenewalsListView{
			Base:       wsPath(slug, ""),
			Tab:        tab,
			Tabs:       csRenewalTabs,
			Msg:        csRenewalsMsg(r.URL.Query().Get("ok")),
			Err:        csRenewalsErrMsg(r.URL.Query().Get("err")),
			Items:      items,
			CanWrite:   canWrite,
			NextCursor: nextCursor,
			After:      r.URL.Query().Get("after"),
			Trail:      pageTrail(r),
		}))
}

// CSRenewalEdit — GET /w/{slug}/renewal-management/{id}/edit.
// Form isi aksi CS untuk satu langganan. Gate F2 write + F3 ownership akun.
func (h *Handler) CSRenewalEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canWriteCSRenewalsPerm(ctx) {
		h.renderCSRenewalsForbidden(w, r)
		return
	}

	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}

	sub, acct, ok := h.loadCSRenewal(w, r, id)
	if !ok {
		return
	}

	// Ambil nama plan untuk judul form.
	plan, err := h.q(ctx).GetPlan(ctx, sub.PlanID)
	if err != nil {
		h.Log.Error("cs-renewals: load plan for form", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	slug := slugFromRequest(r)
	memberOpts, err := h.csRenewalMemberOptions(ctx)
	if err != nil {
		h.Log.Error("cs-renewals: members for form", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.renderWorkspaceShell(w, r, "Edit Renewal Action", "/renewal-management",
		panel.CSRenewalForm(panel.CSRenewalFormView{
			Base:              wsPath(slug, ""),
			Action:            wsPath(slug, "/renewal-management/"+strconv.FormatInt(id, 10)),
			VillageName:       acct.VillageName,
			PlanName:          plan.PlanName,
			RenewalDate:       dateStr(sub.EndDate),
			RenewalStatus:     deref(sub.RenewalStatus),
			CurrentStage:      deref(sub.RenewalStage),
			CurrentRisk:       deref(sub.RenewalRisk),
			CurrentActionPlan: deref(sub.RenewalActionPlan),
			CurrentNextAction: dateStr(sub.RenewalNextActionDate),
			CurrentOwnerID:    sub.RenewalOwner,
			Members:           memberOpts,
			Stages:            csRenewalStageValues,
			Risks:             csRenewalRiskValues,
			Err:               csRenewalsErrMsg(r.URL.Query().Get("err")),
		}))
}
