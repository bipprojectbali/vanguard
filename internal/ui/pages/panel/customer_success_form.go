package panel

import (
	"strings"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// Status onboarding yang menampilkan field "Progres Onboarding" (BL-26 (c)):
// hanya "In Progress"/"Stalled" yang progresnya bermakna sebagai input operator
// (Not Started→0 & Completed→100 dinormalkan backend, tak perlu diketik). Cermin
// nilai enum di customer_success_helpers.go; dipakai membangun ekspresi data-show.
const (
	onbInProgress = "In Progress"
	onbStalled    = "Stalled"
	onbCompleted  = "Completed"
)

// Ekspresi data-show field progres: tampil saat status onboarding "In Progress"
// ATAU "Stalled". Dirakit dari const enum (bukan literal terpisah) agar satu
// perubahan nilai enum tak menyisakan ekspresi klien basi.
const onboardingProgressShowExpr = "$onbstatus == '" + onbInProgress + "' || $onbstatus == '" + onbStalled + "'"

// lifecycleOnboardingStage — nilai "Onboarding" pada Tahap Siklus Hidup. Cermin
// lifecycleOnboarding (handler/customer_success_consistency.go).
const lifecycleOnboardingStage = "Onboarding"

// postOnboardingLifecycleStages — opsi Tahap Siklus Hidup DI LUAR "Onboarding".
// Cermin postOnboardingStages (handler/customer_success_consistency.go, dipakai
// guard K1 & D1 backend) — di sini dipakai MENGUNCI opsi ini di dropdown selama
// onboarding belum "Completed" (jaring klien; K1/checkOnboardingLifecycleConsistency
// tetap penegak sesungguhnya bila jaring ini bocor/di-bypass).
var postOnboardingLifecycleStages = []string{"Adoption", "Retention", "Renewal", "Advocacy"}

func isPostOnboardingStage(s string) bool {
	for _, p := range postOnboardingLifecycleStages {
		if s == p {
			return true
		}
	}
	return false
}

// orEqExpr merakit ekspresi data-* "$sig == 'a' || $sig == 'b' || …" dari daftar
// nilai — dipakai membangun kondisi kunci dropdown dari SATU sumber enum
// (postOnboardingLifecycleStages), bukan literal terpisah yang bisa basi.
func orEqExpr(sig string, vals []string) string {
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = "$" + sig + " == '" + v + "'"
	}
	return strings.Join(parts, " || ")
}

// lifecycleIsPostOnboardingExpr — true saat $lifecycle salah satu tahap DI LUAR
// Onboarding. Dipakai mengunci Status Onboarding ke "Completed" saja (BL-178.1):
// begitu operator memilih tahap siklus hidup lanjut, status onboarding lain
// selain "Completed" tak masuk akal lagi (K1 mewajibkan Completed utk tahap itu).
var lifecycleIsPostOnboardingExpr = orEqExpr("lifecycle", postOnboardingLifecycleStages)

// lifecycleLockExpr — true selama onboarding BELUM "Completed". Dipakai mengunci
// opsi Tahap Siklus Hidup di luar Onboarding (BL-178.2): tak bisa maju tahap
// sebelum onboarding kelar (cermin K1: tahap post-onboarding mewajibkan
// onboarding_status=="Completed"). progress ikut terkunci transitif — begitu
// status terkunci "Completed", field Progres Onboarding otomatis SEMBUNYI
// (onboardingProgressShowExpr hanya tampil utk In Progress/Stalled), tak perlu
// mekanisme disable terpisah.
const lifecycleLockExpr = "$onbstatus != '" + onbCompleted + "'"

// customer_success_form.go — form sunting Customer Success (satu baris, tiga
// section F2). Form NATIVE POST → 303 (gotcha #16). Validasi sesungguhnya di
// backend (parseCustomerSuccessForm); atribut di sini hanya jaring klien.
// Masking F2 per-section terjadi di handler saat SAVE — di sini kartu yang tak
// berhak ditulis SEMBUNYI SELURUHNYA (bukan disabled), karena satu form POST
// tunggal mengirim ketiga section sekaligus dan field yang tak dirender tak
// pernah terkirim, sehingga handler tinggal mempertahankan nilai lama.

