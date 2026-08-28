package handler

import (
	"context"
	"errors"
	"strconv"
	"strings"

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

// contactsChip: linked ke daftar kontak akun ini (route nested, sudah ada
// tombol quick-link "Kontak »" yang sama).
func (h *Handler) contactsChip(ctx context.Context, accountBase string, accountID int64) panel.RelatedRecordChip {
	chip := panel.RelatedRecordChip{Label: "Kontak", Href: accountBase + "/contacts"}
	count, err := h.q(ctx).CountContactsByAccount(ctx, accountID)
	if err != nil {
		h.Log.Error("accounts: count contacts", "err", err)
		return chip
	}
	if count == 0 {
		chip.ValueText = "Belum ada kontak"
		return chip
	}
	at, id := firstPageCursor()
	rows, err := h.q(ctx).ListContactsByAccount(ctx, db.ListContactsByAccountParams{
		AccountID:       accountID,
		CursorCreatedAt: at,
		CursorID:        id,
		PageSize:        relatedRecordsPreviewLimit,
	})
	if err != nil {
		h.Log.Error("accounts: list contacts preview", "err", err)
	}
	titles := make([]string, 0, len(rows))
	for _, c := range rows {
		if t := deref(c.JobTitle); t != "" {
			titles = append(titles, t)
		}
	}
	label := strconv.FormatInt(count, 10) + " kontak"
	if len(titles) > 0 {
		label += " (" + strings.Join(titles, ", ") + ")"
	}
	chip.ValueText = label
	return chip
}

// dealsChip: TANPA href — belum ada rute daftar deal ber-filter desa (keputusan
// sadar, lihat rasional di plan M2-9).
func (h *Handler) dealsChip(ctx context.Context, accountID int64) panel.RelatedRecordChip {
	chip := panel.RelatedRecordChip{Label: "Deal"}
	count, err := h.q(ctx).CountDealsByAccount(ctx, accountID)
	if err != nil {
		h.Log.Error("accounts: count deals", "err", err)
		return chip
	}
	if count == 0 {
		chip.ValueText = "Belum ada deal"
		return chip
	}
	d, err := h.q(ctx).GetLatestDealForAccount(ctx, accountID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			h.Log.Error("accounts: latest deal", "err", err)
		}
		chip.ValueText = strconv.FormatInt(count, 10) + " deal"
		return chip
	}
	chip.ValueText = strconv.FormatInt(count, 10) + " deal (" + d.DealName + " — " + d.Stage + ")"
	return chip
}

// subscriptionsChip menautkan ke daftar langganan workspace (bukan query
// baru — hasil GetLatestSubscriptionForAccount sudah tersedia dari kartu
// ringkasan langganan).
func subscriptionsChip(base string, sub panel.SubscriptionSummaryView) panel.RelatedRecordChip {
	value := "Belum ada langganan"
	if sub.PlanName != "" {
		value = "Lihat semua langganan"
	}
	return panel.RelatedRecordChip{Label: "Langganan", ValueText: value, Href: base + "/subscriptions"}
}

// ticketsChip: TANPA href — belum ada rute daftar tiket ber-filter desa (sama
// rasional dgn dealsChip).
func (h *Handler) ticketsChip(ctx context.Context, accountID int64) panel.RelatedRecordChip {
	chip := panel.RelatedRecordChip{Label: "Tiket"}
	c, err := h.q(ctx).CountTicketsByAccount(ctx, accountID)
	if err != nil {
		h.Log.Error("accounts: count tickets", "err", err)
		return chip
	}
	label := strconv.FormatInt(c.OpenCount, 10) + " terbuka"
	if c.BreachedCount > 0 {
		label += ", " + strconv.FormatInt(c.BreachedCount, 10) + " breach"
	}
	chip.ValueText = label
	return chip
}
