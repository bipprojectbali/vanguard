package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
)

// cs_renewals_data.go — pemuatan & pemetaan data CS Renewals: loadCSRenewal
// (baca satu baris + guard 404), opsi anggota untuk dropdown, dan pemetaan
// baris DB → view row. Dipisah dari handler HTTP di cs_renewals.go agar tiap
// file di bawah ambang tipe Route/Handler (150). Satu paket handler.

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

// csRenewalMemberOptions merakit slice option anggota untuk dropdown owner
// renewal, DISARING ke himpunan CS (BL-32). "Owner CS" = peran ber-kapabilitas
// `crm:renewal_mgmt write` (csm/manager/admin + peran custom ber-write), dihitung
// via authz.RoleCanBusiness agar tahan nama peran kustom per-tenant — bukan cocok
// nama 'csm' harfiah. Anggota tanpa business_role (belum diberi peran CRM) & yang
// bukan CS dibuang.
//
// ensureID = ID owner tersimpan/preselect yang WAJIB tetap muncul walau di luar
// himpunan CS (warisan penugasan lama ke non-CS). Tanpa ini, membuka form lalu
// menyimpan akan menghapus owner diam-diam (data-loss). ensureID 0 = tak ada yang
// dipaksa sertakan.
func (h *Handler) csRenewalMemberOptions(ctx context.Context, ensureID int64) ([]panel.CSRenewalMemberOption, error) {
	tenantID := session.TenantID(ctx)
	memberRows, err := h.q(ctx).ListMembersByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	opts := make([]panel.CSRenewalMemberOption, 0, len(memberRows))
	for _, m := range memberRows {
		br := ""
		if m.BusinessRole != nil {
			br = *m.BusinessRole
		}
		isCS := br != "" && authz.RoleCanBusiness(tenantID, br, "crm:renewal_mgmt", "write")
		if !isCS && m.UserID != ensureID {
			continue
		}
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
		PlanName:       subPlanDisplay(r.PlanName, r.ItemCount),
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
