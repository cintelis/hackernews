package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"testing"
)

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		sign int
	}{
		{"1.2.3", "1.2.3", 0},
		{"1.10.0", "1.9.9", 1},
		{"0.9.0", "1.0.0", -1},
		{"v2.0.0", "1.99.99", 1},
		{"1.0.0", "dev", 1},
	}
	for _, c := range cases {
		got := Compare(c.a, c.b)
		if (got > 0) != (c.sign > 0) || (got < 0) != (c.sign < 0) {
			t.Errorf("Compare(%q, %q) = %d, want sign %d", c.a, c.b, got, c.sign)
		}
	}
}

func TestAssetName(t *testing.T) {
	if got := AssetName("1.2.3", "linux", "arm64"); got != "cintelis_1.2.3_linux_arm64.tar.gz" {
		t.Error(got)
	}
	if got := AssetName("1.2.3", "windows", "amd64"); got != "cintelis_1.2.3_windows_amd64.zip" {
		t.Error(got)
	}
}

func TestChecksumFor(t *testing.T) {
	sum := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	sums := []byte("bbbb  other.zip\n" + sum + "  cintelis_1.0.0_linux_amd64.tar.gz\n")
	if got, err := ChecksumFor(sums, "cintelis_1.0.0_linux_amd64.tar.gz"); err != nil || got != sum {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := ChecksumFor(sums, "cintelis_1.0.0_darwin_arm64.tar.gz"); err == nil {
		t.Fatal("unlisted asset should be an error")
	}
	if _, err := ChecksumFor([]byte("short  other.zip\n"), "other.zip"); err == nil {
		t.Fatal("malformed checksum should be rejected")
	}
}

func TestExtractTarGz(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	add := func(name string, typ byte, body string) {
		tw.WriteHeader(&tar.Header{Name: name, Typeflag: typ, Size: int64(len(body)), Mode: 0o755})
		tw.Write([]byte(body))
	}
	add("README.md", tar.TypeReg, "docs")
	add("cintelis", tar.TypeSymlink, "")
	add("../../cintelis", tar.TypeReg, "BINARY")
	tw.Close()
	gz.Close()

	got, err := Extract(buf.Bytes(), "linux")
	if err != nil || string(got) != "BINARY" {
		t.Fatalf("Extract = %q, %v", got, err)
	}
}

func TestExtractZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("LICENSE")
	w.Write([]byte("mit"))
	w, _ = zw.Create("cintelis.exe")
	w.Write([]byte("EXE"))
	zw.Close()

	got, err := Extract(buf.Bytes(), "windows")
	if err != nil || string(got) != "EXE" {
		t.Fatalf("Extract = %q, %v", got, err)
	}
	if _, err := Extract([]byte("not a zip"), "windows"); err == nil {
		t.Fatal("garbage should fail")
	}
}
