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
	// Warnings = peringatan keselarasan onboarding↔lifecycle LUNAK (BL-26 K2/K4),
	// sudah dihitung & di-gate F2-baca di handler (view murni-data). Dirender
	// sebagai banner alert-warning di atas kartu; kosong = tak ada banner.
	Warnings []string

	CanReadHealth   bool
	CanReadJourney  bool
	CanReadAdoption bool

	// BL-102: entry point kontekstual ke daftar onboarding ter-filter desa ini.
	// CanViewX = gerbang F2 (objek crm:journey, SAMA dgn Journey/Onboarding) sudah
	// dihitung handler. HrefX = URL /impl-tasks?account={id} & /trainings?account={id}
	// (dibangun handler). Tombol dirender di KEDUA cabang (Exists & empty-state):
	// navigasi tak bergantung ada/tidaknya baris CS.
	CanViewImplTasks bool
	CanViewTrainings bool
	ImplTasksHref    string
	TrainingsHref    string

	// Penugasan CS (BL-108): dipindah ke halaman ini dari form edit desa.
	// CanAssign = aktor berhak menulis kolom accounts (crm:accounts write & tak
	// read-only) — gerbang SAMA dengan aksi POST /assign, tak menambah sumbu izin
	// baru. AssignAction = URL POST /accounts/{id}/assign. Members = kandidat.
	// AssignedCSM/BackupCSM = pilihan saat ini (id sebagai string, "" = kosong).
	CanAssign    bool
	AssignAction string
	Members      []AccountMemberOption
	AssignedCSM  string
	BackupCSM    string

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
		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			ui.When(v.CanAssign, assignCSTrigger()),
			ui.When(v.CanWrite, h.A(
				h.Href(base+"/customer-success/edit"), h.Class("btn btn-sm min-h-11"),
				g.Text(editLabel))),
		),
	)

	nav := h.Div(
		h.Class("flex flex-wrap items-center justify-between gap-2"),
		h.A(h.Href(base), h.Class("text-sm text-base-content/60"),
			g.Text("« Kembali ke Desa")),
	)

	// BL-102: tautan masuk ke daftar onboarding ter-filter desa ini. Dirender di
	// KEDUA cabang render (di bawah nav) supaya konsisten & tak bergantung baris CS.
	entryLinks := csOnboardingEntryLinks(v)

	// BL-108 (revisi 9 Sep): penugasan CS lewat MODAL (tombol di header membuka
	// $assignOpen), bukan kartu inline. Modal disertakan SEKALI di KEDUA cabang —
	// assign harus tetap bisa dilakukan bahkan sebelum baris CS pernah diisi
	// (empty-state). Gerbang CanAssign dihitung di handler.
	assign := ui.When(v.CanAssign,
		assignCSModal(v.AssignAction, v.AssignedCSM, v.BackupCSM, v.Members))

	if !v.Exists {
		return h.Div(
			h.Class("grid gap-4 min-w-0"),
			header, nav, entryLinks,
			h.Div(
				h.Class("card bg-base-100 border border-base-300 min-w-0"),
				h.Div(
					h.Class("card-body min-w-0"),
					h.P(h.Class("text-base-content/70"),
						g.Text("Belum ada data Customer Success untuk desa ini.")),
				),
			),
			assign,
		)
	}

	return h.Div(
		h.Class("grid gap-4 min-w-0"),
		header, nav, entryLinks,
		onboardingWarningBanners(v.Warnings, "cs-detail-warn"),
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
		assign,
	)
}

// csOnboardingEntryLinks (BL-102) = baris tombol tautan ke daftar Implementation
// Tracker & Training Schedule ter-filter desa ini. Tiap tombol di-gate CanViewX
// (F2 crm:journey, dihitung handler). Tak ada yang berhak → g.Text("") (tak
// merender wadah kosong). flex-wrap + min-h-11: tap target ≥44px & tak meluber 375px.
func csOnboardingEntryLinks(v CustomerSuccessDetailView) g.Node {
	links := make([]g.Node, 0, 2)
	if v.CanViewImplTasks && v.ImplTasksHref != "" {
		links = append(links, h.A(
			h.Href(v.ImplTasksHref), h.Class("btn btn-sm btn-outline min-h-11"),
			g.Text("Implementation Tracker")))
	}
	if v.CanViewTrainings && v.TrainingsHref != "" {
		links = append(links, h.A(
			h.Href(v.TrainingsHref), h.Class("btn btn-sm btn-outline min-h-11"),
			g.Text("Training Schedule")))
	}
	if len(links) == 0 {
		return g.Text("")
	}
	return h.Div(h.Class("flex flex-wrap items-center gap-2 min-w-0"), g.Group(links))
}

// onboardingWarningBanners merender peringatan keselarasan onboarding↔lifecycle
// LUNAK (BL-26 K2/K4) sebagai alert-warning bertumpuk. Kosong → g.Text("") (bukan
// wadah kosong: .alert selalu punya padding/warna, lihat komentar AlertSlot).
// idPrefix membuat id tiap banner unik (detail vs form dua render berbeda di
// halaman yang sama tak boleh bertabrakan id). Murni-data: pesan sudah dihitung
// & di-gate F2 di handler (onboardingConsistencyWarnings).
func onboardingWarningBanners(warnings []string, idPrefix string) g.Node {
	if len(warnings) == 0 {
		return g.Text("")
	}
	banners := make([]g.Node, 0, len(warnings))
	for i, msg := range warnings {
		banners = append(banners,
			ui.Alert(ui.VariantWarning, idPrefix+"-"+strconv.Itoa(i), g.Text(msg)))
	}
	return h.Div(h.Class("grid gap-2 min-w-0"), g.Group(banners))
}

// CustomerSuccessForbidden — 403; menjelaskan siapa yang berhak (mirror
// AccountsForbidden), bukan sekadar "akses ditolak".
func CustomerSuccessForbidden() g.Node {
	return h.Div(
		h.Class("grid gap-4 min-w-0 max-w-xl mx-auto py-8"),
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("Customer Success")),
		h.P(h.Class("text-base-content/70"),
			g.Text("Halaman ini hanya bisa dibuka oleh pemegang peran CRM yang punya akses "+
				"ke Health Score, Journey, atau Product Adoption (admin, manajer, sales, CS, "+
				"atau support — tergantung section).")),
		h.P(h.Class("text-sm text-base-content/60"),
			g.Text("Hubungi admin workspace bila Anda perlu akses ke data ini.")),
	)
}