// CustomerSuccessFormFields = nilai prefill (edit existing) atau kosong (baris
// belum ada = jalur create). Semua string agar view netral terhadap tipe DB.
type CustomerSuccessFormFields struct {
	AdoptionScore   string
	EngagementScore string
	SupportScore    string
	SentimentScore  string

	LifecycleStage string

	OnboardingStatus   string
	KickoffDate        string
	TargetGoLiveDate   string
	ActualGoLiveDate   string
	OnboardingProgress string

	LastLoginDate       string
	ActiveUsers         string
	LoginFrequency      string
	FeatureAdoptionRate string
	KeyFeaturesUsed     string
	UsageTrend          string
}

// CustomerSuccessFormView = data halaman form. CanWriteHealth/Journey/Adoption
// menentukan kartu section mana yang dirender (forward-compat: bila kelak ada
// role yang boleh menulis sebagian section saja, form otomatis menyesuaikan
// tanpa perubahan view).
type CustomerSuccessFormView struct {
	Base        string
	AccountName string
	Action      string
	Err         string
	Fields      CustomerSuccessFormFields
	// Warnings = peringatan keselarasan onboarding↔lifecycle LUNAK (BL-26 K2/K4),
	// sudah dihitung & di-gate F2-tulis Journey di handler (view murni-data);
	// dirender banner alert-warning di atas form. Kosong = tak ada banner.
	Warnings []string

	CanWriteHealth   bool
	CanWriteJourney  bool
	CanWriteAdoption bool

	// IsTelemetrySourced (BL-27) = baris existing usage_data_source ==
	// "Product Telemetry" (dari tombol "Sinkron dari Desa+" di halaman detail,
	// bukan section-write biasa). Mengunci 3 field yang punya padanan API
	// (last_login_date/active_users/key_features_used) jadi READ-ONLY di form
	// ini — field manual (login_frequency/feature_adoption_rate/usage_trend)
	// TETAP bisa diedit. Penjaga sesungguhnya di backend (applyCustomerSuccess
	// Masking, customer_success_mask.go); flag ini murni tampilan.
	IsTelemetrySourced bool

	// Status Kesehatan (BL-24) sengaja TAK dirender di form sunting: tetap turunan
	// overall_health_score & tampil di halaman DETAIL, tapi bukan bagian form edit.
	// Tren Skor (BL-25) juga TAK dirender di form (BL-128 (b)) — alasan sama.
	// Sejak Tanggal/stage_entry_date (BL-178 Rule E) juga TAK dirender di form
	// sunting sejak sini — murni turunan PERUBAHAN lifecycle_stage (di-set server),
	// bukan input operator; tetap tampil read-only di halaman DETAIL.
	LifecycleStages    []string
	OnboardingStatuses []string
	LoginFrequencies   []string
	UsageTrends        []string
}

