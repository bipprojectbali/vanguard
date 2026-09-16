package panel

import (
	"strings"
	"testing"
)

// contract_value_test.go — kartu "Nilai Kontrak / MRR" (blok D bagian kedua,
// BL-145 subtask 6): ✓/✗ per peran, murni presentasional (tanpa form/POST).

func TestContractValueCard_MarksPerRole(t *testing.T) {
	v := ContractValueView{Roles: []ContractValueRow{
		{Name: "admin", DisplayName: "Administrator", IsSystem: true, Visible: true},
		{Name: "sales", DisplayName: "Sales", Visible: true},
		{Name: "support", DisplayName: "Support", Visible: false},
	}}
	var out strings.Builder
	if err := contractValueCard(v).Render(&out); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := out.String()

	if !strings.Contains(body, "Nilai Kontrak / MRR") {
		t.Error("kartu harus berjudul Nilai Kontrak / MRR")
	}
	if strings.Contains(body, "<form") || strings.Contains(body, `method="post"`) {
		t.Error("kartu harus presentasional murni, tanpa form/POST")
	}

	supportIdx := strings.Index(body, "Support")
	nextRowIdx := len(body)
	if i := strings.Index(body[supportIdx+1:], "</tr>"); i >= 0 {
		nextRowIdx = supportIdx + 1 + i
	}
	if supportIdx < 0 || !strings.Contains(body[supportIdx:nextRowIdx], "✗") {
		t.Error("baris Support harus bertanda ✗ (canSeeARR false)")
	}

	salesIdx := strings.Index(body, ">Sales<")
	salesNextRow := len(body)
	if i := strings.Index(body[salesIdx+1:], "</tr>"); i >= 0 {
		salesNextRow = salesIdx + 1 + i
	}
	if salesIdx < 0 || !strings.Contains(body[salesIdx:salesNextRow], "✓") {
		t.Error("baris Sales harus bertanda ✓ (canSeeARR true)")
	}
}
