package handler

import "testing"

// dashboard_charts_test.go — BL-140: pieOption/barOption adalah helper GENERIK
// baru (menggantikan pola satu-fungsi-per-grafik era pre-BL-98) dipakai lintas
// domain di BL-141..143. Marshal JSON diuji lewat h.marshalChart di
// dashboard_test.go (F4 pipeline sudah ada); di sini cukup bentuk map-nya.

func TestPieOption_Shape(t *testing.T) {
	opt := pieOption("Leads", []string{"Web", "Referral"}, []int64{5, 3})
	series, ok := opt["series"].([]any)
	if !ok || len(series) != 1 {
		t.Fatalf("series harus 1 elemen, got %#v", opt["series"])
	}
	s := series[0].(map[string]any)
	if s["type"] != "pie" || s["name"] != "Leads" {
		t.Errorf("series salah bentuk: %#v", s)
	}
	data, ok := s["data"].([]any)
	if !ok || len(data) != 2 {
		t.Fatalf("data harus 2 elemen (sejajar labels/values), got %#v", s["data"])
	}
	first := data[0].(map[string]any)
	if first["name"] != "Web" || first["value"] != int64(5) {
		t.Errorf("elemen pertama salah: %#v", first)
	}
}

func TestBarOption_Shape(t *testing.T) {
	opt := barOption([]string{"Prospecting", "Demo"}, []int64{4, 2})
	xAxis, ok := opt["xAxis"].(map[string]any)
	if !ok || xAxis["type"] != "category" {
		t.Fatalf("xAxis salah bentuk: %#v", opt["xAxis"])
	}
	cats, ok := xAxis["data"].([]string)
	if !ok || len(cats) != 2 || cats[0] != "Prospecting" {
		t.Errorf("kategori xAxis salah: %#v", xAxis["data"])
	}
	series, ok := opt["series"].([]any)
	if !ok || len(series) != 1 {
		t.Fatalf("series harus 1 elemen, got %#v", opt["series"])
	}
	s := series[0].(map[string]any)
	if s["type"] != "bar" {
		t.Errorf("series type harus bar: %#v", s)
	}
	vals, ok := s["data"].([]int64)
	if !ok || len(vals) != 2 || vals[0] != 4 {
		t.Errorf("data series salah: %#v", s["data"])
	}
}