// CustomerSuccessForm merender halaman form: header, alert galat (bila ada),
// lalu SATU form berisi kartu-kartu section yang berhak ditulis aktor.
func CustomerSuccessForm(v CustomerSuccessFormView) g.Node {
	body := []g.Node{
		h.Div(
			h.H1(h.Class("text-xl font-semibold"), g.Text("Sunting Customer Success")),
			h.P(h.Class("text-sm text-base-content/60"), g.Text(v.AccountName)),
			h.A(h.Href(v.Base), h.Class("text-sm text-base-content/60"),
				g.Text("« Kembali ke Desa")),
		),
	}
	if v.Err != "" {
		body = append(body, ui.Toast(ui.VariantDestructive, "cs-form-err", g.Text(v.Err)))
	}
	body = append(body, onboardingWarningBanners(v.Warnings, "cs-form-warn"))

	fields := []g.Node{
		ui.When(v.CanWriteHealth, formCard("Health Score",
			field("Skor Adopsi (0–100)", "adoption_score", v.Fields.AdoptionScore, false, "number"),
			field("Skor Engagement (0–100)", "engagement_score", v.Fields.EngagementScore, false, "number"),
			field("Skor Support (0–100)", "support_score", v.Fields.SupportScore, false, "number"),
			field("Skor Sentimen (0–100)", "sentiment_score", v.Fields.SentimentScore, false, "number"),
		)),
		ui.When(v.CanWriteJourney, formCard("Journey & Onboarding",
			lifecycleStageSelect(v.Fields.LifecycleStage, v.LifecycleStages),
			onboardingStatusSelect(v.Fields.OnboardingStatus, v.OnboardingStatuses),
			field("Tanggal Kickoff", "kickoff_date", v.Fields.KickoffDate, false, "date"),
			field("Target Go-Live", "target_go_live_date", v.Fields.TargetGoLiveDate, false, "date"),
			field("Go-Live Aktual", "actual_go_live_date", v.Fields.ActualGoLiveDate, false, "date"),
			// Progres onboarding HANYA saat status "In Progress"/"Stalled" (BL-26
			// (c)): terminal (Not Started→0, Completed→100) dinormalkan backend, jadi
			// ketikan operator tak relevan → sembunyikan agar tak menyesatkan.
			// data-show = jaring UX klien; backend (normalizeOnboardingProgress)
			// tetap penegak sesungguhnya walau field basi terkirim.
			showWhen(onboardingProgressShowExpr, "min-w-0",
				field("Progres Onboarding (0–100)", "onboarding_progress", v.Fields.OnboardingProgress, false, "number")),
		)),
		ui.When(v.CanWriteAdoption, formCard("Product Adoption",
			adoptionUsageField(v.IsTelemetrySourced, "Aktivitas Terakhir", "last_login_date", v.Fields.LastLoginDate, "date"),
			adoptionUsageField(v.IsTelemetrySourced, "Pengguna Aktif", "active_users", v.Fields.ActiveUsers, "number"),
			selectField("Frekuensi Login", "login_frequency", v.Fields.LoginFrequency, v.LoginFrequencies, false),
			field("Tingkat Adopsi Fitur (%)", "feature_adoption_rate", v.Fields.FeatureAdoptionRate, false, "text"),
			adoptionUsageTextareaField(v.IsTelemetrySourced, "Fitur Utama Dipakai", "key_features_used", v.Fields.KeyFeaturesUsed),
			selectField("Tren Penggunaan", "usage_trend", v.Fields.UsageTrend, v.UsageTrends, false),
			usageDataSourceNote(v.IsTelemetrySourced),
		)),
	}

	// Signal ephemeral (state form, tak dikirim ke server), diinisialisasi dari
	// nilai TERSIMPAN agar no-FOUC saat prefill:
	//   - $onbstatus → tampil/sembunyi field progres (onboardingStatusSelect + showWhen);
	//     JUGA mengunci opsi Tahap Siklus Hidup di luar Onboarding (lifecycleLockExpr).
	//   - $lifecycle → mengunci opsi Status Onboarding selain "Completed" begitu
	//     tahap siklus hidup lanjut dipilih (lifecycleIsPostOnboardingExpr).
	body = append(body, h.FormEl(
		h.Method("post"), h.Action(v.Action),
		h.Class("grid gap-4 min-w-0"),
		data.Signals(map[string]any{
			"onbstatus": v.Fields.OnboardingStatus,
			"lifecycle": v.Fields.LifecycleStage,
		}),
		g.Group(fields),
		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text("Simpan")),
			h.A(h.Href(v.Base), h.Class("btn btn-ghost min-h-11"), g.Text("Batal")),
		),
	))

	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// onboardingStatusSelect — dropdown "Status Onboarding" yang di-bind ke signal
// $onbstatus (data.Bind) sehingga memilih nilai men-toggle field "Progres
// Onboarding" tanpa round-trip (BL-26 (c)). Selain binding, identik selectField
// opsional (opsi kosong "—" di depan) — KECUALI opsi selain "Completed" dikunci
// reaktif (data-attr:disabled) saat $lifecycle sudah tahap post-onboarding
// (BL-178.1): tak masuk akal mundur/ubah status onboarding setelah siklus hidup
// lanjut — K1 backend tetap penegak sesungguhnya, ini jaring klien. Select TETAP
// aktif (bukan seluruh <select> disabled) agar nilai "Completed" yang sudah
// terpilih tetap TERKIRIM saat submit — <select disabled> tak pernah ikut
// form-data, akan salah kena Guard C "regresi" (checkOnboardingRegression).
// Dibuat manual (bukan enumOptions) karena butuh atribut Datastar per-opsi.
func onboardingStatusSelect(current string, opts []string) g.Node {
	sel := []g.Node{
		h.ID("f-onboarding_status"), h.Name("onboarding_status"),
		data.Bind("onbstatus"),
		h.Class("select text-base w-full"),
	}
	nodes := []g.Node{
		h.Option(h.Value(""), data.Attr("disabled", lifecycleIsPostOnboardingExpr), g.Text("—")),
	}
	for _, o := range opts {
		attrs := []g.Node{h.Value(o)}
		if o == current {
			attrs = append(attrs, h.Selected())
		}
		if o != onbCompleted {
			attrs = append(attrs, data.Attr("disabled", lifecycleIsPostOnboardingExpr))
		}
		nodes = append(nodes, h.Option(append(attrs, g.Text(o))...))
	}
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		labelFor("Status Onboarding", "f-onboarding_status", false),
		h.Select(append(sel, g.Group(nodes))...),
	)
}

