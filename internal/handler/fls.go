package handler

import (
	"context"

	"go_starter/internal/fls"
	"go_starter/internal/session"
)

// fls.go — Field-Level Security (sistem-dan-role.md §5, diperkuat §8.3).
//
// Sumbu ini BEDA dari dua yang lain dan tak boleh dicampur: Casbin (F2) menjawab
// "boleh membuka modul APA", ownership (F3) menjawab "atas desa SIAPA", dan FLS
// di sini menjawab "dalam sebuah baris yang SUDAH boleh ia lihat, field mana yang
// boleh tampil". Seseorang bisa berhak membuka Accounts (Casbin) atas desanya
// (ownership) namun TETAP tak boleh melihat nomor HP kontak di dalamnya (FLS).
//
// Kenapa di HANDLER, bukan di view (sama seperti maskEmail): kalau view yang
// menyamarkan, nilai ASLI tetap harus dioper ke sana — dan satu view yang lupa
// akan mengirimnya ke browser, terbaca di source meski tak tampak di layar. Maka
// nilai sensitif disamarkan DI SINI; yang tak berhak TAK PERNAH menerimanya.
// Itu pula kriteria penerimaan F4: "field masked tak pernah dioper ke view".
//
// FLS memakai business_role (sumbu CRM), BUKAN role tenant/platform: super_admin
// atau owner yang bukan pemegang peran CRM = "" → tak lolos apa pun di sini
// (tegak lurus, §3). Naikkan otoritas CRM lewat memberships.business_role, bukan
// lewat wewenang platform.

// flsHidden = penanda universal "disembunyikan". Dipakai untuk field yang, saat
// tak berhak, tetap perlu MENGISI kolom (mis. ARR di tabel langganan) agar tata
// letak tak berubah antar-role.
const flsHidden = "•••"

// canSeeARR — Nilai Kontrak/ARR/MRR (Deal Amount, estimasi Lead, anggaran
// desa Account, MRR/ARR Subscription, quote, & turunannya di Reports) kini
// KAPABILITAS ter-matriks YANG SAMA dgn canSeeSubscriptionARR (crm:
// subscriptions/arr, BL-58) — disatukan BL-169. Sebelumnya switch atas 4 nama
// role literal (Admin/Manager/Sales/CSM), buta terhadap role kustom/rename:
// tenant yang memberi peran kustom kapabilitas setara tetap tersembunyi krn
// namanya tak dikenal switch. Dua fungsi (canSeeARR/canSeeSubscriptionARR)
// SENGAJA dipertahankan terpisah namanya walau isinya sama — kejelasan di
// titik panggil ("field apa yang sedang di-mask"), bukan dua kebijakan.
// Objek Casbin TETAP "crm:subscriptions" (bukan diganti nama umum) krn
// checkbox "Lihat ARR" di matriks editor peran sudah terikat ke situ sejak
// BL-58 — mengganti nama objek berarti migrasi data tenant existing tanpa
// manfaat tambahan; cakupannya kini memang lebih luas dari sekadar modul
// Subscriptions, checkbox itu jadi gerbang tunggal Nilai Kontrak lintas modul.
// Default grant (business_defaults.go): Admin (glob crm:*) + Manager (BL-58)
// + Sales & CSM (BL-169, supaya tenant existing tak regresi — allow-list lama
// meluluskan keduanya).
func canSeeARR(ctx context.Context) bool {
	return canSeeSubscriptionARR(ctx)
}

// canSeeFullPhone — boleh melihat nomor HP/WhatsApp kontak UTUH. Kebijakan kini
// KONFIGURABEL per-tenant (BL-107, ADR 0012): tiap workspace menentukan sendiri
// business_role mana yang lolos, disunting di halaman Settings /field-security.
// Default (tenant belum mengonfigurasi) = perilaku pra-BL-107: Sales & Admin —
// Sales pemegang hubungan yang menghubungi kepala desa, Admin pengelola workspace.
//
// Fail-CLOSED tetap dijamin di internal/fls: role platform tersasar
// (super_admin/owner/staff BUKAN business_role) & nilai liar tersembunyi, dan
// peran tanpa baris pada tenant terkonfigurasi → false. Sumbu tetap tegak lurus
// (§3) — otoritas platform TIDAK membuka field ini.
func canSeeFullPhone(ctx context.Context) bool {
	return fls.CanViewPhone(session.TenantID(ctx), session.BusinessRole(ctx))
}

