package storage

import "testing"

func TestSafeLocalPath(t *testing.T) {
	if _, err := safeLocalPath("uploads", "checkin/202609/user/file.jpg"); err != nil {
		t.Fatalf("valid storage key rejected: %v", err)
	}
	for _, key := range []string{"../secret", `..\secret`, "C:\\secret"} {
		if _, err := safeLocalPath("uploads", key); err == nil {
			t.Fatalf("unsafe storage key accepted: %q", key)
		}
	}
}
