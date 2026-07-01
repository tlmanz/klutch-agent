package agent

import (
	"testing"

	"github.com/tlmanz/klutch-agent/wire"
)

func TestFingerprint_OrderIndependentAndChangeSensitive(t *testing.T) {
	a := []wire.Printer{{Name: "Thermal-1", Description: "Epson"}, {Name: "Laser-2", Description: "HP"}}
	reordered := []wire.Printer{{Name: "Laser-2", Description: "HP"}, {Name: "Thermal-1", Description: "Epson"}}
	if fingerprint(a) != fingerprint(reordered) {
		t.Fatal("fingerprint must be order-independent (no spurious re-advertise)")
	}

	added := append([]wire.Printer{{Name: "Receipt-3", Description: "Star"}}, a...)
	if fingerprint(a) == fingerprint(added) {
		t.Fatal("adding a printer must change the fingerprint")
	}

	removed := []wire.Printer{{Name: "Thermal-1", Description: "Epson"}}
	if fingerprint(a) == fingerprint(removed) {
		t.Fatal("removing a printer must change the fingerprint")
	}

	renamedDesc := []wire.Printer{{Name: "Thermal-1", Description: "Epson TM-T20"}, {Name: "Laser-2", Description: "HP"}}
	if fingerprint(a) == fingerprint(renamedDesc) {
		t.Fatal("a description change must change the fingerprint")
	}
}

func TestVersionLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"v1.2.0", "v1.10.0", true},
		{"1.2.0", "1.2.0", false},
		{"v1.2.1", "v1.2.0", false},
		{"dev", "v1.0.0", true},
		{"v2.0.0", "v1.9.9", false},
	}
	for _, c := range cases {
		if got := versionLess(c.a, c.b); got != c.want {
			t.Errorf("versionLess(%q,%q)=%v want %v", c.a, c.b, got, c.want)
		}
	}
}
