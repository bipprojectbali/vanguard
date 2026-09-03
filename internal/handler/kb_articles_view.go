package handler

import (
	"context"
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/ui/pages/panel"
)

// kb_articles_view.go — gerbang F2 (Casbin bisnis) modul Knowledge Base
// (Modul 6 Customer Success, slice A3). Satu tempat objek Casbin "crm:kb"
// disebut, agar menu sidebar & gate halaman selalu menyebut objek/aksi yang
// SAMA (nol menu hantu). Meniru playbooks_view.go (slice A2).
//
// Katalog master TANPA F3 ownership: artikel milik WORKSPACE, bukan per-
// desa — RLS (h.q ber-tenant) mengurungnya, tak ada kolom pemilik. Matriks
// izin crm:kb SUDAH lengkap sejak sebelum slice ini (business_policy.csv):
// write = admin (via crm:* glob) + manager + support; read-only = sales +
// csm — tak perlu diubah.

// canViewKBArticles = gerbang READ katalog. Sumber tunggal untuk menu & gate
// halaman KBArticlesList/KBArticleEdit.
func canViewKBArticles(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:kb", "read")
}

// canWriteKBArticlesPerm = izin F2 mentah tulis artikel (tanpa cek arsip).
// Dipakai gate aksi POST; write mencakup read di Casbin.
func canWriteKBArticlesPerm(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:kb", "write")
}

// canWriteKBArticles = tombol tulis di view: izin F2 write DAN workspace
// tak read-only (arsip). Dihitung di handler, dioper bool — view tak
// panggil authz.
func canWriteKBArticles(ctx context.Context) bool {
	return canWriteKBArticlesPerm(ctx) && !IsReadOnly(ctx)
}

// renderKBArticlesForbidden — 403 + penjelasan bagi anggota tanpa peran CRM.
func (h *Handler) renderKBArticlesForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Knowledge Base", "/kb-articles", panel.SalesForbidden("Knowledge Base"))
}

// kbArticlesMsg memetakan ?ok= → pesan sukses Knowledge Base (dipisah dari
// kbArticlesErrMsg agar alert sukses & galat tak pernah tertukar variannya).
func kbArticlesMsg(code string) string {
	switch code {
	case "created":
		return "Artikel ditambahkan ke katalog (draf)."
	case "saved":
		return "Perubahan artikel disimpan."
	case "published":
		return "Artikel diterbitkan."
	case "returned_to_draft":
		return "Artikel dikembalikan ke draf."
	case "archived":
		return "Artikel diarsipkan."
	case "unarchived":
		return "Artikel dipulihkan dari arsip ke draf."
	default:
		return ""
	}
}

// kbArticlesErrMsg memetakan ?err= → pesan galat form Knowledge Base. Lokal
// ke modul (tak menumpang wsErrMsg bersama) agar kode galat khas artikel
// terkumpul di satu tempat.
func kbArticlesErrMsg(code string) string {
	switch code {
	case "required":
		return "Judul artikel wajib diisi."
	case "visibility":
		return "Visibilitas harus salah satu: Public, Internal, atau Portal Only."
	case "failed":
		return "Gagal menyimpan artikel. Coba lagi."
	default:
		return ""
	}
}
