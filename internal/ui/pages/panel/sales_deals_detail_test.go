package panel

import (
	"strings"
	"testing"
)

// sales_deals_detail_test.go — regresi BL-12: stepper detail deal menampilkan 6
// langkah (Closed Won/Lost jadi SATU node terminal), dan field alasan menang/kalah
// + catatan kekalahan hanya tampil (data-show Datastar) saat tahap terminal.
// dealStageOptions (enum handler) tetap 7 — di sini kita jaga representasi visual.

// pipelineStages = tiruan dealStageOptions (7) untuk uji view tanpa impor handler.
var pipelineStages = []string{
	"Prospecting", "Qualification", "Demo", "Proposal", "Negotiation",
	"Closed Won", "Closed Lost",
}

// TestDisplayStages_SixSteps: 7 tahap pipeline → 6 langkah tampilan; dua terminal
// digabung jadi satu node.
func TestDisplayStages_SixSteps(t *testing.T) {
	got := displayStages(pipelineStages, "Demo")
	if len(got) != 6 {
		t.Fatalf("stepper harus 6 langkah, dapat %d: %v", len(got), got)
	}
	want5 := []string{"Prospecting", "Qualification", "Demo", "Proposal", "Negotiation"}
	for i, s := range want5 {
		if got[i] != s {
			t.Errorf("langkah[%d] = %q, mau %q", i, got[i], s)
		}
	}
	// Tahap masih terbuka → node ke-6 = "Ditutup" (bukan Won/Lost harfiah).
	if got[5] != "Ditutup" {
		t.Errorf("node terminal saat terbuka = %q, mau %q", got[5], "Ditutup")
	}
	// Tak boleh ada dua node terminal terpisah.
	for _, s := range got {
		if s == "Closed Won" || s == "Closed Lost" {
			t.Errorf("node terminal terpisah %q bocor ke display stages: %v", s, got)
		}
	}
}

// TestDisplayStages_TerminalLabel: node ke-6 mengikuti hasil aktual saat tertutup.
func TestDisplayStages_TerminalLabel(t *testing.T) {
	cases := map[string]string{
		"Closed Won":  "Closed Won",
		"Closed Lost": "Closed Lost",
		"Negotiation": "Ditutup",
	}
	for current, wantLast := range cases {
		got := displayStages(pipelineStages, current)
		if got[len(got)-1] != wantLast {
			t.Errorf("current=%q → node terminal %q, mau %q", current, got[len(got)-1], wantLast)
		}
	}
}

// TestDealStepper_RendersSixNodes: stepper merender tepat 6 <li> dan menandai
// node terminal "Closed Won" sebagai posisi saat ini (deal menang).
func TestDealStepper_RendersSixNodes(t *testing.T) {
	out := renderLeads(t, dealStepper(displayStages(pipelineStages, "Closed Won"), "Closed Won"))
	if n := strings.Count(out, "<li"); n != 6 {
		t.Errorf("stepper harus 6 <li>, dapat %d:\n%s", n, out)
	}
	if strings.Contains(out, "Closed Lost") {
		t.Errorf("deal menang tak boleh menampilkan node 'Closed Lost':\n%s", out)
	}
	if !strings.Contains(out, "Closed Won") {
		t.Errorf("node terminal 'Closed Won' harus tampil saat menang:\n%s", out)
	}
}

// stageControlFixture = view minimal untuk merender kontrol Ubah Tahap.
func stageControlFixture(stage string) DealDetailView {
	return DealDetailView{
		Base:   "/w/acme",
		ID:     42,
		Stage:  stage,
		Stages: pipelineStages,
	}
}

