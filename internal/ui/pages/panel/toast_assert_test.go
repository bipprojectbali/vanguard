package panel

import (
	"strings"
	"testing"
)

// toast_assert_test.go — helper bersama BL-156 utk paket panel. Sebelum merge
// rollout (156b-g), tiap branch mendefinisikan assertToast LOKAL sendiri
// (independen dari main) — begitu semua digabung ke main, definisi ganda
// bentrok (go vet: "redeclared in this block"). Dikonsolidasi di sini SATU
// kali; file *_toast_test.go lain hanya memanggilnya.
func assertToast(t *testing.T, out, kind, msg string) {
	t.Helper()
	alertClass := "alert-error"
	if kind == "ok" {
		alertClass = "alert-success"
	}
	for _, want := range []string{"fixed", "pointer-events:none", "toast-flash", alertClass, msg} {
		if !strings.Contains(out, want) {
			t.Errorf("toast kurang %q:\n%s", want, out)
		}
	}
}
