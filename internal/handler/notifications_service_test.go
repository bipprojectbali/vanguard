package handler

import "testing"

// notifications_service_test.go — notifText murni (tanpa DB); dites terpisah
// dari notifications_test.go (yang butuh Postgres) karena fungsi ini pure.

// TestNotifText_ReminderHariH: DaysLeft=0 wajib kalimat "hari ini", BUKAN
// "dalam 0 hari" — pointer-lah yang membedakan dari payload lama/rusak
// (DaysLeft nil), jadi nilai 0 itu sendiri harus dirender khusus.
func TestNotifText_ReminderHariH(t *testing.T) {
	zero := 0
	p := notifPayload{VillageName: "Desa Makmur", DaysLeft: &zero}
	got := notifText("subscription.renewal.reminder", "abaikan", p)
	want := "Langganan Desa Makmur jatuh tempo hari ini."
	if got != want {
		t.Fatalf("notifText() = %q, mau %q", got, want)
	}
}

// TestNotifText_ReminderSisaHari: DaysLeft>0 menyebut jumlah hari eksplisit.
func TestNotifText_ReminderSisaHari(t *testing.T) {
	seven := 7
	p := notifPayload{VillageName: "Desa Makmur", DaysLeft: &seven}
	got := notifText("subscription.renewal.reminder", "abaikan", p)
	want := "Langganan Desa Makmur akan jatuh tempo dalam 7 hari."
	if got != want {
		t.Fatalf("notifText() = %q, mau %q", got, want)
	}
}

// TestNotifText_ReminderPayloadRusak: DaysLeft nil (payload lama/rusak) → tetap
// fail-soft dengan kalimat generik, bukan panic atau string kosong.
func TestNotifText_ReminderPayloadRusak(t *testing.T) {
	p := notifPayload{VillageName: "Desa Makmur"}
	got := notifText("subscription.renewal.reminder", "abaikan", p)
	want := "Langganan Desa Makmur mendekati masa jatuh tempo."
	if got != want {
		t.Fatalf("notifText() = %q, mau %q", got, want)
	}
}

// TestNotifText_ReminderTanpaVillageName: VillageName kosong → fallback default,
// bukan "Langganan  jatuh tempo..." dengan spasi ganda yang aneh.
func TestNotifText_ReminderTanpaVillageName(t *testing.T) {
	zero := 0
	p := notifPayload{DaysLeft: &zero}
	got := notifText("subscription.renewal.reminder", "abaikan", p)
	want := "Langganan Langganan Anda jatuh tempo hari ini."
	if got != want {
		t.Fatalf("notifText() = %q, mau %q", got, want)
	}
}
