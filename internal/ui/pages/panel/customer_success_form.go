package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

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
	ScoreTrend      string

	LifecycleStage string
	StageEntryDate string

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

	CanWriteHealth   bool
	CanWriteJourney  bool
	CanWriteAdoption bool

	// HealthStatusLabel/Badge = status kesehatan TERSIMPAN sudah diformat di
	// handler (BL-24: badge read-only, bukan dropdown — status turunan skor).
	HealthStatusLabel  string
	HealthStatusBadge  string
	ScoreTrends        []string
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
		body = append(body, ui.Alert(ui.VariantDestructive, "cs-form-err", g.Text(v.Err)))
	}

	fields := []g.Node{
		ui.When(v.CanWriteHealth, formCard("Health Score",
			healthStatusReadOnly(v.HealthStatusLabel, v.HealthStatusBadge),
			field("Skor Adopsi (0–100)", "adoption_score", v.Fields.AdoptionScore, false, "number"),
			field("Skor Engagement (0–100)", "engagement_score", v.Fields.EngagementScore, false, "number"),
			field("Skor Support (0–100)", "support_score", v.Fields.SupportScore, false, "number"),
			field("Skor Sentimen (0–100)", "sentiment_score", v.Fields.SentimentScore, false, "number"),
			selectField("Tren Skor", "score_trend", v.Fields.ScoreTrend, v.ScoreTrends, false),
		)),
		ui.When(v.CanWriteJourney, formCard("Journey & Onboarding",
			selectField("Tahap Siklus Hidup", "lifecycle_stage", v.Fields.LifecycleStage, v.LifecycleStages, false),
			field("Sejak Tanggal", "stage_entry_date", v.Fields.StageEntryDate, false, "date"),
			selectField("Status Onboarding", "onboarding_status", v.Fields.OnboardingStatus, v.OnboardingStatuses, false),
			field("Tanggal Kickoff", "kickoff_date", v.Fields.KickoffDate, false, "date"),
			field("Target Go-Live", "target_go_live_date", v.Fields.TargetGoLiveDate, false, "date"),
			field("Go-Live Aktual", "actual_go_live_date", v.Fields.ActualGoLiveDate, false, "date"),
			field("Progres Onboarding (0–100)", "onboarding_progress", v.Fields.OnboardingProgress, false, "number"),
		)),
		ui.When(v.CanWriteAdoption, formCard("Product Adoption",
			field("Login Terakhir", "last_login_date", v.Fields.LastLoginDate, false, "date"),
			field("Pengguna Aktif", "active_users", v.Fields.ActiveUsers, false, "number"),
			selectField("Frekuensi Login", "login_frequency", v.Fields.LoginFrequency, v.LoginFrequencies, false),
			field("Tingkat Adopsi Fitur (%)", "feature_adoption_rate", v.Fields.FeatureAdoptionRate, false, "text"),
			textareaField("Fitur Utama Dipakai", "key_features_used", v.Fields.KeyFeaturesUsed),
			selectField("Tren Penggunaan", "usage_trend", v.Fields.UsageTrend, v.UsageTrends, false),
			usageDataSourceNote(),
		)),
	}

	body = append(body, h.FormEl(
		h.Method("post"), h.Action(v.Action),
		h.Class("grid gap-4 min-w-0"),
		g.Group(fields),
		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text("Simpan")),
			h.A(h.Href(v.Base), h.Class("btn btn-ghost min-h-11"), g.Text("Batal")),
		),
	))

	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// healthStatusReadOnly — "Status Kesehatan" sebagai badge READ-ONLY, bukan
// dropdown (BL-24): status turunan overall_health_score, operator tak bisa
// menyetelnya. Tak ada <input>/<select> → tak pernah terkirim POST; label
// menjelaskan asal-nilai agar tak dikira field yang rusak. Badge selaras skor
// aktual saat baris berikutnya disimpan (label = status TERSIMPAN saat ini).
func healthStatusReadOnly(label, badge string) g.Node {
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		h.Span(h.Class("text-sm font-medium"), g.Text("Status Kesehatan")),
		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.Span(h.Class("badge "+badge), g.Text(label)),
		),
		h.P(h.Class("text-xs text-base-content/60"),
			g.Text("Otomatis dari skor kesehatan keseluruhan — tak dapat disetel manual.")),
	)
}

// usageDataSourceNote — usage_data_source TETAP "Manual" di v1 (tanpa input:
// belum ada integrasi telemetri produk, lihat komentar migrasi 00019). Teks
// statis, bukan field, agar tak menjanjikan nilai yang bisa diubah pengguna.
func usageDataSourceNote() g.Node {
	return h.Div(
		h.Class("grid gap-1 min-w-0 sm:col-span-2"),
		h.P(h.Class("text-xs text-base-content/60"),
			g.Text("Sumber Data: Manual — integrasi telemetri produk belum tersedia.")),
	)
}
