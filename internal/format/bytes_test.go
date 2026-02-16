package format

import "testing"

func TestBytesNegativeReturnsZero(t *testing.T) {
	if got := Bytes(-1); got != "0 B" {
		t.Fatalf("expected '0 B', got %q", got)
	}
}

func TestBytesZero(t *testing.T) {
	if got := Bytes(0); got != "0 B" {
		t.Fatalf("expected '0 B', got %q", got)
	}
}

func TestBytesSmallValues(t *testing.T) {
	if got := Bytes(1); got != "1 B" {
		t.Fatalf("expected '1 B', got %q", got)
	}
	if got := Bytes(1023); got != "1023 B" {
		t.Fatalf("expected '1023 B', got %q", got)
	}
}

func TestBytesKiB(t *testing.T) {
	got := Bytes(1024)
	if got != "1.0 KiB" {
		t.Fatalf("expected '1.0 KiB', got %q", got)
	}
	got = Bytes(1536)
	if got != "1.5 KiB" {
		t.Fatalf("expected '1.5 KiB', got %q", got)
	}
}

func TestBytesMiB(t *testing.T) {
	got := Bytes(1024 * 1024)
	if got != "1.0 MiB" {
		t.Fatalf("expected '1.0 MiB', got %q", got)
	}
}

func TestBytesGiB(t *testing.T) {
	got := Bytes(1024 * 1024 * 1024)
	if got != "1.0 GiB" {
		t.Fatalf("expected '1.0 GiB', got %q", got)
	}
}

func TestBytesTiB(t *testing.T) {
	got := Bytes(1024 * 1024 * 1024 * 1024)
	if got != "1.0 TiB" {
		t.Fatalf("expected '1.0 TiB', got %q", got)
	}
}

func TestBytesPiB(t *testing.T) {
	got := Bytes(1024 * 1024 * 1024 * 1024 * 1024)
	if got != "1.0 PiB" {
		t.Fatalf("expected '1.0 PiB', got %q", got)
	}
}

func TestBytesLargePiB(t *testing.T) {
	got := Bytes(5 * 1024 * 1024 * 1024 * 1024 * 1024)
	if got != "5.0 PiB" {
		t.Fatalf("expected '5.0 PiB', got %q", got)
	}
}
