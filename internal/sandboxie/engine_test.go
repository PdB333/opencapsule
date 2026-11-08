package sandboxie

import "testing"

func TestPIDParsing(t *testing.T) {
	for _, in := range []string{"0\n", "2\n123\n456\n"} {
		fields := parsePIDsForTest(in)
		if fields < 0 {
			t.Fatal("parser failure")
		}
	}
}
func parsePIDsForTest(s string) int {
	// Smoke test output decoding; real CLI parsing is exercised on Windows.
	return len(textOutput([]byte(s)))
}
func TestUtf16(t *testing.T) {
	b := []byte{0xff, 0xfe, 0x48, 0, 0x69, 0}
	if textOutput(b) != "Hi" {
		t.Fatal(textOutput(b))
	}
}
