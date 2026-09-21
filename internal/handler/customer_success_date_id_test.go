package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// customer_success_date_id_test.go — kunci format tampil locale Indonesia
// (dateStrID/dateTimeStrID, customer_success_date_id.go) yang dipakai HANYA
// oleh customerSuccessDetailView (halaman baca) — bukan customerSuccessFormFields
// (form edit, tetap ISO agar <input> bolak-balik, lihat sales_format.go).
// Kasus bulan dipilih agar mencakup nama yang BEDA dari Inggris (semua kecuali
// April/Mei~May/Juni~June/Juli~July — 9 dari 12 beda ejaan/urutan huruf).

func TestDateStrID(t *testing.T) {
	cases := []struct {
		name string
		d    pgtype.Date
		want string
	}{
		{"NULL", pgtype.Date{}, ""},
		{"Januari", pgtype.Date{Time: time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC), Valid: true}, "05 Januari 2026"},
		{"Februari", pgtype.Date{Time: time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC), Valid: true}, "28 Februari 2026"},
		{"Maret", pgtype.Date{Time: time.Date(2026, time.March, 28, 0, 0, 0, 0, time.UTC), Valid: true}, "28 Maret 2026"},
		{"Mei", pgtype.Date{Time: time.Date(2026, time.May, 12, 0, 0, 0, 0, time.UTC), Valid: true}, "12 Mei 2026"},
		{"Agustus", pgtype.Date{Time: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC), Valid: true}, "01 Agustus 2026"},
		{"September", pgtype.Date{Time: time.Date(2026, time.September, 21, 0, 0, 0, 0, time.UTC), Valid: true}, "21 September 2026"},
		{"Oktober", pgtype.Date{Time: time.Date(2026, time.October, 9, 0, 0, 0, 0, time.UTC), Valid: true}, "09 Oktober 2026"},
		{"Desember", pgtype.Date{Time: time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC), Valid: true}, "31 Desember 2026"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := dateStrID(c.d); got != c.want {
				t.Errorf("dateStrID(%v) = %q, want %q", c.d, got, c.want)
			}
		})
	}
}

func TestDateTimeStrID(t *testing.T) {
	cases := []struct {
		name string
		ts   pgtype.Timestamptz
		want string
	}{
		{"NULL", pgtype.Timestamptz{}, ""},
		{"dgn jam & menit dua digit", pgtype.Timestamptz{Time: time.Date(2026, time.September, 8, 6, 59, 0, 0, time.UTC), Valid: true}, "08 September 2026 06:59"},
		{"UTC konversi — bukan zona lokal", pgtype.Timestamptz{Time: time.Date(2026, time.January, 1, 23, 5, 0, 0, time.FixedZone("WIB", 7*3600)), Valid: true}, "01 Januari 2026 16:05"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := dateTimeStrID(c.ts); got != c.want {
				t.Errorf("dateTimeStrID(%v) = %q, want %q", c.ts, got, c.want)
			}
		})
	}
}

// TestCustomerSuccessDetail_TanggalFormatIndonesia — regresi end-to-end:
// halaman detail (bukan form edit) wajib merender tanggal ala Indonesia, BUKAN
// ISO mentah — mengunci wiring customerSuccessDetailView (bukan cuma fungsi
// formatnya sendiri).
func TestCustomerSuccessDetail_TanggalFormatIndonesia(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Tanggal ID", &uid, nil, nil)
	env.makeCustomer(t, a.ID, "Active")

	health, trend := "Healthy", "Improving"
	lifecycle, onboarding, freq, usage := "Onboarding", "In Progress", "Weekly", "Increasing"
	adoption, engagement, support, sentiment := int16(80), int16(90), int16(70), int16(60)
	progress := int16(50)
	activeUsers := int32(12)
	_, err := env.q.CreateCustomerSuccess(t.Context(), db.CreateCustomerSuccessParams{
		TenantID:           env.tenantID,
		AccountID:          a.ID,
		OverallHealthScore: computeOverallHealthScore(&adoption, &engagement, &support, &sentiment),
		HealthStatus:       &health,
		AdoptionScore:      &adoption,
		EngagementScore:    &engagement,
		SupportScore:       &support,
		SentimentScore:     &sentiment,
		ScoreTrend:         &trend,
		// 15 Juni 2026 dipilih sengaja BUKAN Sept 2026 — internal/changelog punya
		// entri berdampingan tanggal "2026-09-0x/1x" yang juga muncul di layout
		// AppShell (widget "Yang Baru"), collide dgn assert negatif ISO di bawah.
		HealthLastCalculated: pgtype.Timestamptz{Time: time.Date(2026, time.June, 15, 6, 59, 0, 0, time.UTC), Valid: true},
		LifecycleStage:       &lifecycle,
		StageEntryDate:       pgtype.Date{Time: time.Date(2026, time.March, 28, 0, 0, 0, 0, time.UTC), Valid: true},
		OnboardingStatus:     &onboarding,
		OnboardingProgress:   &progress,
		ActiveUsers:          &activeUsers,
		LoginFrequency:       &freq,
		UsageTrend:           &usage,
		UsageDataSource:      "Manual",
	})
	if err != nil {
		t.Fatalf("seed customer success: %v", err)
	}

	req := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", nil, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessDetail)
	if rec.Code != http.StatusOK {
		t.Fatalf("harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "28 Maret 2026") {
		t.Errorf("StageEntryDate harus dirender '28 Maret 2026' (locale ID), body:\n%s", body)
	}
	if !strings.Contains(body, "15 Juni 2026 06:59") {
		t.Errorf("HealthLastCalculated harus dirender '15 Juni 2026 06:59' (locale ID), body:\n%s", body)
	}
	if strings.Contains(body, "2026-03-28") || strings.Contains(body, "2026-06-15") {
		t.Errorf("halaman detail tidak boleh lagi merender tanggal ISO mentah, body:\n%s", body)
	}
}
