package handler

import (
	"context"
	"errors"
	"fmt"

	"go_starter/internal/db"
	"go_starter/internal/oauth"

	"github.com/jackc/pgx/v5"
)

// oauth_google_link.go — pencarian/penautan user dari klaim Google
// (findOrLinkGoogleUser) & penyegaran profil. Dipisah dari handler login/
// callback di oauth_google.go agar file di bawah ambang tipe Route/Handler
// (150). Satu paket handler.

// findOrLinkGoogleUser memetakan claim Google ke user lokal dalam SATU transaksi:
//  1. oauth_account untuk (google, sub) ada → login user itu.
//  2. belum ada → user dgn email itu ada → AUTO-LINK (buat oauth_account).
//  3. tak ada → buat user OAuth baru + oauth_account.
//
// Auto-link aman di model ini: email_verified sudah dipastikan true oleh
// VerifyIDToken, dan password auth hanya dev-only (bukan permukaan takeover di prod).
func (h *Handler) findOrLinkGoogleUser(ctx context.Context, claims *oauth.Claims) (int64, error) {
	const provider = "google"

	// Pre-identity: tenant belum diketahui (find-or-link MEMUTUSKANNYA). WithSuper
	// = SATU tx bypass RLS (email/sub global-unique). Semua cabang commit/rollback
	// bersama; new-user membuat tenant+owner atomik dalam tx yang sama.
	var userID int64
	// Profil dari provider dibersihkan SEKALI di sini, bukan di tiap cabang:
	// keduanya user-controlled (lihat NormalizeDisplayName), dan pembersihan yang
	// diulang per-cabang adalah tempat satu cabang tertinggal saat aturannya berubah.
	avatar := oauth.NormalizeAvatarURL(claims.Picture)
	name := oauth.NormalizeDisplayName(claims.Name)
	err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		// 1. Identitas Google sudah tertaut? Refresh profil (avatar & nama berubah
		//    di sisi Google tanpa memberi tahu kita; login = satu-satunya kabar).
		acc, err := q.GetOAuthAccount(ctx, db.GetOAuthAccountParams{Provider: provider, ProviderUid: claims.Sub})
		switch {
		case err == nil:
			if err := refreshProfile(ctx, q, acc.UserID, avatar, name); err != nil {
				return err
			}
			userID = acc.UserID
			return nil
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}

		// 2. User dgn email itu sudah ada (mis. akun password dev) → auto-link + profil.
		//    oauth_account ber-tenant_id (RLS) → pakai workspace PERTAMA user. User
		//    kini bisa anggota banyak workspace; tautan OAuth cukup di satu workspace
		//    (identitas login global, bukan per-workspace).
		user, err := q.GetUserByEmail(ctx, claims.Email)
		switch {
		case err == nil:
			ms, e := q.ListMembershipsByUser(ctx, user.ID)
			if e != nil {
				return e
			}
			if len(ms) == 0 {
				return fmt.Errorf("oauth link: user %d tanpa workspace", user.ID)
			}
			if _, err := q.CreateOAuthAccount(ctx, db.CreateOAuthAccountParams{
				UserID: user.ID, Provider: provider, ProviderUid: claims.Sub, TenantID: ms[0].TenantID,
			}); err != nil {
				return err
			}
			if err := refreshProfile(ctx, q, user.ID, avatar, name); err != nil {
				return err
			}
			userID = user.ID
			return nil
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}

		// 3. User baru sepenuhnya dari Google → buat USER + WORKSPACE pertama +
		//    MEMBERSHIP owner (sama seperti register password). Nama workspace =
		//    bagian email sebelum '@' (dirapikan user via /admin/workspace).
		//
		//    `wsName`, bukan `name`: nama WORKSPACE dan nama ORANG adalah dua hal
		//    berbeda yang kebetulan berdampingan di sini. Menamai keduanya sama
		//    membuat yang satu membayangi yang lain tanpa peringatan compiler —
		//    orangnya lalu tersimpan dengan nama workspace, dan tak ada yang
		//    memberi tahu.
		wsName := emailLocal(claims.Email)
		newUser, err := q.CreateOAuthUser(ctx, db.CreateOAuthUserParams{
			Email: claims.Email, AvatarUrl: avatar, Name: name,
		})
		if err != nil {
			return err
		}
		// Sama seperti register password: multi = workspace baru + owner, single =
		// gabung ke aplikasi tunggal sebagai member (0006 §6).
		t, err := placeNewUser(ctx, q, newUser.ID, wsName)
		if err != nil {
			return err
		}
		if _, err := q.CreateOAuthAccount(ctx, db.CreateOAuthAccountParams{
			UserID: newUser.ID, Provider: provider, ProviderUid: claims.Sub, TenantID: t.ID,
		}); err != nil {
			return err
		}
		userID = newUser.ID
		return nil
	})
	return userID, err
}

// refreshProfile menyegarkan avatar & nama tampilan dari provider untuk user
// yang SUDAH ada. Dipanggil di kedua cabang login-berulang (tertaut & auto-link)
// — satu tempat, supaya keduanya tak bisa berbeda perlakuan.
//
// Kedua argumen boleh nil ("provider tak mengirimkannya kali ini"); SQL-nya
// ber-COALESCE, jadi nil MEMPERTAHANKAN nilai lama alih-alih menghapusnya.
// Query dilewati sama sekali bila keduanya nil — bukan demi kecepatan, tapi agar
// login tak menulis ke tabel users tanpa ada satu pun yang berubah.
func refreshProfile(ctx context.Context, q *db.Queries, userID int64, avatar, name *string) error {
	if avatar == nil && name == nil {
		return nil
	}
	return q.UpdateUserProfile(ctx, db.UpdateUserProfileParams{
		ID: userID, AvatarUrl: avatar, Name: name,
	})
}
