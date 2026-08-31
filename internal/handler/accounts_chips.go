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

// accounts_chips.go — pembangun "chip" ringkas rekaman terkait di detail Account
// (Kontak, Deal, Langganan, Tiket). Dipisah dari accounts_rollups.go (kartu
// ringkasan lintas-modul) agar keduanya di bawah ambang tipe Route/Handler (150).
// Semua fungsi BEST-EFFORT: kegagalan query/pgx.ErrNoRows → nilai kosong, tak
// pernah menggagalkan halaman.
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
