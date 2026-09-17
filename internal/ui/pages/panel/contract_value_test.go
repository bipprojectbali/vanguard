package panel

import (
	"strings"
	"testing"
)

// contract_value_test.go — indikator "Nilai Kontrak/MRR" SATU peran (BL-145
// subtask 3): sejak dipindah ke halaman detail peran, axis ini sepenuhnya
// mengikuti F2 (checkbox "Lihat ARR" baris Subscriptions), bukan lagi kartu
// F4 tenant-wide — lihat field_security.go.

func TestContractValueIndicator_Static(t *testing.T) {
	var out strings.Builder
	if err := contractValueIndicator(true, false).Render(&out); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := out.String()
	if strings.Contains(body, "<form") || strings.Contains(body, `method="post"`) {
		t.Error("indikator harus presentasional murni, tanpa form/POST")
	}
	if !strings.Contains(body, "Terlihat") {
		t.Error("visible=true harus tampil \"Terlihat\"")
	}
	if strings.Contains(body, "data-show") {
		t.Error("varian statis tak boleh punya data-show")
	}

	out.Reset()
	if err := contractValueIndicator(false, false).Render(&out); err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out.String(), "Tersembunyi") {
		t.Error("visible=false harus tampil \"Tersembunyi\"")
	}
}

func TestContractValueIndicator_Reactive(t *testing.T) {
	var out strings.Builder
	if err := contractValueIndicator(true, true).Render(&out); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := out.String()
	if !strings.Contains(body, "arr_subscriptions") {
		t.Error("varian reaktif harus merujuk signal arr_subscriptions")
	}
	if !strings.Contains(body, "Terlihat") || !strings.Contains(body, "Tersembunyi") {
		t.Error("varian reaktif harus merender KEDUA span (toggle via data-show)")
	}
}
