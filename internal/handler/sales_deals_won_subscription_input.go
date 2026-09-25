package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"go_starter/internal/db"
)

// sales_deals_won_subscription_input.go — validasi & pengumpulan input untuk
// subscriptionFromWonDeal (sales_deals_won_subscription.go): resolusi
// quote+termin, validasi status, item quote & plan induk (BL-88 PR2b), cek
// konflik one-active.

// wonSubscriptionInput = hasil validasi resolveWonSubscriptionInputs, siap
// dipakai subscriptionFromWonDeal untuk membuat Subscription.
type wonSubscriptionInput struct {
	quote      db.Quote
	term       string
	months     int32
	status     string
	items      []db.QuoteItem
	parentPlan *int64
}

// resolveWonSubscriptionInputs mengumpulkan & memvalidasi seluruh input untuk
// membuat langganan dari deal yang menang lewat form single-deal (`DealStage`):
// baca `subscription_status` dari form lalu delegasi ke resolveWonSubscriptionCore.
// Menulis wsRedirect & mengembalikan ok=false pada kegagalan apa pun; pemanggil
// (subscriptionFromWonDeal) WAJIB berhenti tanpa membuat apa pun.
func (h *Handler) resolveWonSubscriptionInputs(w http.ResponseWriter, r *http.Request, deal db.Deal) (wonSubscriptionInput, bool) {
	idStr := strconv.FormatInt(deal.ID, 10)
	// Status awal dari form (keputusan a). Wajib Trial/Active.
	status := strings.TrimSpace(r.FormValue("subscription_status"))
	input, reason := h.resolveWonSubscriptionCore(r.Context(), deal, status)
	if reason != "" {
		wsRedirect(w, r, "/deals/"+idStr, reason)
		return wonSubscriptionInput{}, false
	}
	return input, true
}

// resolveWonSubscriptionCore = inti resolveWonSubscriptionInputs TANPA efek samping
// HTTP (BL-75): status diterima sebagai parameter eksplisit (bukan dibaca dari
// r.FormValue) agar dipakai bersama oleh jalur single-deal (wrapper di atas, baca
// form) DAN DealStageBulk (nilai status sudah di-resolve per-baris dari mode
// shared/individual). Mengembalikan reason code non-kosong ("quote_required",
// "sub_status", "plan_required", "sub_active_exists", "failed") alih-alih menulis
// redirect — pemanggil single-deal menerjemahkannya ke wsRedirect, pemanggil bulk
// cukup skip baris (tak ada rollback tx parsial, lihat catatan subscriptionFromWonDeal).
func (h *Handler) resolveWonSubscriptionCore(ctx context.Context, deal db.Deal, status string) (wonSubscriptionInput, string) {
	// BL-88: quote otoritatif → langganan lahir dari quote Accepted, bukan field deal
	// manual. Tanpa quote Accepted, nilai & termin tak punya sumber sah → tolak (bukan
	// diam-diam) agar user meng-Accept quote dulu. Index idx_quotes_one_accepted menjamin
	// ≤1 → :one; ErrNoRows = belum ada quote Accepted.
	quote, err := h.q(ctx).GetAcceptedQuoteForDeal(ctx, &deal.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return wonSubscriptionInput{}, "quote_required"
		}
		h.Log.Error("deals: won accepted-quote", "deal_id", deal.ID, "err", err)
		return wonSubscriptionInput{}, "failed"
	}
	// Termin dari QUOTE (BL-88), bukan deal. subscription_term = billing cycle (harus
	// enum valid Monthly/Annual/Multi-year) + fallback bulan-kontrak; contract_term_months
	// (opsional) MENIMPA durasi numerik (mis. Multi-year = 24). Quote tanpa termin valid →
	// tolak (quote_required) agar user set termin di quote dulu.
	term := deref(quote.SubscriptionTerm)
	months, termOK := termContractMonths[term]
	if !termOK {
		return wonSubscriptionInput{}, "quote_required"
	}
	if quote.ContractTermMonths != nil && *quote.ContractTermMonths > 0 {
		months = *quote.ContractTermMonths
	}
	if _, ok := validInitialSubStatuses[status]; !ok {
		return wonSubscriptionInput{}, "sub_status"
	}
	// BL-88 PR2b: identitas paket ada di subscription_items (mirror quote_items). Ambil
	// baris quote LEBIH DULU: darinya diturunkan (a) plan_id parent — SATU paket distinct →
	// paket itu (kenyamanan single-plan agar label/JOIN lama tetap berguna); >1 → NULL
	// (identitas di item); (b) pre-check one-active lintas SEMUA paket quote. Quote tanpa
	// baris berpaket tak bisa jadi langganan bermakna → tolak (plan_required).
	items, err := h.q(ctx).ListQuoteItems(ctx, quote.ID)
	if err != nil {
		h.Log.Error("deals: won quote items", "deal_id", deal.ID, "quote_id", quote.ID, "err", err)
		return wonSubscriptionInput{}, "failed"
	}
	parentPlan, hasPlan := singleQuotePlan(items)
	if !hasPlan {
		return wonSubscriptionInput{}, "plan_required"
	}
	// Konflik one-active (keputusan c: TOLAK) — hanya menggigit bila status Active
	// (parent_active); Trial boleh koeksis. Pre-check lintas SEMUA paket quote beri pesan
	// ramah; idx_subscription_items_one_active tetap penjaga keras bila balapan.
	if status == "Active" {
		exists, err := h.q(ctx).HasActiveItemForQuotePlans(ctx, db.HasActiveItemForQuotePlansParams{
			AccountID: deal.AccountID,
			QuoteID:   quote.ID,
		})
		if err != nil {
			h.Log.Error("deals: won sub active-check", "deal_id", deal.ID, "err", err)
			return wonSubscriptionInput{}, "failed"
		}
		if exists {
			return wonSubscriptionInput{}, "sub_active_exists"
		}
	}
	return wonSubscriptionInput{
		quote: quote, term: term, months: months, status: status,
		items: items, parentPlan: parentPlan,
	}, ""
}

// singleQuotePlan menurunkan plan_id PARENT langganan dari baris quote (BL-88 PR2b):
// bila SEMUA baris berpaket menunjuk SATU paket distinct → (&paket, true) (kenyamanan
// single-plan agar label & JOIN plans lama tetap berguna); >1 paket distinct → (nil,
// true) (identitas ada di subscription_items, parent plan_id NULL); tak ada baris
// berpaket sama sekali → (nil, false) (quote tak bisa jadi langganan bermakna).
func singleQuotePlan(items []db.QuoteItem) (*int64, bool) {
	var seen *int64
	for _, it := range items {
		if it.PlanID == nil {
			continue
		}
		if seen == nil {
			p := *it.PlanID
			seen = &p
			continue
		}
		if *seen != *it.PlanID {
			return nil, true // >1 paket distinct → parent NULL
		}
	}
	return seen, seen != nil
}
