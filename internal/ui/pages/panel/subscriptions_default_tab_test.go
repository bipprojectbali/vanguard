package panel

import (
	"strings"
	"testing"
)

// subscriptions_default_tab_test.go — regresi BL-20 (kontrak URL, murni-data):
// tab "Semua" mengirim penanda eksplisit ?status=all (bukan "") agar tetap
// TERJANGKAU ketika landing murni default ke Active; penanda status di-thread ke
// pager. Perilaku default-Active & terjemahan "all"→tak-menyaring diuji di sisi
// handler (subscriptions_default_tab_test.go paket handler).

func TestSubList_SemuaTabUsesAllMarker(t *testing.T) {
	// Sedang di tab Active (default landing). Tab "Semua" harus tetap punya URL.
	out := renderLeads(t, SubList(SubListView{
		Base:         "/w/desa",
		StatusFilter: "Active",
		Statuses:     []string{"Active", "Trial", "Cancelled"},
		Items:        []SubRow{{ID: 1, Village: "Desa Satu"}},
	}))
	if !strings.Contains(out, `href="/w/desa/subscriptions?status=all"`) {
		t.Errorf("tab 'Semua' harus menaut ke ?status=all (tetap terjangkau di default Active):\n%s", out)
	}
}

func TestSubList_SemuaTabActiveAndThreads(t *testing.T) {
	// Sedang di tab "Semua" (StatusFilter = SubStatusAll). Tab ditandai aktif &
	// penanda 'all' di-thread ke pager.
	out := renderLeads(t, SubList(SubListView{
		Base:         "/w/desa",
		StatusFilter: SubStatusAll,
		Statuses:     []string{"Active", "Trial"},
		Items:        []SubRow{{ID: 1, Village: "Desa Satu"}},
		NextCursor:   "42_7",
	}))
	// Pager membawa after + penanda status=all.
	for _, want := range []string{"after=42_7", "status=all"} {
		if !strings.Contains(out, want) {
			t.Errorf("pager harus membawa %q saat tab Semua:\n%s", want, out)
		}
	}
	// Tab "Semua" ditandai aktif (tab-active) saat StatusFilter = all.
	if !strings.Contains(out, "tab-active") {
		t.Errorf("tab aktif harus ditandai tab-active:\n%s", out)
	}
}
