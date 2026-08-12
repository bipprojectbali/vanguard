package handler

import (
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// plans_form.go — parsing & validasi form Plan, dipakai bersama create & update.
// Dipisah dari handler aksi agar aturan validasi (enum, angka, wajib) punya SATU
// tempat: create & edit tak boleh menerima nilai yang berbeda sahnya untuk kolom
// yang sama. Meniru sales_deals_form.go.
//
// plan_category = CERMIN CHECK plans_category_chk (migrasi 00005). billing_frequency
// TANPA CHECK di DB (kolom teks nullable) — divalidasi di handler demi konsistensi
// pelaporan (dropdown menawarkan set tetap). currency default IDR bila kosong
// (kolom NOT NULL DEFAULT 'IDR'); tak divalidasi enum (bebas per workspace).

const maxPlanNameLen = 200

// defaultPlanCurrency = fallback saat form tak mengisi currency. Cermin DEFAULT
// kolom plans.currency (migrasi 00005). Named-const, bukan literal telanjang.
const defaultPlanCurrency = "IDR"

var (
	// validPlanCategories = kategori plan (cermin plans_category_chk). Wajib.
	validPlanCategories = map[string]struct{}{
		"Core": {}, "Add-on": {}, "Module": {},
	}
	// validBillingFrequencies = siklus tagih. DB tak punya CHECK; divalidasi di
	// handler agar hanya nilai yang dropdown tawarkan yang tersimpan. Opsional.
	validBillingFrequencies = map[string]struct{}{
		"Monthly": {}, "Annual": {},
	}
)

// planForm = nilai form Plan yang SUDAH divalidasi & siap dipetakan ke Create/
// UpdatePlanParams. Kolom opsional pointer (nil = NULL); harga pgtype.Numeric
// (nil-valid = NULL). is_active TAK di sini — pensiun/aktifkan jalur tersendiri.
type planForm struct {
	PlanName         string
	PlanCode         string
	PlanCategory     string
	Description      *string
	BasePrice        pgtype.Numeric
	BillingFrequency *string
	SetupFee         pgtype.Numeric
	Currency         string
	IncludedFeatures *string
}

// parsePlanForm membaca & memvalidasi form. (form, "") bila sah, atau (zero,
// kode) yang dipetakan plansErrMsg. Nilai user-controlled → validasi backend
// adalah penjaga sesungguhnya. Keunikan plan_code diverifikasi DB (idx_plans_code)
// → ditangani planWriteErr, bukan di sini.
func parsePlanForm(fv func(string) string) (planForm, string) {
	var f planForm

	f.PlanName = strings.TrimSpace(fv("plan_name"))
	f.PlanCode = strings.TrimSpace(fv("plan_code"))
	if f.PlanName == "" || len(f.PlanName) > maxPlanNameLen || f.PlanCode == "" {
		return planForm{}, "required"
	}

	// plan_category wajib & harus enum (cermin CHECK).
	f.PlanCategory = strings.TrimSpace(fv("plan_category"))
	if _, ok := validPlanCategories[f.PlanCategory]; !ok {
		return planForm{}, "category"
	}

	// billing_frequency opsional: kosong = NULL; terisi wajib enum.
	if s := strings.TrimSpace(fv("billing_frequency")); s != "" {
		if _, ok := validBillingFrequencies[s]; !ok {
			return planForm{}, "billing"
		}
		f.BillingFrequency = &s
	}

	// Harga dasar & biaya setup: kosong = NULL; terisi wajib desimal sah.
	base, code := optNumeric(fv("base_price"), "base_price")
	if code != "" {
		return planForm{}, code
	}
	f.BasePrice = base

	setup, code := optNumeric(fv("setup_fee"), "setup_fee")
	if code != "" {
		return planForm{}, code
	}
	f.SetupFee = setup

	// currency default IDR bila kosong (kolom NOT NULL DEFAULT 'IDR').
	f.Currency = strings.TrimSpace(fv("currency"))
	if f.Currency == "" {
		f.Currency = defaultPlanCurrency
	}

	// Teks bebas opsional: trim, kosong → NULL.
	f.Description = optTrim(fv("description"))
	f.IncludedFeatures = optTrim(fv("included_features"))

	return f, ""
}

// planCategoryOptions / billingFrequencyOptions = opsi dropdown BERURUT (map
// validasi tak berurutan). Nilai HARUS himpunan sama dengan map validasi; urutan
// untuk tampilan.
var (
	planCategoryOptions     = []string{"Core", "Add-on", "Module"}
	billingFrequencyOptions = []string{"Monthly", "Annual"}
)

// compile-time: opsi & map validasi sepakat (panjang sama). Berbeda = dropdown
// menawarkan nilai yang ditolak backend, atau sebaliknya.
var _ = func() struct{} {
	if len(planCategoryOptions) != len(validPlanCategories) ||
		len(billingFrequencyOptions) != len(validBillingFrequencies) {
		panic("plans: opsi enum tak sinkron dengan map validasi")
	}
	return struct{}{}
}()

// planFormFields memetakan Plan → nilai prefill form (semua string; nil → "").
func planFormFields(p db.Plan) panel.PlanFormFields {
	return panel.PlanFormFields{
		PlanName:         p.PlanName,
		PlanCode:         p.PlanCode,
		PlanCategory:     p.PlanCategory,
		Description:      deref(p.Description),
		BasePrice:        numericStr(p.BasePrice),
		BillingFrequency: deref(p.BillingFrequency),
		SetupFee:         numericStr(p.SetupFee),
		Currency:         p.Currency,
		IncludedFeatures: deref(p.IncludedFeatures),
	}
}