// TestDealStageControl_ConditionalFields: <select> di-bind ke signal $stage;
// "Alasan Menang/Kalah" data-show saat Won ATAU Lost; "Catatan Kekalahan" HANYA
// saat Lost. Nilai atribut ter-escape (&#39;) — browser mengembalikannya ke ' saat
// dibaca Datastar (verifikasi bentuk ter-render benar, bukan lewat helper dsx.go).
func TestDealStageControl_ConditionalFields(t *testing.T) {
	out := renderLeads(t, dealStageControl(stageControlFixture("Negotiation"), "/w/acme/deals/42"))

	for _, want := range []string{
		`data-signals=`,     // sinyal $stage diinisialisasi
		`data-bind="stage"`, // <select> menyetir $stage
		`name="win_loss_reason"`,
		`name="loss_notes"`,
		// win_loss tampil saat Won ATAU Lost
		`data-show="$stage == &#39;Closed Won&#39; || $stage == &#39;Closed Lost&#39;"`,
		// loss_notes HANYA saat Lost
		`data-show="$stage == &#39;Closed Lost&#39;"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("kontrol tahap harus memuat %q:\n%s", want, out)
		}
	}
}

// TestDealStageControl_NativePost: aksi ganti tahap tetap NATIVE POST (gotcha #16),
// bukan @post Datastar — data-bind hanya untuk toggle klien, tak menggeser submit.
func TestDealStageControl_NativePost(t *testing.T) {
	out := renderLeads(t, dealStageControl(stageControlFixture("Demo"), "/w/acme/deals/42"))
	if !strings.Contains(out, `method="post"`) || !strings.Contains(out, `action="/w/acme/deals/42/stage"`) {
		t.Errorf("form ganti tahap harus native POST ke .../stage:\n%s", out)
	}
	if strings.Contains(out, "@post") {
		t.Errorf("ganti tahap tak boleh pakai @post Datastar (gotcha #16):\n%s", out)
	}
}

// TestDealStageControl_WonSubscriptionStatus: BL-21 — saat WonSubStatuses terisi,
// kontrol tahap merender <select name="subscription_status"> yang data-show HANYA
// pada Closed Won (deal menang membuat langganan otomatis). Opsi pertama = default
// terpilih. Ini juga regresi panik "index out of range": g.Iff (bukan g.If) menjaga
// WonSubStatuses[0] tak diakses saat slice kosong.
func TestDealStageControl_WonSubscriptionStatus(t *testing.T) {
	fx := stageControlFixture("Closed Won")
	fx.WonSubStatuses = []string{"Active", "Trial"}
	out := renderLeads(t, dealStageControl(fx, "/w/acme/deals/42"))

	for _, want := range []string{
		`name="subscription_status"`,
		// tampil hanya saat Closed Won
		`data-show="$stage == &#39;Closed Won&#39;"`,
		// dua opsi status awal terisi
		`>Active<`,
		`>Trial<`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("kontrol Won harus memuat %q:\n%s", want, out)
		}
	}
}

// TestDealStageControl_NoWonSubStatuses_NoPanic: WonSubStatuses kosong → field
// status langganan TIDAK dirender dan TIDAK panik (g.Iff menunda akses [0]).
func TestDealStageControl_NoWonSubStatuses_NoPanic(t *testing.T) {
	out := renderLeads(t, dealStageControl(stageControlFixture("Closed Won"), "/w/acme/deals/42"))
	if strings.Contains(out, `name="subscription_status"`) {
		t.Errorf("tanpa WonSubStatuses, field subscription_status tak boleh dirender:\n%s", out)
	}
}

// TestDealQuotesCard_ButtonGatedByStage: BL-86 — tombol "Buat Quote" hanya tampil
// saat CanCreateQuote (boleh tulis DAN stage quotable). Simetris dengan gerbang
// backend BL-13: di luar jendela quotable tombol ABSEN meski boleh tulis, dan hint
// alasan ditampilkan agar UX tak menyesatkan.
func TestDealQuotesCard_ButtonGatedByStage(t *testing.T) {
	base := DealDetailView{Base: "/w/acme", ID: 42}

	// Stage quotable + boleh tulis → tombol ADA, tanpa hint.
	quotable := base
	quotable.CanWrite = true
	quotable.CanCreateQuote = true
	out := renderLeads(t, dealQuotesCard(quotable))
	if !strings.Contains(out, "Buat Quote") {
		t.Errorf("stage quotable + CanWrite: tombol 'Buat Quote' harus ADA:\n%s", out)
	}

	// Prospecting: boleh tulis tapi stage di luar jendela → tombol ABSEN + hint.
	locked := base
	locked.CanWrite = true
	locked.CanCreateQuote = false
	locked.QuoteStageLockMsg = "Deal masih di tahap Prospecting — quote baru dapat dibuat atau diubah mulai tahap Qualification."
	out = renderLeads(t, dealQuotesCard(locked))
	if strings.Contains(out, "Buat Quote") {
		t.Errorf("stage di luar jendela: tombol 'Buat Quote' harus ABSEN:\n%s", out)
	}
	if !strings.Contains(out, "tahap Qualification") {
		t.Errorf("stage terkunci: hint alasan harus tampil:\n%s", out)
	}

	// Tak boleh tulis → tombol ABSEN dan hint TIDAK ditampilkan (bukan urusan pembaca).
	readonly := base
	readonly.CanWrite = false
	readonly.CanCreateQuote = false
	readonly.QuoteStageLockMsg = "apa pun"
	out = renderLeads(t, dealQuotesCard(readonly))
	if strings.Contains(out, "Buat Quote") {
		t.Errorf("tak boleh tulis: tombol 'Buat Quote' harus ABSEN:\n%s", out)
	}
	if strings.Contains(out, "apa pun") {
		t.Errorf("tak boleh tulis: hint stage tak perlu ditampilkan:\n%s", out)
	}
}

// TestDealStepper_Horizontal (BL-123): kartu Tahap Pipeline penuh-lebar di atas →
// stepper horizontal (steps-horizontal) dalam kontainer overflow-x-auto agar tak
// meluber di mobile.
func TestDealStepper_Horizontal(t *testing.T) {
	out := renderLeads(t, dealStepper(displayStages(pipelineStages, "Demo"), "Demo"))
	if !strings.Contains(out, "steps-horizontal") {
		t.Errorf("stepper detail harus steps-horizontal:\n%s", out)
	}
	if !strings.Contains(out, "overflow-x-auto") {
		t.Errorf("stepper horizontal harus dalam kontainer overflow-x-auto:\n%s", out)
	}
}

// TestDealDetail_StageModalGatedByWrite (BL-123): aksi ubah tahap = modal dipicu
// tombol header. CanWrite=true → pemicu + modal (signal dealStageOpen) dirender;
// CanWrite=false → keduanya absen (pembaca tak boleh mengubah tahap).
func TestDealDetail_StageModalGatedByWrite(t *testing.T) {
	base := DealDetailView{Base: "/w/acme", ID: 42, Stage: "Negotiation", Stages: pipelineStages}

	writable := base
	writable.CanWrite = true
	out := renderLeads(t, DealDetail(writable))
	if !strings.Contains(out, "Ubah Tahap") {
		t.Errorf("CanWrite: pemicu 'Ubah Tahap' harus ADA:\n%s", out)
	}
	if !strings.Contains(out, dealStageSignal) {
		t.Errorf("CanWrite: modal tahap (signal %q) harus dirender:\n%s", dealStageSignal, out)
	}
	if !strings.Contains(out, `action="/w/acme/deals/42/stage"`) {
		t.Errorf("CanWrite: form modal harus native POST ke .../stage:\n%s", out)
	}

	readonly := base
	readonly.CanWrite = false
	out = renderLeads(t, DealDetail(readonly))
	if strings.Contains(out, "Ubah Tahap") {
		t.Errorf("read-only: pemicu/modal 'Ubah Tahap' harus ABSEN:\n%s", out)
	}
	if strings.Contains(out, dealStageSignal) {
		t.Errorf("read-only: signal modal tahap tak boleh dirender:\n%s", out)
	}
}

// TestDealDetail_TerminalHidesActions: deal terminal (Closed Won/Closed Lost) =
// terkunci → tombol Sunting/Ubah Tahap/Hapus disembunyikan (dan modal tahap tak
// dirender), meski CanWrite. Stage terbuka (mis. Negotiation) tetap menampilkan aksi.
func TestDealDetail_TerminalHidesActions(t *testing.T) {
	for _, stage := range []string{"Closed Won", "Closed Lost"} {
		term := DealDetailView{Base: "/w/acme", ID: 42, Stage: stage, Stages: pipelineStages, CanWrite: true}
		out := renderLeads(t, DealDetail(term))
		for _, absent := range []string{">Sunting<", "Ubah Tahap", ">Hapus<", dealStageSignal} {
			if strings.Contains(out, absent) {
				t.Errorf("%s: aksi %q harus ABSEN:\n%s", stage, absent, out)
			}
		}
	}

	open := DealDetailView{Base: "/w/acme", ID: 42, Stage: "Negotiation", Stages: pipelineStages, CanWrite: true}
	out := renderLeads(t, DealDetail(open))
	if !strings.Contains(out, "Ubah Tahap") || !strings.Contains(out, ">Sunting<") {
		t.Errorf("stage terbuka + CanWrite: aksi harus ADA:\n%s", out)
	}
}

// TestDealDetail_SystemAuditCard (BL-123): kartu "Sistem & Audit" merender label
// pembuat/pengubah + waktu, terisi dari field audit view.
func TestDealDetail_SystemAuditCard(t *testing.T) {
	v := DealDetailView{
		Base: "/w/acme", ID: 42, Stage: "Demo", Stages: pipelineStages,
		CreatedByName: "Budi", CreatedAt: "01 Jan 2026 10:00",
		UpdatedByName: "Sari", UpdatedAt: "02 Jan 2026 11:30",
	}
	out := renderLeads(t, DealDetail(v))
	for _, want := range []string{
		"Sistem &amp; Audit", "Dibuat Oleh", "Budi", "Tanggal Dibuat", "01 Jan 2026 10:00",
		"Diubah Oleh", "Sari", "Terakhir Diubah", "02 Jan 2026 11:30",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("kartu Sistem & Audit harus memuat %q:\n%s", want, out)
		}
	}
}

// TestDealDetail_FeedbackBanner (BL-99): halaman detail merender banner umpan
// balik PRG. Kritis untuk tahap terminal — gerbang Closed Won/Lost yang gagal
// redirect ke DETAIL dgn ?err; tanpa banner, penolakan tampak "tak tersimpan".
// Err → alert-error (deal-err); Msg → alert default (deal-ok); keduanya kosong
// → tak ada banner sama sekali.
func TestDealDetail_FeedbackBanner(t *testing.T) {
	base := DealDetailView{Base: "/w/acme", ID: 42, Stage: "Negotiation", Stages: pipelineStages}

	withErr := base
	withErr.Err = "Alasan menang/kalah wajib diisi untuk menutup deal."
	out := renderLeads(t, DealDetail(withErr))
	if !strings.Contains(out, `id="deal-err"`) || !strings.Contains(out, withErr.Err) {
		t.Errorf("Err diset → banner error (deal-err) harus tampil dgn pesannya:\n%s", out)
	}

	withMsg := base
	withMsg.Msg = "Tahap deal diperbarui."
	out = renderLeads(t, DealDetail(withMsg))
	if !strings.Contains(out, `id="deal-ok"`) || !strings.Contains(out, withMsg.Msg) {
		t.Errorf("Msg diset → banner sukses (deal-ok) harus tampil dgn pesannya:\n%s", out)
	}

	out = renderLeads(t, DealDetail(base))
	if strings.Contains(out, `id="deal-err"`) || strings.Contains(out, `id="deal-ok"`) {
		t.Errorf("Err & Msg kosong → tak ada banner umpan balik:\n%s", out)
	}
}