// canEditPhone = boleh MENYUNTING nomor HP/WhatsApp kontak. Sumbu terpisah dari
// lihat (edit⇒view ditegakkan config): role bisa lihat-penuh tapi read-only,
// mempertahankan realita Admin=lihat-saja. KONFIGURABEL per-tenant (BL-107);
// default pra-BL-107 = Sales saja. Dipakai modul Kontak & Lead (form + konversi);
// BL-106 melepasnya dari Account, jadi rumahnya di sini bersama helper phone FLS lain.
func canEditPhone(ctx context.Context) bool {
	return fls.CanEditPhone(session.TenantID(ctx), session.BusinessRole(ctx))
}

// Catatan Internal (Activity.Notes) & Health Score SENGAJA tanpa predikat FLS:
// keduanya terbuka untuk semua business role yang sudah lolos F2 (crm:
// sales_activity) atas targetnya. Gate Admin/CSM-only utk Catatan Internal
// (canSeeInternalNotes/maskInternalNotes) DIHAPUS — terbuka untuk semua role
// yang bisa membuka aktivitasnya (dihilangkan sementara; kalau perlu
// dikembalikan, pertimbangkan lewat kapabilitas Casbin bukan switch nama role,
// pola sama BL-169 canSeeARR).
//
// Health Score SENGAJA tanpa predikat: §5 menyatakannya terbuka untuk SEMUA role
// ("justru harus dilihat bersama"). Tidak ada gate di sini adalah keputusan, bukan
// kelalaian — menambahkannya akan mengingkari maksud spec. Jangan tambahkan.

// maskARR mengembalikan nilai ARR siap-tampil: string yang SUDAH diformat bila
// berhak, atau penanda tersembunyi bila tidak. Sengaja menerima string
// terformat (bukan angka) supaya urusan format mata uang tetap di handler dan
// FLS hanya soal boleh-tampil-atau-tidak. Nilai asli tak pernah keluar saat
// tersembunyi. Menerima bool (bukan ctx/businessRole) — pola sama
// maskSubscriptionARR (BL-58) sejak canSeeARR disatukan ke kapabilitas Casbin
// (BL-169): pemanggil hitung SEKALI via canSeeARR(ctx) lalu alirkan bool ke
// row-mapper murni-data (tak boleh panggil authz/session di dalamnya, §
// "View murni-data").
func maskARR(formatted string, canSee bool) string {
	if canSee {
		return formatted
	}
	return flsHidden
}

// maskPhone menyamarkan nomor HP bagi role yang tak berhak melihat penuh (kebijakan
// per-tenant, lihat canSeeFullPhone). Penyamaran = SEMBUNYIKAN PENUH (penanda tetap),
// bukan sekadar menutup sebagian: alasan §5 adalah "membatasi sebaran PII", dan
// membocorkan digit awal/akhir tetap membocorkan nomor sekaligus panjangnya. Baris
// tetap bisa dibedakan lewat NAMA kontak — jadi tak ada guna membocorkan digit demi
// "biar bisa dikenali" (beda dari maskEmail, yang lahir saat tabel belum punya kolom
// nama). Nomor kosong tetap kosong (tak ada yang disembunyikan). Menerima ctx (bukan
// businessRole) karena kebijakan kini bergantung tenant.
func maskPhone(ctx context.Context, phone string) string {
	if phone == "" {
		return ""
	}
	if canSeeFullPhone(ctx) {
		return phone
	}
	return flsHidden
}
