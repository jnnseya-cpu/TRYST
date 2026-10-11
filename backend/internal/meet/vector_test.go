package meet

import "testing"

// Cross-language vector: web/test/doorwords.test.ts pins the same words, so both phones
// (any platform) show identical door-check words.
func TestDoorWordsVector(t *testing.T) {
	got := DoorWords([]byte("tryst-vector-secret-0123456789ab"), "mt_vector")
	if got != [3]string{"fern", "lantern", "birch"} {
		t.Fatalf("door words changed: %v", got)
	}
}
