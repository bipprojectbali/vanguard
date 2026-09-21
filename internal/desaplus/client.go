package desaplus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// ErrVillageNotFound — codeVanguard belum dipasangkan di sisi desa-plus (404
// bisnis, BUKAN error transport). Saat ditulis, 0 dari 2 desa di data dev
// mereka sudah diisi — kemungkinan besar operasional (tim desa-plus belum
// mengisi field `codeVanguard` untuk desa ini), bukan bug integrasi.
var ErrVillageNotFound = errors.New("desaplus: desa tidak ditemukan (codeVanguard belum dipasangkan)")

// ErrUnauthorized — x-api-key ditolak (401). Cek DESA_PLUS_TOKEN.
var ErrUnauthorized = errors.New("desaplus: unauthorized (token ditolak)")

// requestTimeout — desa-plus layanan pihak lain; sync dipicu tombol manual per
// desa (bukan job latar), jadi harus gagal cepat & jelas ke CSM yang menunggu.
const requestTimeout = 10 * time.Second

// Client — HTTP client tipis ke API integrasi khusus desa-plus (BL-27).
// Adapter TIPIS: satu method, satu endpoint, tak menyimpan state lintas-request.
type Client struct {
	baseURL string
	token   string
	hc      *http.Client
}

// NewClient membuat Client. baseURL TANPA trailing slash (dipangkas di
// config.MustLoad), token dikirim sebagai header `x-api-key` (BUKAN Bearer —
// kontrak API mereka, dikonfirmasi baca source langsung karena swagger rusak).
func NewClient(baseURL, token string) *Client {
	return &Client{baseURL: baseURL, token: token, hc: &http.Client{Timeout: requestTimeout}}
}

// villageSummaryResponse — bentuk amplop balasan API (success/message/data),
// sama di jalur sukses maupun gagal (data nil bila gagal).
type villageSummaryResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    *VillageSummary `json:"data"`
}

// VillageSummary memanggil GET {baseURL}/village-summary?codeVanguard=...
// Bentuk balasan terverifikasi baca langsung source sistem-desa-mandiri:
//   - sukses (200): success:true, data:{...}
//   - tak ditemukan (404, business-level): success:false, data:null → ErrVillageNotFound
//   - unauthorized (401): success:false, message:"Unauthorized" → ErrUnauthorized
//   - lainnya (mis. 500): success:false, data:null → error umum berkonteks
func (c *Client) VillageSummary(ctx context.Context, codeVanguard string) (*VillageSummary, error) {
	reqURL := c.baseURL + "/village-summary?" + url.Values{"codeVanguard": {codeVanguard}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("desaplus: build request: %w", err)
	}
	req.Header.Set("x-api-key", c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("desaplus: request village-summary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrUnauthorized
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("desaplus: read response body: %w", err)
	}

	var parsed villageSummaryResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("desaplus: parse response (status %d): %w", resp.StatusCode, err)
	}

	if !parsed.Success || parsed.Data == nil {
		if resp.StatusCode == http.StatusNotFound {
			return nil, ErrVillageNotFound
		}
		return nil, fmt.Errorf("desaplus: village-summary gagal (status %d): %s", resp.StatusCode, parsed.Message)
	}

	return parsed.Data, nil
}
