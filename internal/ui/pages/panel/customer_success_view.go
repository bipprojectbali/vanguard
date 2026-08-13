package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// customer_success_view.go — halaman detail Customer Success satu desa (Modul 6
// slice B1: wireframe 6.1 Health + 6.2 Journey/Onboarding + 6.4 Adoption).
// SATU baris `customer_success`, TIGA kartu — kartu yang aktornya tak berhak
// baca disembunyikan lewat CanReadX (F2 per-section), bukan seluruh halaman
// ditolak (itu tugas CustomerSuccessForbidden, dipakai gerbang GET di handler).
// Murni-data: seluruh string sudah diformat/dihitung di handler
// (customerSuccessDetailView) — view tak memanggil authz atau menghitung apa pun.

// CustomerSuccessDetailView = seluruh data satu desa siap render. Exists=false
// → baris belum pernah dibuat (bukan galat — desanya tetap ada). CanWrite =
// tombol Sunting/Isi tampil (berhak menulis MINIMAL satu section & tak read-only).
type CustomerSuccessDetailView struct {
	Base        string
	ID          int64
	AccountName string
	CanWrite    bool
	Exists      bool

	CanReadHealth   bool
	CanReadJourney  bool
	CanReadAdoption bool

	OverallHealthScore   string
	HealthStatus         string
	AdoptionScore        string
	EngagementScore      string
	SupportScore         string
	SentimentScore       string
	ScoreTrend           string
	HealthLastCalculated string

	LifecycleStage     string
	StageEntryDate     string
	DaysInStage        string
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
	UsageDataSource     string
}

// CustomerSuccessDetail merender header (nama desa + aksi) lalu 3 kartu section,
// tiap kartu di-gate CanReadX. Empty-state ("belum ada data") menggantikan
// SELURUH bagian kartu bila !Exists — section masih relevan ditampilkan sebagai
// judul kosong-berisi CTA, bukan tiga kartu kosong terpisah yang membingungkan.
func CustomerSuccessDetail(v CustomerSuccessDetailView) g.Node {
	idStr := strconv.FormatInt(v.ID, 10)
	base := v.Base + "/accounts/" + idStr

	editLabel := "Isi Customer Success"
	if v.Exists {
		editLabel = "Sunting"
	}
	header := h.Div(
		h.Class("flex flex-wrap items-start justify-between gap-2"),
		h.Div(
			h.Class("min-w-0"),
			h.H1(h.Class("text-xl font-semibold truncate"), g.Text(v.AccountName)),
			h.P(h.Class("text-sm text-base-content/60"), g.Text("Customer Success")),
		),
		ui.When(v.CanWrite, h.A(
			h.Href(base+"/customer-success/edit"), h.Class("btn btn-sm min-h-11"),
			g.Text(editLabel))),
	)

	nav := h.Div(
		h.Class("flex flex-wrap items-center justify-between gap-2"),
		h.A(h.Href(base), h.Class("text-sm text-base-content/60"),
			g.Text("« Kembali ke Desa")),
	)

	if !v.Exists {
		return h.Div(
			h.Class("grid gap-4 min-w-0"),
			header, nav,
			h.Div(
				h.Class("card bg-base-100 border border-base-300 min-w-0"),
				h.Div(
					h.Class("card-body min-w-0"),
					h.P(h.Class("text-base-content/70"),
						g.Text("Belum ada data Customer Success untuk desa ini.")),
				),
			),
		)
	}

	return h.Div(
		h.Class("grid gap-4 min-w-0"),
		header, nav,
		ui.When(v.CanReadHealth, detailCard("Health Score", []detailField{
			{"Skor Kesehatan Keseluruhan", v.OverallHealthScore},
			{"Status Kesehatan", v.HealthStatus},
			{"Skor Adopsi", v.AdoptionScore},
			{"Skor Engagement", v.EngagementScore},
			{"Skor Support", v.SupportScore},
			{"Skor Sentimen", v.SentimentScore},
			{"Tren Skor", v.ScoreTrend},
			{"Terakhir Dihitung", v.HealthLastCalculated},
		})),
		ui.When(v.CanReadJourney, detailCard("Journey & Onboarding", []detailField{
			{"Tahap Siklus Hidup", v.LifecycleStage},
			{"Sejak Tanggal", v.StageEntryDate},
			{"Lama di Tahap Ini", v.DaysInStage},
			{"Status Onboarding", v.OnboardingStatus},
			{"Tanggal Kickoff", v.KickoffDate},
			{"Target Go-Live", v.TargetGoLiveDate},
			{"Go-Live Aktual", v.ActualGoLiveDate},
			{"Progres Onboarding", v.OnboardingProgress},
		})),
		ui.When(v.CanReadAdoption, detailCard("Product Adoption", []detailField{
			{"Login Terakhir", v.LastLoginDate},
			{"Pengguna Aktif", v.ActiveUsers},
			{"Frekuensi Login", v.LoginFrequency},
			{"Tingkat Adopsi Fitur", v.FeatureAdoptionRate},
			{"Fitur Utama Dipakai", v.KeyFeaturesUsed},
			{"Tren Penggunaan", v.UsageTrend},
			{"Sumber Data", v.UsageDataSource},
		})),
	)
}

// CustomerSuccessForbidden — 403; menjelaskan siapa yang berhak (mirror
// AccountsForbidden), bukan sekadar "akses ditolak".
func CustomerSuccessForbidden() g.Node {
	return h.Div(
		h.Class("grid gap-4 min-w-0 max-w-xl mx-auto py-8"),
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("Customer Success")),
		h.P(h.Class("text-base-content/70"),
			g.Text("Halaman ini hanya bisa dibuka oleh pemegang peran CRM yang punya akses "+
				"ke Health Score, Journey, atau Product Adoption (admin, manajer, sales, CSM, "+
				"atau support — tergantung section).")),
		h.P(h.Class("text-sm text-base-content/60"),
			g.Text("Hubungi admin workspace bila Anda perlu akses ke data ini.")),
	)
}
