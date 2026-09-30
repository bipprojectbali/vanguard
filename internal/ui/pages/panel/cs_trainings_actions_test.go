package panel

import (
	"strings"
	"testing"
)

func renderTrainingActions(t *testing.T, status string) string {
	t.Helper()
	var sb strings.Builder
	if err := csTrainingStatusForm("/w/desa", CSTrainingRow{ID: 7, StatusLabel: status}, nil, "").Render(&sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	return sb.String()
}

// TestCSTrainingActions_RescheduleIsModal: BL-180 — "Jadwal Ulang" = tombol popovertarget
// + dialog popover berisi form native POST status=rescheduled,
// bukan panel inline Datastar.
func TestCSTrainingActions_RescheduleIsModal(t *testing.T) {
	out := renderTrainingActions(t, "Scheduled")
	for _, want := range []string{
		`popovertarget="resc-7"`,
		`id="resc-7"`,
		`action="/w/desa/trainings/7/status"`,
		`value="rescheduled"`,
		`name="training_date"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("harus memuat %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "resc7") {
		t.Errorf("signal inline resc7 tak boleh tersisa:\n%s", out)
	}
}

// TestCSTrainingActions_RescheduledRowHasNoRescheduleModal: baris Rescheduled tak
// menawarkan jadwal-ulang lagi.
func TestCSTrainingActions_RescheduledRowHasNoRescheduleModal(t *testing.T) {
	out := renderTrainingActions(t, "Rescheduled")
	if strings.Contains(out, "resc-7") {
		t.Errorf("baris Rescheduled tak boleh punya modal jadwal ulang:\n%s", out)
	}
}

// TestCSTrainingActions_DoneStaysInline: Selesai tetap panel inline (signal done7).
func TestCSTrainingActions_DoneStaysInline(t *testing.T) {
	out := renderTrainingActions(t, "Scheduled")
	if !strings.Contains(out, "done7") {
		t.Errorf("panel Selesai inline harus tetap memakai signal done7:\n%s", out)
	}
}
