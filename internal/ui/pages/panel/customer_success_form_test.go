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
		LifecycleStages:    []string{"Onboarding", "Adoption", "Retention", "Renewal", "Advocacy"},
		OnboardingStatuses: []string{"Not Started", "In Progress", "Completed", "Stalled"},
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

// TestCSForm_LifecycleOnboardingLock (BL-178.1/.2): dua signal klien
// ($lifecycle, $onbstatus) hadir di data-signals form, dan opsi dropdown
// saling mengunci reaktif — jaring UX di atas K1 backend
// (checkOnboardingLifecycleConsistency), bukan penegak sesungguhnya.
func TestCSForm_LifecycleOnboardingLock(t *testing.T) {
	out := renderLeads(t, CustomerSuccessForm(csFormFixture()))

	if !strings.Contains(out, `data-signals="{&#34;lifecycle&#34;:&#34;&#34;,&#34;onbstatus&#34;:&#34;&#34;}"`) {
		t.Errorf("form harus deklarasikan signal $lifecycle & $onbstatus:\n%s", out)
	}

	// Opsi tahap siklus hidup PASCA-onboarding dikunci selama onboarding_status
	// belum "Completed"; "—" & "Onboarding" TETAP selalu aktif (BL-178.2).
	postOnboarding := []string{"Adoption", "Retention", "Renewal", "Advocacy"}
	for _, stage := range postOnboarding {
		want := `<option value="` + stage + `" data-attr="{disabled: $onbstatus != &#39;Completed&#39;}">` + stage + `</option>`
		if !strings.Contains(out, want) {
			t.Errorf("opsi lifecycle_stage %q harus dikunci ke $onbstatus != 'Completed':\n%s", stage, out)
		}
	}
	if strings.Contains(out, `<option value="Onboarding" data-attr`) {
		t.Errorf("opsi lifecycle_stage \"Onboarding\" TAK boleh dikunci:\n%s", out)
	}

	// Opsi status onboarding SELAIN "Completed" dikunci saat lifecycle_stage
	// sudah pasca-onboarding; "Completed" TETAP selalu bisa dipilih (BL-178.1).
	lockedOnboarding := []string{"", "Not Started", "In Progress", "Stalled"}
	wantExpr := `data-attr="{disabled: $lifecycle == &#39;Adoption&#39; || $lifecycle == &#39;Retention&#39; || $lifecycle == &#39;Renewal&#39; || $lifecycle == &#39;Advocacy&#39;}"`
	for _, status := range lockedOnboarding {
		want := `<option value="` + status + `" ` + wantExpr
		if !strings.Contains(out, want) {
			t.Errorf("opsi onboarding_status %q harus dikunci ke lifecycle pasca-onboarding:\n%s", status, out)
		}
	}
	if strings.Contains(out, `<option value="Completed" data-attr`) {
		t.Errorf("opsi onboarding_status \"Completed\" TAK boleh dikunci:\n%s", out)
	}
}
