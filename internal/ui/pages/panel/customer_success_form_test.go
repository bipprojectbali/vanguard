package panel

import (
	"strings"
	"testing"
)

// customer_success_form_test.go — form sunting Customer Success:
//   - "Tren Skor" (BL-25) TAK dirender di form (BL-128 (b)).
//   - "Status Kesehatan" (BL-24) juga TAK dirender di form — tetap turunan skor,
//     dihitung backend & tampil di halaman DETAIL, hanya keluar dari form edit.
// (Pratinjau skor live BL-128 (a) di-descope: input komponen tetap field biasa.)

func csFormFixture() CustomerSuccessFormView {
	return CustomerSuccessFormView{
		Base:        "/w/desa",
		AccountName: "Desa Contoh",
		Action:      "/w/desa/accounts/7/customer-success",
		Fields: CustomerSuccessFormFields{
			AdoptionScore:   "80",
			EngagementScore: "60",
			SupportScore:    "",
			SentimentScore:  "",
		},
		CanWriteHealth:     true,
		CanWriteJourney:    true,
		CanWriteAdoption:   true,
		LifecycleStages:    []string{"Onboarding", "Adopting"},
		OnboardingStatuses: []string{"Not Started", "In Progress", "Completed"},
		LoginFrequencies:   []string{"Daily", "Weekly"},
		UsageTrends:        []string{"Increasing", "Stable"},
	}
}

// TestCSForm_TrenSkorDihapus: "Tren Skor" (BL-25) TAK dirender di form sunting
// (BL-128 (b)) — tetap dihitung backend, hanya keluar dari form edit.
func TestCSForm_TrenSkorDihapus(t *testing.T) {
	out := renderLeads(t, CustomerSuccessForm(csFormFixture()))
	if strings.Contains(out, "Tren Skor") {
		t.Errorf("BL-128 (b): 'Tren Skor' tak boleh dirender di form sunting:\n%s", out)
	}
}

// TestCSForm_StatusKesehatanDihapus: badge "Status Kesehatan" (BL-24) TAK
// dirender di form sunting — turunan skor, tampil di halaman DETAIL saja.
func TestCSForm_StatusKesehatanDihapus(t *testing.T) {
	out := renderLeads(t, CustomerSuccessForm(csFormFixture()))
	if strings.Contains(out, "Status Kesehatan") {
		t.Errorf("'Status Kesehatan' tak boleh dirender di form sunting:\n%s", out)
	}
}
