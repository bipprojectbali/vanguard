package main

import (
	"log/slog"
	"net/http"

	"go_starter/internal/handler"
	"go_starter/internal/mw"

	"github.com/go-chi/chi/v5"
)

// mcpRoute membawa rute /mcp opsional ke registerRoutes tanpa memaksa routes.go
// mengimpor config/mcpserver — handler-nya dirakit di main (tempat cfg & pool
// ada). Token kosong / handler nil = rute tak didaftarkan.
type mcpRoute struct {
	Token   string
	Handler http.Handler
}

// registerRoutes mendaftarkan SEMUA route — single source of truth (§4.1).
// devMode (=!production) menentukan apakah auth password diaktifkan; di produksi
// hanya Google yang jadi jalur login.
func registerRoutes(r chi.Router, h *handler.Handler, staticFS http.Handler, log *slog.Logger, devMode bool, mcp mcpRoute) {
	// Urutan middleware penting: request-id dulu (dipakai log/recover),
	// lalu recover (tangkap panic downstream), log, security headers.
	r.Use(mw.RequestID)
	r.Use(mw.Recover(log))
	r.Use(mw.RequestLog(log))
	r.Use(mw.SecurityHeaders)

	// Health (tanpa auth)
	r.Get("/healthz", h.Liveness)
	r.Get("/readyz", h.Readiness)

	// MCP server READ-ONLY — hanya didaftarkan bila token diisi (opt-in).
	//
	// Sejajar /healthz (di luar auth SESSION): kliennya agent/program, bukan
	// manusia ber-cookie, jadi ia dijaga Bearer token sendiri — bukan RequireAuth.
	// Token kosong = rute TAK ADA sama sekali: fitur yang membuka runtime ke AI
	// tak boleh menyala karena lupa, hanya karena sengaja diisi.
	if mcp.Token != "" && mcp.Handler != nil {
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireBearer(mcp.Token))
			r.Handle("/mcp", mcp.Handler)
		})
	}

	// Static (embedded, tanpa auth)
	r.Handle("/static/*", staticFS)

	// Browser tetap auto-minta /favicon.ico di root (bookmark, tab lama) walau
	// <head> menunjuk favicon.svg — redirect permanen agar tak jadi 404 di log.
	r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/favicon.svg", http.StatusMovedPermanently)
	})

	// Publik: landing page (TIDAK redirect ke /login). Scope buka tx ber-tenant
	// bila login (no-op anonim) — RefreshIdentity/TrackPresence butuh h.q(ctx).
	// RefreshIdentity agar redirect per-role pakai role SEGAR dari DB (self-heal
	// session lama). TrackPresence: rekam kehadiran bila login (no-op anonim).
	r.With(h.Scope, h.RefreshIdentity, h.TrackPresence).Get("/", h.Home)

	// Login Google — SELALU aktif (jalur login utama di produksi). Path pakai
	// prefix /api/auth/ agar exact-match dengan redirect URI di Google Console.
	//
	// GoogleLogin ikut dijaga RequireGuest — ia jalur MASUK, dan memulainya saat
	// sudah masuk berakhir menimpa sesi aktif tanpa pernah menyebutkannya.
	// CALLBACK-nya TIDAK dijaga: ia menyelesaikan alur yang dimulai saat pengunjung
	// masih tamu, dan menolaknya di tengah jalan justru mematahkan login yang sah.
	r.With(h.RequireGuest).Get("/api/auth/google", h.GoogleLogin)
	r.Get(handler.PathGoogleCallback, h.GoogleCallback)

	// Logout SELALU tersedia, tanpa gerbang tamu: ia justru jalan keluarnya.
	r.Post("/logout", h.Logout)

	// Halaman masuk & daftar — HANYA untuk yang belum masuk (RequireGuest).
	// Tanpa gerbang itu, /login tetap terbuka bagi yang sudah login: header
	// menampilkan email orang yang SEDANG masuk sementara isinya menawarkan
	// "Masuk" dan "Belum punya akun? Daftar" — satu halaman berbicara dua hal
	// yang bertentangan, dan mengisi formnya diam-diam MENGGANTI sesi aktif.
	//
	// GET /login tetap selalu terdaftar (target redirect RequireAuth); rendernya
	// adaptif — form password hanya muncul di dev (handler.SetDevMode).
	r.Group(func(r chi.Router) {
		r.Use(h.RequireGuest)
		r.Get("/login", h.LoginPage)

		// Auth password — DEV-ONLY. Di produksi permukaan serang ini tidak ada;
		// hanya untuk mempermudah agen/dev masuk ke runtime.
		if devMode {
			r.Post("/login", h.Login)
			r.Get("/register", h.RegisterPage)
			r.Post("/register", h.Register)
		}
	})

	// Semua route terproteksi memakai urutan middleware sama:
	// RequireAuth (authn) → RefreshIdentity (role/status SEGAR dari DB, self-heal
	// session lama + enforcement real-time) → RequireEnforce (authz per resource).

	// Panel /dev — owner/developer. Hanya super_admin/root lolos "dev:users".
	r.Route("/dev", func(r chi.Router) {
		r.Use(mw.RequireAuth)
		r.Use(h.Scope) // buka tx ber-tenant (RLS) SEBELUM RefreshIdentity butuh h.q
		r.Use(h.RefreshIdentity)
		r.Use(h.TrackPresence)
		r.Use(mw.RequireEnforce("dev:users", "read"))
		// /dev telanjang → arahkan ke halaman default panel (/dev/users). Tanpa
		// ini, /dev 404 (dan HomePath super_admin menuju /dev).
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/dev/users", http.StatusSeeOther)
		})
		r.Get("/users", h.DevUsersList)
		r.Post("/users/{id}/role", h.DevUserSetRole)
		r.Post("/users/{id}/status", h.DevUserSetStatus)
		r.Post("/users/{id}/delete", h.DevUserDelete)

		// Workspace lintas-platform (0005): tangguhkan/aktifkan/pulihkan. Suspend
		// PLATFORM-ONLY dan sengaja tak punya padanan di sisi owner — kalau owner
		// bisa membatalkannya sendiri, gunanya hilang.
		//
		// Di mode single daftar ini berisi tepat satu baris — tak berguna sebagai
		// DAFTAR, tapi suspend/restore-nya tetap dibutuhkan sebagai maintenance
		// mode (satu-satunya cara menutup aplikasi sementara tanpa restart).
		r.Get("/workspaces", h.DevWorkspaces)
		r.Post("/workspaces/{id}/suspend", h.DevWorkspaceSuspend)
		r.Post("/workspaces/{id}/unsuspend", h.DevWorkspaceUnsuspend)
		r.Post("/workspaces/{id}/restore", h.DevWorkspaceRestore)

		// Identitas & siklus hidup workspace — PLATFORM-ONLY (BL-53). Ganti nama +
		// arsip/pulihkan/hapus dipindah dari /w/{slug}/settings (dulu wewenang
		// owner) ke sini: mengelola identitas workspace adalah keputusan platform,
		// sejalan dengan suspend/restore di atas. Rumah aplikasi tetap tak bisa
		// diarsipkan/dihapus (dijaga handler + SQL). Gate sama (dev:users) dengan
		// daftar di atas — bukan platform:settings, itu untuk pengaturan global.
		r.Get("/workspaces/{id}", h.DevWorkspaceDetail)
		r.Post("/workspaces/{id}/rename", h.DevWorkspaceRename)
		r.Post("/workspaces/{id}/archive", h.DevWorkspaceArchive)
		r.Post("/workspaces/{id}/unarchive", h.DevWorkspaceUnarchive)
		r.Post("/workspaces/{id}/delete", h.DevWorkspaceDelete)

		// Pengaturan platform — berlaku SEKETIKA (tabel platform_settings + cache),
		// bukan env yang butuh restart. Hak khusus per-user diatur di /dev/users
		// karena di sanalah orangnya terlihat; dua tempat mengelola hal yang sama
		// hanya bikin salah satunya basi.
		//
		// Gate SENDIRI (platform:settings), BUKAN dev:users yang menjaga grup ini:
		// aturan di sini berlaku untuk SETIAP user di platform, jadi staff — yang
		// aksesnya sengaja dibatasi — tak boleh mengubahnya. Deny-default di
		// policy.csv; hanya super_admin (root) yang lolos.
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireEnforce("platform:settings", "write"))
			r.Get("/settings", h.DevSettings)
			r.Post("/settings/quota", h.DevSettingsQuota)
			// Masa simpan jejak audit. Gate yang SAMA dengan kuota & mode: ini
			// keputusan tentang penghapusan permanen yang berlaku bagi seluruh
			// jejak platform, jadi tak boleh lebih longgar.
			r.Post("/settings/retention", h.DevSettingsRetention)
			// Kenaikan mode tenancy: single → multi, SEKALI JALAN (0007). Di-gate
			// platform:settings yang sama — ini keputusan paling fundamental di
			// halaman ini, jadi tak boleh lebih longgar dari kuota.
			r.Post("/settings/tenancy", h.DevSettingsTenancy)
			r.Post("/users/{id}/quota", h.DevUserQuota)
			r.Post("/users/{id}/quota/reset", h.DevUserQuotaReset)
		})

		// Panel aktivitas user — TERSEDIA di produksi (data-driven, super_admin
		// butuh pantau aktivitas nyata). Beda dari /health & /erd yang dev-only.
		r.Get("/logs", h.DevLogs)

		// File Health — DEV-ONLY. Butuh source .go di disk (tak ada di
		// single-binary produksi). Tak didaftarkan di prod → menu pun tak muncul.
		if devMode {
			r.Get("/health", h.DevHealth)
			r.Get("/erd", h.DevERD)
		}
	})

	// Ruang kerja — /w/{workspace}/… (keputusan 0004). SATU ruang per workspace:
	// /admin & /user dilebur karena keduanya membedakan ROLE, bukan RESOURCE —
	// halaman anggotanya sama, yang beda hanya aksi yang boleh dilakukan.
	// Beda role = beda AKSI di halaman yang sama, BUKAN beda ALAMAT (satu orang
	// bisa admin di A & member di B; alamat yang ikut berubah = tautan rusak).
	registerWorkspaceRoutes(r, h)

	// Workspace: pindah & buat baru. SELALU didaftarkan (0007) — dulu bersyarat
	// mode, dan itu sudah tak sah begitu mode bisa NAIK saat aplikasi berjalan:
	// route yang dipasang sekali saat boot akan telanjur salah sesudahnya, dan
	// gejalanya adalah 404 pada menu yang baru saja muncul.
	//
	// Yang menahannya kini middleware `mw.RequireMulti`, dibaca PER-REQUEST. Itu
	// tetap memenuhi kaidah 0006 §4 (bukan cuma menu yang disembunyikan — jalurnya
	// benar-benar tertutup), sekaligus mengikuti mode tanpa restart.
	r.Group(func(r chi.Router) {
		r.Use(mw.RequireAuth)
		r.Use(mw.RequireMulti)
		r.Use(h.Scope)
		r.Use(h.RefreshIdentity)
		r.Post("/workspace/switch", h.WorkspaceSwitch)
	})
	r.Group(func(r chi.Router) {
		r.Use(mw.RequireAuth)
		r.Use(mw.RequireMulti)
		// TANPA Scope: user tanpa workspace tak punya tenant untuk di-scope.
		// Handler pakai db.WithSuper langsung (membership belum ada).
		r.Get("/workspace/new", h.WorkspaceNewPage)
		r.Post("/workspace/new", h.WorkspaceCreate)
	})
	// Unarchive workspace kini di /dev/workspaces/{id}/unarchive (BL-53): sebagai
	// aksi platform lintas-tenant BY ID di bawah scope /dev, ia tak lagi butuh
	// jalur khusus di luar prefix /w/{slug} — masalah "gerbang read-only mengunci
	// pintu keluar dari dalam" (0005 §4) tak berlaku bagi platform yang sengaja
	// tembus gerbang siklus hidup.

	// Undangan — PUBLIK (penerima belum tentu punya akun). Tanpa Scope: penerima
	// belum jadi anggota workspace mana pun saat membuka tautan.
	r.Get("/invite/{token}", h.InvitePage)
	r.Post("/invite/{token}/accept", h.InviteAccept)

	// Notifikasi — LINTAS-PANEL (bukan milik /admin maupun /user): undangan bisa
	// datang dari workspace yang belum jadi milik user. Scope tetap dipasang agar
	// shell (switcher workspace) punya Queries ber-scope; data notifikasinya
	// sendiri dibaca via db.WithSuper karena lintas-workspace.
	r.Route("/notifications", func(r chi.Router) {
		r.Use(mw.RequireAuth)
		r.Use(h.Scope)
		r.Use(h.RefreshIdentity)
		r.Use(h.TrackPresence)
		r.Use(mw.RequireEnforce("notif:home", "read"))
		r.Get("/", h.NotificationsPage)
		r.Post("/invite/{token}/accept", h.NotificationAccept)
		r.Post("/invite/{token}/decline", h.NotificationDecline)
	})

}

