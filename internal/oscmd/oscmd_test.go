package oscmd

import "testing"

func TestQuote(t *testing.T) {
	cases := []struct{ in, want string }{
		// A Windows path must reach PowerShell with its backslashes intact; this is
		// the case Go's %q gets wrong, doubling every one of them.
		{`C:\Users\ops\AppData\Local\Klutch\spool\job.pdf`, `'C:\Users\ops\AppData\Local\Klutch\spool\job.pdf'`},
		{"Generic / Text Only", "'Generic / Text Only'"},
		// A quote in the value is escaped by doubling, so it cannot close the
		// literal and run the rest of the string as script.
		{"Front O'Neill", "'Front O''Neill'"},
		{"", "''"},
	}
	for _, c := range cases {
		if got := Quote(c.in); got != c.want {
			t.Errorf("Quote(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}
