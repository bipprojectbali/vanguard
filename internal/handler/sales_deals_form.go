package handler

import (
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_deals_form.go — parsing & validasi form Deal, dipakai bersama create &
// update. Dipisah dari handler aksi agar aturan validasi (enum, panjang, angka,
// tanggal) punya SATU tempat: create & edit tak boleh menerima nilai yang berbeda
// sahnya untuk kolom yang sama. Meniru sales_leads_form.go.
//
// Enum di sini = CERMIN CHECK constraint migrasi 00009 (deals_type_chk,
// deals_term_chk) — penolakan terjadi di sini SEBELUM DB; CHECK jaring terakhir.
// forecast_category SENGAJA teks bebas (tak ada CHECK di 00009) → tak divalidasi
// enum. Stage TAK diparse di sini: create memulai di 'Prospecting' dan perpindahan
// stage adalah aksi tersendiri (UpdateDealStage), bukan field form profil.

const maxDealNameLen = 200

// dealInitialStage = stage awal setiap deal baru (standalone create & hasil
// konversi lead). Perpindahan berikutnya lewat aksi stage tersendiri.
const dealInitialStage = "Prospecting"

var (
	// validDealTypes = tipe deal (cermin deals_type_chk). Opsional: kosong = NULL.
	validDealTypes = map[string]struct{}{
		"New Business": {}, "Renewal": {}, "Upsell": {}, "Cross-sell": {},
	}
	// validDealStages = tahap pipeline (cermin deals_stage_chk). Dipakai gate aksi
	// stage (UpdateDealStage), bukan form profil. Closed Won/Lost = stage terminal.
	validDealStages = map[string]struct{}{
		"Prospecting": {}, "Qualification": {}, "Demo": {}, "Proposal": {},
		"Negotiation": {}, "Closed Won": {}, "Closed Lost": {},
	}
	// validSubscriptionTerms = termin langganan (cermin deals_term_chk). Opsional.
	validSubscriptionTerms = map[string]struct{}{
		"Monthly": {}, "Annual": {}, "Multi-year": {},
	}
)

// dealForm = nilai form Deal yang SUDAH divalidasi & siap dipetakan ke
// Create/UpdateDealParams. Kolom opsional pointer (nil = NULL); Amount pgtype.Numeric
// (nil-valid = NULL); ExpectedCloseDate pgtype.Date (nil-valid = NULL). Stage,
// owner, primary_contact, plan_requested TAK di sini (jalur/preservasi terpisah).
type dealForm struct {
	DealName          string
	AccountID         int64
	DealType          *string
	Amount            pgtype.Numeric
	Probability       *int16
	ExpectedCloseDate pgtype.Date
	ForecastCategory  *string
	NextStep          *string
	Competitor        *string
}

// parseDealForm membaca & memvalidasi form. (form, "") bila sah, atau (zero,
// kode) yang dipetakan wsErrMsg. Nilai user-controlled → validasi backend adalah
// penjaga sesungguhnya. account_id wajib (deal selalu menempel ke satu desa).
func parseDealForm(fv func(string) string) (dealForm, string) {
	var f dealForm

	f.DealName = strings.TrimSpace(fv("deal_name"))
	if f.DealName == "" || len(f.DealName) > maxDealNameLen {
		return dealForm{}, "deal_name"
	}

	// account_id wajib & harus bilangan bulat positif. Keberadaan baris (dan
	// cakupan ownership) diverifikasi handler; di sini hanya bentuk.
	acc, err := strconv.ParseInt(strings.TrimSpace(fv("account_id")), 10, 64)
	if err != nil || acc <= 0 {
		return dealForm{}, "account_req"
	}
	f.AccountID = acc

	// deal_type opsional: kosong = NULL; terisi wajib enum.
	if s := strings.TrimSpace(fv("deal_type")); s != "" {
		if _, ok := validDealTypes[s]; !ok {
			return dealForm{}, "deal_type"
		}
		f.DealType = &s
	}
	// BL-88: subscription_term TIDAK lagi milik deal — pindah ke quote (quote otoritatif
	// atas nilai & termin komersial). Form deal tak lagi menerima/memvalidasinya.

	// Nilai PERKIRAAN deal (forecast pipeline tahap awal, BL-88): kosong = NULL; terisi
	// wajib angka sah. Bukan nilai diakui — itu disalin dari grand_total quote saat Accept.
	// Pemisah ribuan dibuang dulu (cleanThousands) agar input terkelompok "5.000.000" dari
	// numgroup.js — atau ketikan manual tanpa JS — sama-sama sah (BL-8, sejajar
	// estimated_value lead di BL-2). Rupiah bulat: titik = pemisah ribuan.
	amt, code := optNumeric(cleanThousands(fv("amount")), "amount")
	if code != "" {
		return dealForm{}, code
	}
	f.Amount = amt

	// Probabilitas 0–100 opsional.
	prob, code := optProbability(fv("probability"))
	if code != "" {
		return dealForm{}, code
	}
	f.Probability = prob

	// Tanggal perkiraan tutup opsional (belum dijadwalkan = NULL).
	ecd, code := optDate(fv("expected_close_date"))
	if code != "" {
		return dealForm{}, code
	}
	f.ExpectedCloseDate = ecd

	// Teks bebas opsional: trim, kosong → NULL. forecast_category tanpa CHECK.
	f.ForecastCategory = optTrim(fv("forecast_category"))
	f.NextStep = optTrim(fv("next_step"))
	f.Competitor = optTrim(fv("competitor"))

	return f, ""
}
