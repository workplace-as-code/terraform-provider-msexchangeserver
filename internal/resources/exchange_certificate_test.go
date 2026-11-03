package resources

import (
	"slices"
	"testing"
)

const testThumbprint = "0123456789ABCDEF0123456789ABCDEF01234567"

func TestParseCertificateID(t *testing.T) {
	server, thumbprint, err := parseCertificateID(`EX01.contoso.com\` + testThumbprint)
	if err != nil || server != "EX01.contoso.com" || thumbprint != testThumbprint {
		t.Fatalf("got %q %q %v", server, thumbprint, err)
	}
	for _, bad := range []string{
		testThumbprint,       // no server
		`\` + testThumbprint, // empty server
		`EX01\0123456789abcdef0123456789abcdef01234567`, // lower case
		`EX01\01234567`, // too short
	} {
		if _, _, err := parseCertificateID(bad); err == nil {
			t.Errorf("%q: expected an error", bad)
		}
	}
}

func TestKeptServicesIgnoresServicesNotManaged(t *testing.T) {
	// Exchange marked the certificate for SMTP on its own: SMTP must not show
	// up in the state of a resource that only manages POP and IMAP.
	got := keptServices([]string{"POP", "IMAP"}, []string{"IMAP", "POP", "SMTP"})
	if !slices.Equal(got, []string{"POP", "IMAP"}) {
		t.Errorf("got %v", got)
	}
	// A service moved to another certificate drops out, so the plan enables it again.
	if got := keptServices([]string{"POP", "IIS"}, []string{"POP"}); !slices.Equal(got, []string{"POP"}) {
		t.Errorf("got %v", got)
	}
	// After an import, every known service the certificate carries.
	if got := keptServices(certificateServices, []string{"IMAP", "POP", "IIS", "SMTP", "Federation"}); len(got) != 4 {
		t.Errorf("got %v", got)
	}
}

func TestMissingServices(t *testing.T) {
	if got := missingServices([]string{"IIS", "SMTP"}, []string{"SMTP"}); !slices.Equal(got, []string{"IIS"}) {
		t.Errorf("got %v", got)
	}
	if got := missingServices([]string{"IIS"}, []string{"IIS", "SMTP"}); len(got) != 0 {
		t.Errorf("got %v", got)
	}
}