// registerWorkspaceRoutes mendaftarkan grup /w/{workspace} — semua halaman yang
// cakupannya SATU workspace. Dipisah dari registerRoutes agar file tetap terbaca;
// tetap di routes.go supaya "semua route di satu tempat" tak dilanggar.
//
// Yang SENGAJA di luar grup ini (lihat 0004): /dev (platform, lintas-workspace),
// /notifications (milik user — undangan datang dari workspace yang belum jadi
// miliknya), /invite/{token} (publik), /workspace/new (belum punya workspace).
// Cakupan data menentukan bentuk path, bukan sebaliknya.
func registerWorkspaceRoutes(r chi.Router, h *handler.Handler) {
	// SATU pola untuk kedua mode (0007). Dulu mode single memakai "/app" telanjang,
	// sehingga menaikkan aplikasi ke multi mengubah setiap alamat yang sudah
	// tersebar — dan route harus didaftarkan ulang, yang berarti restart.
	r.Route(handler.WorkspacePrefix+"/{workspace}", func(r chi.Router) {
		r.Use(mw.RequireAuth)
		// Scope menerjemahkan {workspace} → tenant_id + memvalidasi keanggotaan;
		// bukan anggota → 404 (bukan 403: itu mengonfirmasi workspace-nya ada).
		r.Use(h.Scope)
		r.Use(h.RefreshIdentity)
		r.Use(h.TrackPresence)
		// Gerbang = user:home (semua anggota). Pembatasan per-aksi ada di handler
		// (canEditWorkspace/canManageMembers) — member tetap boleh MEMBUKA halaman.
		r.Use(mw.RequireEnforce("user:home", "read"))

		r.Get("/", h.WorkspaceHome)

		// Identitas & siklus hidup workspace (ganti nama, arsip/hapus/pulihkan)
		// SENGAJA TAK ADA di sini — dipindah ke /dev/workspaces/{id} sebagai
		// wewenang platform (BL-53). Yang tersisa di ruang kerja = pengaturan yang
		// memang milik pengelola workspace: Customization (format kode) di bawah,
		// Anggota, dan Peran.

		// Format kode-unik entitas (DESA-001, dst). Gerbang canEditWorkspace di
		// handler: lihat untuk semua anggota, ubah untuk pengelola.
		r.Get("/codes", h.WorkspaceCodeFormats)
		r.Post("/codes", h.WorkspaceCodeFormatUpdate)

		// Anggota (model membership). Lihat = semua anggota; ubah/keluarkan/undang
		// = owner/admin (di-guard handler via canManageMembers).
		r.Get("/members", h.MembersPage)
		r.Post("/members/{id}/role", h.MemberSetRole)
		r.Post("/members/{id}/remove", h.MemberRemove)
		r.Post("/members/invite", h.InviteCreate)
		r.Post("/members/invite/{id}/delete", h.InviteDelete)

		// Desa (Account, hub CRM Modul 2). Gerbang di HANDLER pada sumbu BISNIS
		// (CanBusiness "crm:accounts"), BUKAN role tenant: satu alamat melayani
		// semua role (0004), dan pemegang otoritas workspace (owner/super_admin)
		// TIDAK otomatis punya akses CRM. Baca = read; tulis = write. Kepemilikan
		// antar-desa (F3) menyaring baris di dalam handler/query.
		r.Get("/accounts", h.AccountsList)
		r.Get("/accounts/new", h.AccountNew)
		r.Post("/accounts", h.AccountCreate)
		r.Get("/accounts/{id}", h.AccountDetail)
		r.Get("/accounts/{id}/edit", h.AccountEdit)
		r.Post("/accounts/{id}", h.AccountUpdate)
		r.Post("/accounts/{id}/assign", h.AccountAssign)
		r.Post("/accounts/{id}/delete", h.AccountDelete)

		// Kontak (Modul 3). Nested di bawah desa induk — kepemilikan F3 DIWARISI
		// desa (bukan filter kontak sendiri): gerbangnya loadOwnedAccount atas
		// {id}. Gerbang F2 di HANDLER pada objek "crm:contacts" (BUKAN role
		// tenant): support = read, pemegang peran CRM = write. Nomor HP/WhatsApp
		// tersamar F4 (Sales saja utuh) — sejajar accounts.
		r.Get("/accounts/{id}/contacts", h.ContactsList)
		r.Get("/accounts/{id}/contacts/new", h.ContactNew)
		r.Post("/accounts/{id}/contacts", h.ContactCreate)
		r.Get("/accounts/{id}/contacts/{contactID}", h.ContactDetail)
		r.Get("/accounts/{id}/contacts/{contactID}/edit", h.ContactEdit)
		r.Post("/accounts/{id}/contacts/{contactID}", h.ContactUpdate)
		r.Post("/accounts/{id}/contacts/{contactID}/primary", h.ContactSetPrimary)
		r.Post("/accounts/{id}/contacts/{contactID}/delete", h.ContactDelete)

		// Daftar kontak LINTAS-desa (global). Cakupannya masih satu workspace
		// (RLS), tapi menembus batas per-desa; filter kepemilikan DESA INDUK di
		// query (ListContacts JOIN accounts). Alamat sibling accounts.
		//
		// BUAT dari sini: desa induk BELUM di URL → form global memuat dropdown
		// pemilih desa (hanya yang boleh ditulis aktor); account_id dari FORM
		// divalidasi ulang loadOwnedAccount (F3). Insert dibagi jalur nested.
		r.Get("/contacts", h.ContactsAll)
		r.Get("/contacts/new", h.ContactNewGlobal)
		r.Post("/contacts", h.ContactCreateGlobal)

		// Customer Success (Modul 6 slice B1): Health Score (6.1) + Journey/
		// Onboarding (6.2) + Product Adoption (6.4) — SATU baris `customer_success`
		// per desa, TIGA objek Casbin (crm:health/journey/adoption). F3 DIWARISI
		// desa induk (loadOwnedAccount), sama pola dengan Kontak. Gerbang F2
		// PER-SECTION di HANDLER (bukan di sini): baca = boleh buka bila berhak
		// MINIMAL satu section, tulis = masking per-section saat SAVE.
		r.Get("/accounts/{id}/customer-success", h.CustomerSuccessDetail)
		r.Get("/accounts/{id}/customer-success/edit", h.CustomerSuccessEdit)
		r.Post("/accounts/{id}/customer-success", h.CustomerSuccessSave)

		// Lead (Sales, CRM Modul 4). Gerbang di HANDLER sumbu BISNIS
		// (CanBusiness "crm:leads") + ownership-filter F3 (lead_owner) +
		// masking F4. Konversi = halaman review (GET) → 1 TX atomik (POST):
		// lead → Desa + Kontak + Deal sekaligus. Navigasi aksi native POST → 303.
		r.Get("/leads", h.LeadsList)
		r.Get("/leads/new", h.LeadNew)
		r.Post("/leads", h.LeadCreate)
		r.Get("/leads/{id}", h.LeadDetail)
		r.Get("/leads/{id}/edit", h.LeadEdit)
		r.Post("/leads/{id}", h.LeadUpdate)
		r.Post("/leads/{id}/delete", h.LeadDelete)
		r.Get("/leads/{id}/convert", h.LeadConvertPage)
		r.Post("/leads/{id}/convert", h.LeadConvert)

		// Deal (Sales, CRM Modul 4). Sumbu BISNIS "crm:deals" + ownership
		// (deal_owner) + masking ARR F4. Pipeline Kanban statis + toggle Tabel;
		// ganti stage = aksi tersendiri (native POST), BUKAN drag-drop.
		r.Get("/deals", h.DealsList)
		r.Get("/deals/new", h.DealNew)
		r.Post("/deals", h.DealCreate)
		r.Get("/deals/{id}", h.DealDetail)
		r.Get("/deals/{id}/edit", h.DealEdit)
		r.Post("/deals/{id}", h.DealUpdate)
		r.Post("/deals/{id}/stage", h.DealStage)
		r.Post("/deals/{id}/delete", h.DealDelete)

		// Sales Activity Log (Sales, CRM 4.4). VIEW TERFILTER (activity_context=
		// 'sales') atas tabel polimorfik `activities` (Modul 7). Gerbang di HANDLER
		// sumbu BISNIS ("crm:sales_activity") + ownership F3 (owner_id). Target
		// polimorfik (deal/account/contact) diverifikasi dalam cakupan aktor sebelum
		// insert. Ubah status = aksi tersendiri (native POST → 303).
		r.Get("/activities", h.ActivitiesList)
		r.Get("/activities/new", h.ActivityNew)
		r.Post("/activities", h.ActivityCreate)
		r.Get("/activities/{id}", h.ActivityDetail)
		r.Get("/activities/{id}/edit", h.ActivityEdit)
		r.Post("/activities/{id}", h.ActivityUpdate)
		r.Post("/activities/{id}/status", h.ActivityStatus)
		r.Post("/activities/{id}/delete", h.ActivityDelete)

		// Daftar aktivitas LINTAS-CONTEXT (menu "Activities" top-level, M7). Reuse
		// gate canViewSalesActivity; query ListAllActivities (tanpa context_filter).
		// TODO(activities): ganti ke crm:activities saat permission pass M9.
		r.Get("/activity-log", h.AllActivitiesList)

		// Daftar quote LINTAS-deal (menu sidebar "Quotes"). Read-only: ownership F3
		// DIWARISI dari deal induk (ListQuotes JOIN deals → deal_owner), keyset.
		// Pembuatan/penyuntingan tetap NEST di bawah deal (blok di bawah).
		r.Get("/quotes", h.QuotesIndex)

		// Quote (Sales, CRM Modul 4). Ter-NEST di bawah deal — quote hidup di
		// bawah deal ({id}) & MEWARISI sumbu bisnis "crm:deals" + ownership deal
		// (loadOwnedQuote → loadOwnedDeal). Builder = tabel line item ber-SNAPSHOT
		// harga (unit_price beku dari plan). Semua aksi native POST → 303. Item
		// ter-nest lagi di bawah quote ({quoteID}).
		r.Get("/deals/{id}/quotes", h.QuotesList)
		r.Get("/deals/{id}/quotes/new", h.QuoteNew)
		r.Post("/deals/{id}/quotes", h.QuoteCreate)
		r.Get("/deals/{id}/quotes/{quoteID}", h.QuoteDetail)
		r.Get("/deals/{id}/quotes/{quoteID}/edit", h.QuoteEdit)
		r.Post("/deals/{id}/quotes/{quoteID}", h.QuoteUpdate)
		r.Post("/deals/{id}/quotes/{quoteID}/status", h.QuoteStatus)
		r.Post("/deals/{id}/quotes/{quoteID}/tax", h.QuoteTax)
		r.Post("/deals/{id}/quotes/{quoteID}/delete", h.QuoteDelete)
		r.Post("/deals/{id}/quotes/{quoteID}/items", h.QuoteItemAdd)
		r.Post("/deals/{id}/quotes/{quoteID}/items/{itemID}", h.QuoteItemUpdate)
		r.Post("/deals/{id}/quotes/{quoteID}/items/{itemID}/delete", h.QuoteItemDelete)

		// Katalog Plans & Pricing (Subscriptions, CRM Modul 5). Master data milik
		// WORKSPACE (sumbu BISNIS "crm:plans" read/write; tulis = admin) — TANPA F3
		// (RLS satu-satunya pengurung). Pensiun/aktifkan = aksi status tersendiri
		// (bukan efek edit). Semua aksi native POST → 303.
		r.Get("/plans", h.PlansList)
		r.Get("/plans/new", h.PlanNew)
		r.Post("/plans", h.PlanCreate)
		r.Get("/plans/{id}/edit", h.PlanEdit)
		r.Post("/plans/{id}", h.PlanUpdate)
		r.Post("/plans/{id}/retire", h.PlanRetire)
		r.Post("/plans/{id}/activate", h.PlanActivate)

		// Active Subscriptions (CRM Modul 5, M5-3b GET-only). Milik WORKSPACE
		// (sumbu BISNIS "crm:subscriptions" read) DENGAN F3 ownership
		// (subscription_owner) di layer query + F4 masking ARR untuk non-manager.
		// Daftar berkeyset + detail dgn riwayat rantai renewal. Renew/churn menyusul.
		r.Get("/subscriptions", h.SubscriptionsList)
		// Dasbor Renewals (Menu 5.2, READ-ONLY). Rute statik SEBELUM "/{id}" — chi
		// memprioritaskan segmen statik, tapi ditaruh eksplisit agar niatnya jelas.
		// Gerbang sama: crm:subscriptions read + F3 ownership. Aksi perpanjangan
		// tetap di detail langganan.
		r.Get("/subscriptions/renewals", h.SubscriptionRenewals)
		// Dasbor Churn (Menu 5.2/5.4, READ-ONLY). Rute statik SEBELUM "/{id}" (sama
		// alasan dgn renewals). Gerbang sama: crm:subscriptions read + F3 ownership.
		r.Get("/subscriptions/churn", h.SubscriptionChurnList)
		r.Get("/subscriptions/{id}", h.SubscriptionDetail)
		// Mutasi langganan (M5-3c), native POST → 303 (gotcha #16). Gerbang bisnis
		// terpisah: renew (crm:renewals write), approve/reject (crm:renewal_mgmt
		// approve — hanya manager/admin), churn (crm:churn write). F3 ownership
		// ditegakkan per-baris di handler (loadOwnedSubscription).
		r.Post("/subscriptions/{id}/renew", h.SubscriptionRenew)
		r.Post("/subscriptions/{id}/approve", h.SubscriptionRenewApprove)
		r.Post("/subscriptions/{id}/reject", h.SubscriptionRenewReject)
		r.Post("/subscriptions/{id}/churn", h.SubscriptionChurn)

		// Preset Report read-only (CRM Modul 8, M8-1). Milik WORKSPACE (sumbu
		// BISNIS "crm:reports" read) DENGAN F3 ownership (sumber SAMA dgn
		// modul asal: deals → DealsListFilter). Export CSV route TERPISAH
		// (bukan ?format=csv) — handler tetap kecil, data via helper privat.
		r.Get("/reports/sales", h.ReportsSales)
		r.Get("/reports/sales/export", h.ReportsSalesExport)
		// Customer Success Report (8.2): Health/Adoption breakdown per status
		// (customer_success). NPS/CSAT ditunda — tabel survei belum ada
		// (lihat doc comment reports_cs.go).
		r.Get("/reports/customer-success", h.ReportsCS)
		r.Get("/reports/customer-success/export", h.ReportsCSExport)
		// Support Report (8.3): ticket volume/SLA/resolution breakdown per
		// status (tickets + sla_policies).
		r.Get("/reports/support", h.ReportsSupport)
		r.Get("/reports/support/export", h.ReportsSupportExport)
		// Subscription Report (M8-1 slice 3): tab renewal-forecast (jendela
		// TETAP "due") + churn, ?section= (gotcha #16). Export loop SEMUA
		// halaman keyset (rule "no silent caps").
		r.Get("/reports/subscriptions", h.ReportsSubscriptions)
		r.Get("/reports/subscriptions/export", h.ReportsSubscriptionsExport)

		// Kebijakan SLA (Customer Success, CRM Modul 6 slice A1). Master data milik
		// WORKSPACE (sumbu BISNIS "crm:sla" read/write; tulis = manager/admin) —
		// TANPA F3 (RLS satu-satunya pengurung, pola sama dgn Plans). Target respon/
		// selesai satuan MENIT (migrasi 00016). Pensiun/aktifkan = aksi status
		// tersendiri (bukan efek edit). Semua aksi native POST → 303.
		r.Get("/sla-policies", h.SLAPoliciesList)
		r.Get("/sla-policies/new", h.SLAPolicyNew)
		r.Post("/sla-policies", h.SLAPolicyCreate)
		r.Get("/sla-policies/{id}/edit", h.SLAPolicyEdit)
		r.Post("/sla-policies/{id}", h.SLAPolicyUpdate)
		r.Post("/sla-policies/{id}/retire", h.SLAPolicyRetire)
		r.Post("/sla-policies/{id}/activate", h.SLAPolicyActivate)

		// Playbooks (Customer Success, CRM Modul 6 slice A2). Master data milik
		// WORKSPACE (sumbu BISNIS "crm:playbooks" read/write; tulis = manager/csm/
		// admin) — TANPA F3 (RLS satu-satunya pengurung, pola sama dgn SLA
		// Policies). Jadikan-draf/aktifkan = aksi status tersendiri (bukan efek
		// edit). Semua aksi native POST → 303.
		r.Get("/playbooks", h.PlaybooksList)
		r.Get("/playbooks/new", h.PlaybookNew)
		r.Post("/playbooks", h.PlaybookCreate)
		r.Get("/playbooks/{id}/edit", h.PlaybookEdit)
		r.Post("/playbooks/{id}", h.PlaybookUpdate)
		r.Post("/playbooks/{id}/draft", h.PlaybookDraft)
		r.Post("/playbooks/{id}/activate", h.PlaybookActivate)

		// Knowledge Base (Customer Success, CRM Modul 6 slice A3). Master data
		// milik WORKSPACE (sumbu BISNIS "crm:kb" read/write; tulis =
		// admin/manager/support) — TANPA F3 (RLS satu-satunya pengurung, pola
		// sama dgn Playbooks). Transisi status (submit-review/publish/return-
		// to-draft) = aksi tersendiri (bukan efek edit). Semua aksi native
		// POST → 303.
		r.Get("/kb-articles", h.KBArticlesList)
		r.Get("/kb-articles/new", h.KBArticleNew)
		r.Post("/kb-articles", h.KBArticleCreate)
		r.Get("/kb-articles/{id}/edit", h.KBArticleEdit)
		r.Post("/kb-articles/{id}", h.KBArticleUpdate)
		r.Post("/kb-articles/{id}/publish", h.KBArticlePublish)
		r.Post("/kb-articles/{id}/return-to-draft", h.KBArticleReturnToDraft)
		r.Post("/kb-articles/{id}/archive", h.KBArticleArchive)
		r.Post("/kb-articles/{id}/unarchive", h.KBArticleUnarchive)

		// Health Score (Customer Success, CRM Modul 6 slice C1, wireframe 6.1).
		// Workspace-level listing health score semua desa. Data dari customer_success
		// (1:1 accounts, B1). F3 ownership pola AccountsListFilter (scope_all/is_csm/
		// is_sales). Gate di handler (canViewHealthScore = crm:health read).
		r.Get("/health-scores", h.HealthScoreList)

		// Success Plans (Customer Success, CRM Modul 6 slice 6.3, wireframe 6.2).
		// Plan sukses per-desa: nama, objektif, metrik, target, status, progres %.
		// F2 gerbang di HANDLER (canViewSuccessPlans/canWriteSuccessPlans);
		// "crm:success_plans" read/write = Admin/Manager/CSM.
		// F3 ownership lewat SuccessPlansListFilterFor (ownership.go): ScopeAll (Admin/Manager)
		// atau ScopeOwn (CSM — desa binaan atau plan milik CSM itu). PRG: POST → 303 (gotcha #16).
		r.Get("/success-plans", h.SuccessPlansList)
		r.Get("/success-plans/new", h.SuccessPlanNew)
		r.Post("/success-plans", h.SuccessPlanCreate)
		r.Get("/success-plans/{id}/edit", h.SuccessPlanEdit)
		r.Post("/success-plans/{id}", h.SuccessPlanUpdate)

		// Engagements / Check-ins (Customer Success, CRM Modul 6 slice 6.5).
		// F2 gerbang di HANDLER (canViewEngagements/canWriteEngagements);
		// "crm:engagements" read/write = Admin/Manager/CSM. F3 ownership lewat
		// EngagementsListFilterFor (ownership.go): ScopeAll (Admin/Manager) atau
		// ScopeOwn (CSM, desa binarannya). Semua aksi native POST → 303 (gotcha #16).
		r.Get("/engagements", h.EngagementsList)
		r.Get("/engagements/new", h.EngagementNew)
		r.Post("/engagements", h.EngagementCreate)
		r.Post("/engagements/{id}/status", h.EngagementUpdateStatus)

		// Implementation Tasks & Training Schedule (Customer Success, CRM Modul 6,
		// sub-item Onboarding 6.2.1.1/6.2.1.2). F2 REUSE "crm:journey" (Journey/
		// Onboarding) — bukan objek Casbin baru, sub-fitur onboarding yang sama.
		r.Get("/impl-tasks", h.CSImplTasksList)
		r.Get("/impl-tasks/new", h.CSImplTaskNew)
		r.Post("/impl-tasks", h.CSImplTaskCreate)
		r.Post("/impl-tasks/{id}/status", h.CSImplTaskUpdateStatus)

		r.Get("/trainings", h.CSTrainingsList)
		r.Get("/trainings/new", h.CSTrainingNew)
		r.Post("/trainings", h.CSTrainingCreate)
		r.Post("/trainings/{id}/status", h.CSTrainingUpdateStatus)

		// Renewal Management — AKSI CS pada langganan (Modul 6 slice 6.6).
		// "Renewal Dua-Rumah": DATA renewal tetap di subscriptions (Modul 5.2);
		// halaman ini MENAMPILKAN data tersebut dan MENULIS field aksi CS
		// (renewal_stage/risk/action_plan/next_action_date/owner).
		// F2 gerbang di HANDLER (canViewCSRenewals/canWriteCSRenewalsPerm);
		// "crm:renewal_mgmt" read = Admin/Manager/CSM/Sales, write = Admin/Manager/CSM.
		// F3 ownership lewat CSRenewalsListFilterFor (ownership.go): ScopeAll atau
		// ScopeOwn (assigned_csm/backup_csm/account_owner). PRG: POST → 303 (gotcha #16).
		r.Get("/renewal-management", h.CSRenewalsList)
		r.Get("/renewal-management/{id}/edit", h.CSRenewalEdit)
		r.Post("/renewal-management/{id}", h.CSRenewalUpdate)

		// Tickets / Cases (Customer Success, CRM Modul 6 slice B2, wireframe 6.9).
		// F2 gerbang di HANDLER (canViewTickets/canWriteTickets); "crm:tickets"
		// read = semua peran CRM, write = Support/Manager/Admin. F3 ownership lewat
		// TicketsListFilterFor (ownership.go); Support override ScopeNone→ScopeAll
		// saat canWrite=true. SLA deadline di-snapshot saat create (bukan JOIN live).
		// Semua aksi native POST → 303 (gotcha #16).
		r.Get("/tickets", h.TicketsList)
		r.Get("/tickets/new", h.TicketNew)
		r.Post("/tickets", h.TicketCreate)
		r.Post("/tickets/{id}/status", h.TicketUpdateStatus)

		// Peran CRM per-workspace (sumbu BISNIS, objek "crm:roles"). Gerbang di
		// HANDLER (canManageRoles), BUKAN role tenant: satu alamat melayani semua
		// role, izin yang membedakan. Editor matriks izin per-modul + cakupan data
		// F3. {name} = identitas mesin peran (subject Casbin), bukan angka.
		r.Get("/roles", h.RolesPage)
		r.Post("/roles", h.RoleCreate)
		r.Get("/roles/{name}", h.RoleEditPage)
		r.Post("/roles/{name}", h.RoleUpdate)
		r.Post("/roles/{name}/delete", h.RoleDelete)
	})
}
