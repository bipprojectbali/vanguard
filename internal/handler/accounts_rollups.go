package handler

import (
	"context"
	"errors"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
)

// accounts_rollups.go — pembangun kartu ringkasan lintas-modul di detail
// Account (Langganan, Customer Success, Sistem, Terkait — M2-7..M2-9). Semua
// fungsi di sini BEST-EFFORT: kegagalan query/pgx.ErrNoRows menghasilkan
// nilai kosong/zero-value, TIDAK PERNAH menggagalkan seluruh halaman (mirror
// pola accountLabel/dealQuotesPreview di sales_deals_detail.go).

// relatedRecordsPreviewLimit = jumlah kontak dicicipi utk label chip "Kontak"
// (bukan daftar penuh — cukup 3 job title utk gambaran singkat).
const relatedRecordsPreviewLimit = 3

// subscriptionSummaryFor merakit kartu "Ringkasan Langganan" (langganan
// TERBARU satu desa). MRR/ARR disamarkan F4 via maskARR — kelas sensitivitas
// sama dgn VillageBudget/deal Amount, tersembunyi hanya utk Support.
func (h *Handler) subscriptionSummaryFor(ctx context.Context, base string, accountID int64, br string) panel.SubscriptionSummaryView {
	href := base + "/subscriptions"
	s, err := h.q(ctx).GetLatestSubscriptionForAccount(ctx, accountID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			h.Log.Error("accounts: latest subscription", "err", err)
		}
		return panel.SubscriptionSummaryView{Href: href}
	}
	return panel.SubscriptionSummaryView{
		PlanName:         s.PlanName,
		StatusLabel:      s.Status,
		MRR:              maskARR(formatRupiah(s.Mrr), br),
		ARR:              maskARR(formatRupiah(s.Arr), br),
		RenewalOrEndDate: dateStr(s.EndDate),
		Href:             href,
	}
}

// customerSuccessSummaryFor merakit kartu "Ringkasan Customer Success". Health
// Score TANPA masking (terbuka semua role, konvensi existing —
// healthScoreStatus dari health_score_view.go, tak diciptakan ulang). CSM name
// via names (dirakit sekali oleh pemanggil, bukan query per kartu).
func (h *Handler) customerSuccessSummaryFor(ctx context.Context, base string, a db.Account, names map[int64]string) panel.CustomerSuccessSummaryView {
	v := panel.CustomerSuccessSummaryView{
		CSMName: memberName(names, a.AssignedCsm),
		Href:    base + "/accounts/" + strconv.FormatInt(a.ID, 10) + "/customer-success",
	}
	if cs, err := h.q(ctx).GetCustomerSuccessByAccountID(ctx, a.ID); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			h.Log.Error("accounts: customer success", "err", err)
		}
	} else {
		v.HealthLabel, v.HealthBadgeClass = healthScoreStatus(cs.HealthStatus)
		v.LifecycleStage = deref(cs.LifecycleStage)
	}
	if eng, err := h.q(ctx).GetLatestEngagementForAccount(ctx, a.ID); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			h.Log.Error("accounts: latest engagement", "err", err)
		}
	} else {
		v.LastEngagementDate = fmtDateTime(eng.ScheduledAt)
	}
	return v
}

// auditViewFor merakit kartu "Sistem". Metadata operasional, bukan data
// sensitif → tanpa masking F4.
func auditViewFor(a db.Account, names map[int64]string) panel.AuditView {
	return panel.AuditView{
		CreatedByName: memberName(names, a.CreatedBy),
		CreatedAt:     fmtDateTime(a.CreatedAt),
		UpdatedByName: memberName(names, a.UpdatedBy),
		UpdatedAt:     fmtDateTime(a.UpdatedAt),
	}
}

// parentAccountFor meresolusi induk akun (baris "Induk Akun" di Identitas).
// Beda dari accountLabel (dipakai deal): gagal/terhapus/tak ada → label+href
// KOSONG (bukan "Desa #<id>" cadangan) — baris kosong lebih tepat drpd tautan
// yang merujuk id tak valid.
func (h *Handler) parentAccountFor(ctx context.Context, base string, parentID *int64) (label, href string) {
	if parentID == nil {
		return "", ""
	}
	p, err := h.q(ctx).GetAccount(ctx, *parentID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			h.Log.Error("accounts: parent account", "err", err)
		}
		return "", ""
	}
	label = p.VillageName
	if p.EntityCode != nil && *p.EntityCode != "" {
		label = *p.EntityCode + " — " + p.VillageName
	}
	return label, base + "/accounts/" + strconv.FormatInt(p.ID, 10)
}

// relatedRecordsFor merakit baris "Terkait": empat chip ringkasan modul lain.
// sub = ringkasan langganan yang SUDAH DIAMBIL oleh subscriptionSummaryFor
// (dipakai ulang di sini, bukan query kedua kalinya).
func (h *Handler) relatedRecordsFor(ctx context.Context, base string, accountID int64, sub panel.SubscriptionSummaryView) panel.RelatedRecordsView {
	accountBase := base + "/accounts/" + strconv.FormatInt(accountID, 10)
	return panel.RelatedRecordsView{
		Chips: []panel.RelatedRecordChip{
			h.contactsChip(ctx, accountBase, accountID),
			h.dealsChip(ctx, accountID),
			subscriptionsChip(base, sub),
			h.ticketsChip(ctx, accountID),
		},
	}
}
