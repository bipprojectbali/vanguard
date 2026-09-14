package handler

import (
	"context"
	"strconv"
)

// sales_deals_labels.go — resolusi label best-effort (desa/kontak) untuk detail
// Deal, dipisah dari sales_deals_detail.go (ukuran file). Perilaku identik;
// hanya organisasi file yang berubah.

// accountLabel meresolusi nama desa untuk tautan (kode — nama). Gagal → "Desa
// #<id>" sebagai cadangan (bukan 500): detail deal tetap terbaca.
func (h *Handler) accountLabel(ctx context.Context, id int64) string {
	a, err := h.q(ctx).GetAccount(ctx, id)
	if err != nil {
		return "Desa #" + strconv.FormatInt(id, 10)
	}
	return accountPickerLabel(a.VillageCode, a.VillageName)
}

// contactLabel meresolusi nama kontak utama. Gagal → "Kontak #<id>" cadangan.
func (h *Handler) contactLabel(ctx context.Context, id int64) string {
	c, err := h.q(ctx).GetContact(ctx, id)
	if err != nil {
		return "Kontak #" + strconv.FormatInt(id, 10)
	}
	return contactFullName(c)
}

// dealLabel meresolusi label deal sumber (kode — nama), dipakai kartu System &
// Audit langganan (BL-154, "Source: Converted from DEAL-xxxx"). Gagal (di luar
// tenant/terhapus) → "Deal #<id>" cadangan, bukan 500.
func (h *Handler) dealLabel(ctx context.Context, id int64) string {
	d, err := h.q(ctx).GetDeal(ctx, id)
	if err != nil {
		return "Deal #" + strconv.FormatInt(id, 10)
	}
	code := deref(d.EntityCode)
	if code == "" {
		return d.DealName
	}
	return code + " — " + d.DealName
}
