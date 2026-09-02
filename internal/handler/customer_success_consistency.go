package handler

import "go_starter/internal/db"

// customer_success_consistency.go — keselarasan onboarding ↔ lifecycle (BL-26).
// SATU baris customer_success, tapi lifecycle_stage & onboarding_status (+
// onboarding_progress/actual_go_live_date) diisi MANUAL & INDEPENDEN → mungkin
// kombinasi mustahil (mis. tahap sudah Adoption tapi onboarding belum selesai).
// Helper MURNI di sini (tanpa HTTP) agar bisa diuji langsung; dipanggil
// CustomerSuccessSave (normalisasi progress + guard K1/K3) & handler GET
// (peringatan lunak K2/K4 utk banner). Keputusan user (2 Sep): guard K1/K3
// (tolak simpan), warn K2/K4 (banner, tak memblok).

// Nilai onboarding_progress terminal yang DIPAKSA mengikuti status (BL-26 c):
// status yang maknanya sudah pasti tak menyisakan ruang untuk angka bebas.
// Konstanta bernama (rule §15), bukan literal telanjang di logika.
const (
	onboardingProgressNotStarted = 0   // "Not Started" → progres 0
	onboardingProgressCompleted  = 100 // "Completed"   → progres 100
)

// Nilai enum onboarding_status & lifecycle_stage yang dirujuk logika keselarasan.
// Cermin CHECK migrasi 00019 & validOnboardingStatuses/validLifecycleStages
// (customer_success_helpers.go) — bila enum berubah, ubah di sini juga.
const (
	onboardingNotStarted = "Not Started"
	onboardingCompleted  = "Completed"
	lifecycleOnboarding  = "Onboarding"
)

// Kode galat guard (K1/K3) — dipetakan customerSuccessErrMsg (helpers) ke pesan
// jelas, dipakai PRG ?err= (gotcha #16, native POST→303).
const (
	errOnboardingLifecycleMismatch = "onboarding_lifecycle_mismatch" // K1
	errOnboardingGoLiveMismatch    = "onboarding_golive_mismatch"    // K3
)

// Pesan peringatan LUNAK (K2/K4) — tak memblok simpan, ditampilkan sebagai
// banner di form & detail. String di sini (bukan view) agar bisa di-assert test.
const (
	warnOnboardingCompletedNoGoLive        = "Status onboarding \"Completed\", tetapi Tanggal Go-Live Aktual belum diisi."
	warnOnboardingCompletedStillOnboarding = "Onboarding sudah \"Completed\", tetapi tahap siklus hidup masih \"Onboarding\" — pertimbangkan memajukan ke \"Adoption\"."
)

// postOnboardingStages = tahap siklus hidup DI LUAR Onboarding (sudah lulus
// onboarding). Salah satunya + onboarding_status ≠ Completed = kontradiksi K1.
var postOnboardingStages = map[string]struct{}{
	"Adoption": {}, "Retention": {}, "Renewal": {}, "Advocacy": {},
}

// normalizeOnboardingProgress memaksa onboarding_progress ke nilai terminal yang
// pasti mengikuti status (BL-26 c): "Not Started" → 0, "Completed" → 100, APA PUN
// isi field. UI menyembunyikan input saat status terminal (data-show), tapi field
// tersembunyi tetap TERKIRIM dengan nilai basi → normalisasi backend inilah
// penegaknya (menyembunyikan UI saja tak cukup). Hanya {In Progress, Stalled}
// menghormati angka input (0–100, sudah divalidasi optScore). status nil (belum
// dipilih) → pertahankan raw apa adanya. Dipanggil HANYA saat section Journey
// berhak ditulis aktor (F2) — lihat CustomerSuccessSave, jangan overwrite nilai
// lama yang ter-mask.
func normalizeOnboardingProgress(status *string, raw *int16) *int16 {
	if status == nil {
		return raw
	}
	switch *status {
	case onboardingNotStarted:
		v := int16(onboardingProgressNotStarted)
		return &v
	case onboardingCompleted:
		v := int16(onboardingProgressCompleted)
		return &v
	default: // In Progress, Stalled — di tengah jalan, angka bermakna
		return raw
	}
}

// checkOnboardingLifecycleConsistency menegakkan kontradiksi MUSTAHIL (guard,
// K1/K3): kembalikan (kode, false) untuk memblok simpan (PRG ?err=), atau
// ("", true) bila selaras. K2/K4 (lemah/borderline) TIDAK diblok di sini —
// ditangani onboardingConsistencyWarnings (banner). Beroperasi atas form yang
// SUDAH ter-masking F2 & ter-normalisasi progress.
func checkOnboardingLifecycleConsistency(f customerSuccessForm) (string, bool) {
	// K1: tahap sudah lewat Onboarding (Adoption/…/Advocacy) tetapi onboarding
	// belum "Completed" — mustahil naik tahap sambil onboarding belum selesai.
	if f.LifecycleStage != nil {
		if _, post := postOnboardingStages[*f.LifecycleStage]; post {
			if f.OnboardingStatus == nil || *f.OnboardingStatus != onboardingCompleted {
				return errOnboardingLifecycleMismatch, false
			}
		}
	}
	// K3: status "Not Started" tetapi Go-Live Aktual sudah terisi — belum mulai
	// onboarding mustahil sudah go-live.
	if f.OnboardingStatus != nil && *f.OnboardingStatus == onboardingNotStarted && f.ActualGoLiveDate.Valid {
		return errOnboardingGoLiveMismatch, false
	}
	return "", true
}

// onboardingConsistencyWarnings mengembalikan peringatan LUNAK (K2/K4) atas baris
// CS TERSIMPAN — ditampilkan sebagai banner di detail & form (tak memblok).
// Sub-kasus K2 "progress<100" sudah MUSTAHIL berkat normalizeOnboardingProgress
// (Completed selalu 100), jadi K2 di sini murni "Completed tapi go-live kosong".
// Dipanggil HANYA bila aktor berhak MEMBACA section Journey (jangan bocorkan
// keadaan section yang disembunyikan) — lihat pemanggil di handler.
func onboardingConsistencyWarnings(cs db.CustomerSuccess) []string {
	status, stage := deref(cs.OnboardingStatus), deref(cs.LifecycleStage)
	var out []string
	// K2: Completed tetapi actual_go_live_date kosong (bukti go-live belum lengkap).
	if status == onboardingCompleted && !cs.ActualGoLiveDate.Valid {
		out = append(out, warnOnboardingCompletedNoGoLive)
	}
	// K4: masih tahap Onboarding tetapi onboarding sudah Completed (borderline —
	// mungkin transisi tahap belum di-set; cukup nudge, bukan blok).
	if stage == lifecycleOnboarding && status == onboardingCompleted {
		out = append(out, warnOnboardingCompletedStillOnboarding)
	}
	return out
}
