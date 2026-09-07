package panel

import (
	"strings"
	"testing"
)

// account_typeahead_test.go — BL-76: pemilih desa/target di form CS/Support &
// aktivitas kini typeahead CSP-safe (datalist + input teks + hidden + skrip
// same-origin), BUKAN <select> polos. Satu markup dari account_typeahead.go.

// assertDesaTypeahead memverifikasi markup picker desa (account_id) + regresi
// bahwa <select> polos sudah tak dipakai lagi.
func assertDesaTypeahead(t *testing.T, out string, wantNames ...string) {
	t.Helper()
	want := []string{
		`data-account-picker`,             // wadah pemilih
		`<datalist id="account-options">`, // sumber opsi native (CSP-safe)
		`list="account-options"`,          // input tampak terhubung ke datalist
		`data-account-search`,             // kait input ketik
		`name="account_id"`,               // nilai id yang ter-submit
		`type="hidden"`,                   // id disimpan di input tersembunyi
		`data-account-value`,              // kait input hidden
		`/static/accountpicker.js`,        // skrip typeahead same-origin dimuat
	}
	want = append(want, wantNames...)
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("picker desa typeahead harus memuat %q:\n%s", w, out)
		}
	}
	// Regresi: account_id tak boleh lagi <select> polos.
	if strings.Contains(out, `<select id="account_id"`) {
		t.Errorf("account_id tak boleh lagi <select> polos:\n%s", out)
	}
}

func TestTicketForm_DesaTypeahead(t *testing.T) {
	out := renderLeads(t, TicketForm(TicketFormView{
		Base:   "/w/acme",
		Action: "/w/acme/tickets",
		Accounts: []TicketAccountOption{
			{ID: 7, Name: "3201052008 — Sukamaju"},
			{ID: 12, Name: "Cibeureum"},
		},
		Priorities: []string{"sedang"},
	}))
	assertDesaTypeahead(t, out, `data-account-id="7"`, `data-account-id="12"`, "Sukamaju", "Cibeureum")
}

func TestEngagementForm_DesaTypeahead(t *testing.T) {
	out := renderLeads(t, EngagementForm(EngagementFormView{
		Base:   "/w/acme",
		Action: "/w/acme/engagements",
		Accounts: []EngagementAccountOption{
			{ID: 7, Name: "3201052008 — Sukamaju"},
		},
		Types: []string{"touch_point"},
	}))
	assertDesaTypeahead(t, out, `data-account-id="7"`, "Sukamaju")
}

func TestCSTrainingForm_DesaTypeahead(t *testing.T) {
	out := renderLeads(t, CSTrainingForm(CSTrainingFormView{
		Base:   "/w/acme",
		Action: "/w/acme/trainings",
		Accounts: []CSTrainingAccountOption{
			{ID: 7, Name: "3201052008 — Sukamaju"},
		},
	}))
	assertDesaTypeahead(t, out, `data-account-id="7"`, "Sukamaju")
}

func TestCSImplTaskForm_DesaTypeahead(t *testing.T) {
	out := renderLeads(t, CSImplTaskForm(CSImplTaskFormView{
		Base:   "/w/acme",
		Action: "/w/acme/impl-tasks",
		Accounts: []CSImplTaskAccountOption{
			{ID: 7, Name: "3201052008 — Sukamaju"},
		},
	}))
	assertDesaTypeahead(t, out, `data-account-id="7"`, "Sukamaju")
}

// activityCreateFixture = form aktivitas mode CREATE dgn target polimorfik.
func activityCreateFixture() ActivityFormView {
	return ActivityFormView{
		Base:   "/w/acme",
		Action: "/w/acme/activities",
		Kind:   "task",
		Kinds:  []string{"task"},
		Targets: []ActivityTargetOption{
			{Value: "deal:5", Label: "Deal · Langganan 2026"},
			{Value: "account:7", Label: "Desa · 3201052008 — Sukamaju"},
			{Value: "contact:3", Label: "Kontak · Budi"},
		},
	}
}

// TestActivityForm_TargetTypeahead: picker target polimorfik (deal/desa/kontak)
// kini typeahead — nilai opaque "type:id", datalist tersendiri, pesan validity
// kustom. BUKAN <select> polos lagi.
func TestActivityForm_TargetTypeahead(t *testing.T) {
	out := renderLeads(t, ActivityForm(activityCreateFixture()))

	for _, want := range []string{
		`data-account-picker`,
		`<datalist id="target-options">`,               // datalist sendiri (bukan account-options)
		`list="target-options"`,                        //
		`name="target"`,                                // field ter-submit
		`data-invalid-msg="Pilih target dari daftar."`, // pesan validity kustom
		`data-account-id="deal:5"`,                     // nilai opaque "type:id"
		`data-account-id="account:7"`,
		`data-account-id="contact:3"`,
		"Deal · Langganan 2026", "Desa · 3201052008 — Sukamaju", "Kontak · Budi",
		`/static/accountpicker.js`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("picker target typeahead harus memuat %q:\n%s", want, out)
		}
	}
	// Regresi: target tak boleh lagi <select> polos.
	if strings.Contains(out, `<select id="f-target"`) {
		t.Errorf("target tak boleh lagi <select> polos:\n%s", out)
	}
}

// TestActivityForm_TargetPrefillOnCreate: pra-isi target (mis. "tambah aktivitas
// utk deal ini") harus mengisi input teks dgn label & hidden dgn nilai — label
// diturunkan dari Options meski TargetLabel kosong.
func TestActivityForm_TargetPrefillOnCreate(t *testing.T) {
	v := activityCreateFixture()
	v.TargetValue = "account:7" // hanya value; label harus dicari dari Options
	out := renderLeads(t, ActivityForm(v))

	if !strings.Contains(out, `name="target" data-account-value="" value="account:7"`) {
		t.Errorf("hidden target harus preselect \"account:7\":\n%s", out)
	}
	if !strings.Contains(out, `value="Desa · 3201052008 — Sukamaju"`) {
		t.Errorf("input teks target harus preselect label desa:\n%s", out)
	}
}
