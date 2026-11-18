package logger

import "testing"

func TestMaskEmail(t *testing.T) {
	tests := map[string]string{
		"josej@example.com":      "j***@example.com",
		"  ana.lopez@mail.co  ":  "a***@mail.co",
		"a@example.com":          "a***@example.com",
		"delivered+x@resend.dev": "d***@resend.dev",
		"":                       "",
		"not-an-email":           "***",
	}

	for input, want := range tests {
		if got := MaskEmail(input); got != want {
			t.Errorf("MaskEmail(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestMaskEmails(t *testing.T) {
	got := MaskEmails([]string{"josej@example.com", "ana@example.com"})
	if want := "j***@example.com, a***@example.com"; got != want {
		t.Fatalf("MaskEmails() = %q, want %q", got, want)
	}
}
