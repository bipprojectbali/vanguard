package panel

import (
	"strings"
	"testing"
)

// sales_activities_test.go — regresi kolom Target pada tabel Sales Activities
// (ActivitiesList). BUG 14 Sep: aktivitas ber-target lead dirender "—" (bukan
// "Lead #id") karena activityTargetParts belum punya case "lead" — Lead baru
// ditambahkan ke picker target (BL-160) tapi lupa disinkronkan ke tabel daftar.
// activityTargetLink dipakai juga oleh AllActivitiesList (all_activities.go),
// jadi satu fix ini menutup keduanya.

func TestActivitiesList_TargetColumnAllTypes(t *testing.T) {
	out := renderLeads(t, ActivitiesList(ActivitiesListView{
		Base: "/w/desa",
		Items: []ActivityRow{
			{ID: 1, Subject: "Tindak lanjut deal", TargetType: "deal", TargetID: 199},
			{ID: 2, Subject: "Tindak lanjut desa", TargetType: "account", TargetID: 189},
			{ID: 3, Subject: "Tindak lanjut kontak", TargetType: "contact", TargetID: 386},
			{ID: 4, Subject: "Tindak lanjut lead", TargetType: "lead", TargetID: 42},
		},
	}))

	for _, tc := range []struct{ href, label string }{
		{`/w/desa/deals/199`, "Deal #199"},
		{`/w/desa/accounts/189`, "Desa #189"},
		{`/w/desa/contacts/386`, "Kontak #386"},
		{`/w/desa/leads/42`, "Lead #42"}, // regresi: dulu dirender "—"
	} {
		if !strings.Contains(out, `href="`+tc.href+`"`) || !strings.Contains(out, tc.label) {
			t.Errorf("kolom Target harus memuat tautan %q berlabel %q:\n%s", tc.href, tc.label, out)
		}
	}
}

// TestActivitiesList_TargetColumnUnknownTypeFallsBackToDash: tipe target tak
// dikenal (seharusnya tak terjadi lewat picker) tetap fallback aman "—", bukan
// tautan rusak.
func TestActivitiesList_TargetColumnUnknownTypeFallsBackToDash(t *testing.T) {
	out := renderLeads(t, ActivitiesList(ActivitiesListView{
		Base:  "/w/desa",
		Items: []ActivityRow{{ID: 1, Subject: "Tipe asing", TargetType: "unknown", TargetID: 1}},
	}))
	if !strings.Contains(out, ">—<") {
		t.Errorf("tipe target tak dikenal harus fallback \"—\":\n%s", out)
	}
}
