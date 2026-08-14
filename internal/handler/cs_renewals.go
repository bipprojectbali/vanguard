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

// loadCSRenewal memuat satu langganan lalu menegakkan F3 ownership via akun.
// Di luar cakupan → 404 (keberadaan tak diungkap). Mengembalikan (sub, acct, true)
// hanya bila baris ada DAN dalam cakupan aktor. acct dikembalikan agar caller
// dapat pakai VillageName tanpa query tambahan.
func (h *Handler) loadCSRenewal(w http.ResponseWriter, r *http.Request, id int64) (db.Subscription, db.Account, bool) {
	ctx := r.Context()
	sub, err := h.q(ctx).GetSubscription(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return db.Subscription{}, db.Account{}, false
		}
		h.Log.Error("cs-renewals: load subscription", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.Subscription{}, db.Account{}, false
	}
	// F3: muat akun untuk periksa kepemilikan (assigned_csm/backup_csm/account_owner).
	acct, err := h.q(ctx).GetAccount(ctx, sub.AccountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return db.Subscription{}, db.Account{}, false
		}
		h.Log.Error("cs-renewals: load account for ownership", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.Subscription{}, db.Account{}, false
	}
	filter := db.CSRenewalsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	if !filter.Allows(uid, acct.AccountOwner, acct.AssignedCsm, acct.BackupCsm) {
		http.NotFound(w, r)
		return db.Subscription{}, db.Account{}, false
	}
	return sub, acct, true
}

// csRenewalMemberOptions merakit slice option anggota untuk dropdown owner renewal.
// Pola sama dengan EngagementNew — memakai ListMembersByTenant.
func (h *Handler) csRenewalMemberOptions(ctx context.Context) ([]panel.CSRenewalMemberOption, error) {
	memberRows, err := h.q(ctx).ListMembersByTenant(ctx, session.TenantID(ctx))
	if err != nil {
		return nil, err
	}
	opts := make([]panel.CSRenewalMemberOption, 0, len(memberRows))
	for _, m := range memberRows {
		name := m.Email
		if m.Name != nil && *m.Name != "" {
			name = *m.Name
		}
		opts = append(opts, panel.CSRenewalMemberOption{ID: m.UserID, Name: name})
	}
	return opts, nil
}

// csRenewalRowView mengonversi satu baris query → panel.CSRenewalRow.
func csRenewalRowView(r db.ListCSRenewalsRow, slug string) panel.CSRenewalRow {
	return panel.CSRenewalRow{
		ID:             r.ID,
		VillageName:    r.VillageName,
		PlanName:       r.PlanName,
		RenewalDate:    dateStr(r.EndDate),
		RenewalStatus:  deref(r.RenewalStatus),
		StageLabel:     csRenewalStageLabel(r.RenewalStage),
		StageBadge:     csRenewalStageBadge(r.RenewalStage),
		RiskLabel:      csRenewalRiskLabel(r.RenewalRisk),
		RiskBadge:      csRenewalRiskBadge(r.RenewalRisk),
		OwnerName:      deref(r.RenewalOwnerName),
		NextActionDate: dateStr(r.RenewalNextActionDate),
		HrefEdit:       wsPath(slug, "/renewal-management/"+strconv.FormatInt(r.ID, 10)+"/edit"),
	}
}

// csRenewalTabs — tab filter stage (sumber tunggal handler + view).
var csRenewalTabs = []panel.CSRenewalTab{
	{Key: "", Label: "Semua"},
	{Key: "Not Started", Label: "Belum Dimulai"},
	{Key: "Outreach", Label: "Outreach"},
	{Key: "Negotiation", Label: "Negosiasi"},
	{Key: "Won", Label: "Won"},
	{Key: "Lost", Label: "Lost"},
}

// csRenewalStageValues — nilai sah stage, dipakai validasi + dropdown.
var csRenewalStageValues = []string{
	"Not Started", "Outreach", "Negotiation", "Won", "Lost",
}

// csRenewalRiskValues — nilai sah risk, dipakai validasi + dropdown.
var csRenewalRiskValues = []string{
	"Low", "Medium", "High",
}

// isValidCSRenewalStage — validasi nilai stage form.
func isValidCSRenewalStage(s string) bool {
	for _, v := range csRenewalStageValues {
		if v == s {
			return true
		}
	}
	return false
}

// isValidCSRenewalRisk — validasi nilai risk form.
func isValidCSRenewalRisk(s string) bool {
	for _, v := range csRenewalRiskValues {
		if v == s {
			return true
		}
	}
	return false
}

// csRenewalStageLabel → label Indonesia dari nilai DB (nil = "—").
func csRenewalStageLabel(s *string) string {
	if s == nil {
		return "—"
	}
	switch *s {
	case "Not Started":
		return "Belum Dimulai"
	case "Outreach":
		return "Outreach"
	case "Negotiation":
		return "Negosiasi"
	case "Won":
		return "Won"
	case "Lost":
		return "Lost"
	default:
		return *s
	}
}

// csRenewalStageBadge → kelas badge daisyUI per stage.
func csRenewalStageBadge(s *string) string {
	if s == nil {
		return "badge-ghost"
	}
	switch *s {
	case "Not Started":
		return "badge-neutral"
	case "Outreach":
		return "badge-info"
	case "Negotiation":
		return "badge-warning"
	case "Won":
		return "badge-success"
	case "Lost":
		return "badge-error"
	default:
		return "badge-ghost"
	}
}

// csRenewalRiskLabel → label Indonesia dari nilai DB (nil = "—").
func csRenewalRiskLabel(s *string) string {
	if s == nil {
		return "—"
	}
	switch *s {
	case "Low":
		return "Rendah"
	case "Medium":
		return "Sedang"
	case "High":
		return "Tinggi"
	default:
		return *s
	}
}

// csRenewalRiskBadge → kelas badge daisyUI per risk level.
func csRenewalRiskBadge(s *string) string {
	if s == nil {
		return "badge-ghost"
	}
	switch *s {
	case "Low":
		return "badge-success"
	case "Medium":
		return "badge-warning"
	case "High":
		return "badge-error"
	default:
		return "badge-ghost"
	}
}
