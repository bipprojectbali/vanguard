package handler

import "testing"

// sales_deals_form_test.go — unit parseDealForm untuk BL-8 (field Nilai numerik
// deal). Kontrak: backend penjaga sesungguhnya. Nilai (Rp) menerima input
// terkelompok "5.000.000" (dari numgroup.js maupun ketikan manual) sebagai
// 5000000; teks bukan-angka ditolak. moneyRupiahStr menjaga prefill edit tetap
// rupiah bulat (buang skala ".00") agar numgroup.js tak menggandakan 100x.
// parseDealForm murni (tak sentuh DB) → uji langsung. fvFromMap dari
// sales_leads_form_test.go (satu paket).

// dealBase = field wajib minimal (deal_name + account_id) agar parseDealForm
// lolos ke validasi amount; ditimpa per-kasus.
func dealBase(extra map[string]string) map[string]string {
	m := map[string]string{"deal_name": "Deal Contoh", "account_id": "1"}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func TestParseDealForm_NilaiTerkelompokDiterima(t *testing.T) {
	cases := map[string]struct {
		in   string
		want string // numericStr yang diharapkan ("" = NULL)
	}{
		"terkelompok titik": {"5.000.000", "5000000"},
		"digit polos":       {"5000000", "5000000"},
		"dengan spasi":      {"1 500 000", "1500000"},
		"kosong → NULL":     {"", ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f, code := parseDealForm(fvFromMap(dealBase(map[string]string{"amount": c.in})))
			if code != "" {
				t.Fatalf("amount %q ditolak (code=%q), harusnya diterima", c.in, code)
			}
			if got := numericStr(f.Amount); got != c.want {
				t.Errorf("amount %q → numericStr=%q, mau %q", c.in, got, c.want)
			}
		})
	}
}

func TestParseDealForm_NilaiBukanAngkaDitolak(t *testing.T) {
	_, code := parseDealForm(fvFromMap(dealBase(map[string]string{"amount": "abc"})))
	if code != "amount" {
		t.Errorf("amount non-angka: code=%q, mau %q", code, "amount")
	}
}

// BL-124: forecast_category kini enum (Pipeline/Best Case/Commit/Closed), bukan
// teks bebas. Backend penjaga: nilai di luar himpunan ditolak, kosong = NULL.
func TestParseDealForm_ForecastCategoryEnum(t *testing.T) {
	t.Run("nilai sah diterima", func(t *testing.T) {
		for _, v := range forecastCategoryOptions {
			f, code := parseDealForm(fvFromMap(dealBase(map[string]string{"forecast_category": v})))
			if code != "" {
				t.Fatalf("forecast %q ditolak (code=%q), harusnya diterima", v, code)
			}
			if f.ForecastCategory == nil || *f.ForecastCategory != v {
				t.Errorf("forecast %q → %v, mau pointer ke %q", v, f.ForecastCategory, v)
			}
		}
	})
	t.Run("kosong → NULL", func(t *testing.T) {
		f, code := parseDealForm(fvFromMap(dealBase(map[string]string{"forecast_category": ""})))
		if code != "" {
			t.Fatalf("forecast kosong ditolak (code=%q)", code)
		}
		if f.ForecastCategory != nil {
			t.Errorf("forecast kosong → %v, mau nil (NULL)", *f.ForecastCategory)
		}
	})
	t.Run("nilai asing ditolak", func(t *testing.T) {
		_, code := parseDealForm(fvFromMap(dealBase(map[string]string{"forecast_category": "Bukan Kategori"})))
		if code != "forecast" {
			t.Errorf("forecast asing: code=%q, mau %q", code, "forecast")
		}
	})
}

// moneyRupiahStr HARUS membuang bagian pecahan skala kolom NUMERIC(15,2). Tanpa
// ini prefill "7500000.00" akan dibaca numgroup.js sebagai digit "750000000"
// (100x) → korupsi nilai saat deal disunting & disimpan ulang.
func TestMoneyRupiahStr_BuangPecahan(t *testing.T) {
	cases := map[string]string{
		"7500000.00": "7500000",
		"7500000":    "7500000",
		"0.00":       "0",
		"":           "",
	}
	for in, want := range cases {
		if got := moneyRupiahStr(numFrom(t, in)); got != want {
			t.Errorf("moneyRupiahStr(%q) = %q, mau %q", in, got, want)
		}
	}
}
