package handler

import (
	"context"
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// scope.go — jantung anti-footgun multi-tenancy. Menggantikan h.DB (Queries
// terikat-pool, tanpa scope) dengan h.q(ctx) yang mengembalikan *db.Queries
// ber-tenant dari transaksi request. "Lupa scope" jadi PANIC keras di dev, bukan
// kebocoran tenant senyap. Isolasi sebenarnya ditegakkan Postgres RLS (migrasi
// 00007) — GUC transaction-local yang di-set WithTenant/WithSuper (internal/db).

type scopeCtxKey struct{}

// withQueries menaruh Queries ber-scope ke context (dipanggil middleware Scope).
func withQueries(ctx context.Context, q *db.Queries) context.Context {
	return context.WithValue(ctx, scopeCtxKey{}, q)
}

// q mengambil *db.Queries ber-tenant dari context request. PANIC bila Scope tak
// terpasang di rantai middleware — kesalahan wiring ketahuan seketika (dev),
// bukan jadi query lintas-tenant tak ter-scope. Handler SELALU pakai h.q(ctx),
// TIDAK PERNAH h.DB (yang sudah dihapus).
func (h *Handler) q(ctx context.Context) *db.Queries {
	q, ok := ctx.Value(scopeCtxKey{}).(*db.Queries)
	if !ok || q == nil {
		panic("handler.q: Scope middleware tidak terpasang — RLS tak ter-scope (bug wiring routes.go)")
	}
	return q
}

// Scope membuka SATU transaksi ber-scope per request dan menaruh Queries-nya di
// context. Platform (super_admin/staff) → WithSuper (bypass RLS); tenant user →
// WithTenant(session.TenantID). Anonim → next tanpa tx (handler terproteksi tak
// akan lolos RequireAuth). Pasang SETELAH RequireAuth, SEBELUM RefreshIdentity/
// TrackPresence (keduanya butuh h.q). Transaksi commit bila handler tak menulis
// error ke pipeline — di sini SELALU commit (handler pakai SSE/HTTP langsung,
// bukan return error); RLS + WITH CHECK yang menjaga integritas, bukan rollback.
func (h *Handler) Scope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		uid := session.UserID(ctx)
		if uid == 0 {
			next.ServeHTTP(w, r) // anonim — tak ada scope (route terproteksi butuh RequireAuth)
			return
		}
		run := func(q *db.Queries) error {
			next.ServeHTTP(w, r.WithContext(withQueries(ctx, q)))
			return nil
		}
		// Di mode single ini selalu terisi (slug tenant tunggal) — jadi seluruh
		// cabang di bawah berjalan sama persis seperti mode multi, tanpa satu pun
		// `if mode` tambahan (0006 §1: satu jalur kode, bukan dua).
		slug := slugFromRequest(r)

		// Platform (super_admin/staff) lintas-tenant → bypass RLS, tak perlu membership.
		if isPlatformRole(session.Role(ctx)) {
			// Tapi slug di path tetap WAJIB diikuti: handler membaca workspace dari
			// session.TenantID, jadi tanpa ini platform yang membuka /w/acme/members
			// akan melihat anggota workspace LAIN (yang kebetulan aktif di session)
			// di bawah URL yang menjanjikan acme — salah data secara senyap.
			if slug != "" {
				var ok bool
				if ctx, ok = h.adoptTenantBySlug(ctx, slug); !ok {
					http.NotFound(w, r)
					return
				}
				// run dirakit sebelum cabang ini dengan ctx yang lama — pasang ulang
				// agar sifat workspace ikut sampai ke handler.
				run = func(q *db.Queries) error {
					next.ServeHTTP(w, r.WithContext(withQueries(ctx, q)))
					return nil
				}
			}
			// SENGAJA TANPA gateLifecycle (0005): platform tetap boleh masuk
			// workspace yang ditangguhkan/diarsipkan/terhapus. Merekalah yang
			// menangguhkan — menghalangi mereka membuat suspensi mustahil
			// diselidiki, dan restore mustahil diverifikasi sebelum ditekan.
			// Ini keputusan, bukan kelalaian; dikunci TestPlatformTembusSuspensi.
			if err := db.WithSuper(ctx, h.Pool, run); err != nil {
				h.Log.Error("scope: tx super", "err", err)
			}
			return
		}

		// Tenant user: VALIDASI keanggotaan sebelum membuka tx ber-tenant. Baik slug
		// di path MAUPUN tenantID di session user-controlled → tanpa cek ini,
		// workspace sembarang bisa dipaksa.
		//
		// Slug di path MENANG atas session (0004): URL adalah state yang lengkap,
		// jadi dua tab bisa membuka workspace berbeda tanpa saling merusak.
		if slug != "" {
			t, ok := h.resolveTenantBySlug(ctx, uid, slug)
			if !ok {
				// 404, BUKAN 403 dan BUKAN redirect: 403 mengonfirmasi workspace itu
				// ada (kebocoran keberadaan), redirect ke workspace lain menampilkan
				// data yang salah secara senyap — penyakit yang justru diobati 0004.
				http.NotFound(w, r)
				return
			}
			// Gerbang siklus hidup (0005) — SETELAH keanggotaan terbukti. Urutannya
			// penting: yang dilindungi 0004 adalah keberadaan workspace dari ORANG
			// LUAR, bukan alasan dari orang dalam.
			if !h.gateLifecycle(w, r, t) {
				return
			}
			ctx = withTenantStatus(ctx, t.Status)
			ctx = withTenantPrimary(ctx, t.IsPrimary)
			run = func(q *db.Queries) error {
				next.ServeHTTP(w, r.WithContext(withQueries(ctx, q)))
				return nil
			}
			if err := db.WithTenant(ctx, h.Pool, t.ID, run); err != nil {
				h.Log.Error("scope: tx tenant", "err", err)
			}
			return
		}

		tenantID, ok := h.resolveActiveTenant(ctx, uid)
		if !ok {
			// Belum punya workspace sama sekali (mis. membership terakhir dicabut)
			// → arahkan buat workspace baru. Bukan error: keadaan sah di model ini.
			http.Redirect(w, r, "/workspace/new", http.StatusSeeOther)
			return
		}
		if err := db.WithTenant(ctx, h.Pool, tenantID, run); err != nil {
			// Commit/begin gagal SETELAH response ditulis handler — tak bisa ubah
			// status. Log saja (fail-visible). Isolasi tetap terjaga (tx rollback).
			h.Log.Error("scope: tx tenant", "err", err)
		}
	})
}