// lifecycleStageSelect — dropdown "Tahap Siklus Hidup" yang di-bind ke signal
// $lifecycle (data.Bind) agar onboardingStatusSelect bisa bereaksi (BL-178.1).
// Opsi post-onboarding (postOnboardingLifecycleStages) dikunci reaktif
// (data-attr:disabled) selama onboarding BELUM "Completed" (BL-178.2,
// lifecycleLockExpr) — cermin K1 backend, jaring klien saja. Opsi "Onboarding"
// & "—" TETAP selalu aktif (scope permintaan: hanya opsi "di luar Onboarding").
// Dibuat manual (bukan selectField) karena butuh binding + atribut per-opsi.
func lifecycleStageSelect(current string, opts []string) g.Node {
	nodes := []g.Node{h.Option(h.Value(""), g.Text("—"))}
	for _, o := range opts {
		attrs := []g.Node{h.Value(o)}
		if o == current {
			attrs = append(attrs, h.Selected())
		}
		if isPostOnboardingStage(o) {
			attrs = append(attrs, data.Attr("disabled", lifecycleLockExpr))
		}
		nodes = append(nodes, h.Option(append(attrs, g.Text(o))...))
	}
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		labelFor("Tahap Siklus Hidup", "f-lifecycle_stage", false),
		h.Select(
			h.ID("f-lifecycle_stage"), h.Name("lifecycle_stage"),
			data.Bind("lifecycle"),
			h.Class("select text-base w-full"),
			g.Group(nodes),
		),
	)
}

// usageDataSourceNote — catatan kecil arti "Sumber Data" (bukan field, agar
// tak menjanjikan nilai yang bisa diubah pengguna dari form ini). BL-27:
// sejak sinkronisasi Desa+ ada, usage_data_source bisa "Product Telemetry" —
// teks menyesuaikan & menjelaskan CAKUPAN PARSIAL (hanya 3 dari 6 field),
// biar operator tak mengira seluruh section Adoption otomatis. Sinkronisasi
// sendiri dipicu tombol "Sinkron dari Desa+" di halaman DETAIL, bukan di sini.
func usageDataSourceNote(isTelemetrySourced bool) g.Node {
	text := "Sumber Data: Manual."
	if isTelemetrySourced {
		text = "Sumber Data: Product Telemetry — Aktivitas Terakhir, Pengguna Aktif, dan " +
			"Fitur Utama Dipakai disinkron otomatis dari Desa+ (read-only di sini). Frekuensi " +
			"Login, Tingkat Adopsi Fitur, dan Tren Penggunaan tetap isian manual."
	}
	return h.Div(
		h.Class("grid gap-1 min-w-0 sm:col-span-2"),
		h.P(h.Class("text-xs text-base-content/60"), g.Text(text)),
	)
}

// adoptionUsageField — field() biasa, KECUALI saat isTelemetrySourced: berubah
// jadi tampilan read-only (badge) tanpa <input>, agar operator tak mengira
// nilai ini bisa diketik manual selagi berasal dari sync Desa+ (BL-27).
// Penjaga sesungguhnya tetap di backend (applyCustomerSuccessMasking,
// customer_success_mask.go) — ini murni tampilan, form klien cuma jaring UX.
func adoptionUsageField(isTelemetrySourced bool, label, name, val, typ string) g.Node {
	if !isTelemetrySourced {
		return field(label, name, val, false, typ)
	}
	return readonlyField(label, val)
}

// adoptionUsageTextareaField — padanan adoptionUsageField untuk field
// key_features_used (textarea, 2-kolom), sama alasan.
func adoptionUsageTextareaField(isTelemetrySourced bool, label, name, val string) g.Node {
	if !isTelemetrySourced {
		return textareaField(label, name, val)
	}
	return readonlyTextField(label, val)
}
