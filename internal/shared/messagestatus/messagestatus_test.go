package messagestatus

import "testing"

func TestValid(t *testing.T) {
	for _, s := range All {
		if !Valid(s) {
			t.Errorf("Valid(%q) = false, want true", s)
		}
	}
	for _, s := range []Status{"", "unknown", "UNKNOWN", "Sent", "delivering", "canceled"} {
		if Valid(s) {
			t.Errorf("Valid(%q) = true, want false", s)
		}
	}
}

func TestIsTerminal(t *testing.T) {
	terminal := []Status{Delivered, Failed, Expired, Rejected, Cancelled}
	nonTerminal := []Status{Pending, Queued, Scheduled, Sent}

	for _, s := range terminal {
		if !IsTerminal(s) {
			t.Errorf("IsTerminal(%q) = false, want true", s)
		}
	}
	for _, s := range nonTerminal {
		if IsTerminal(s) {
			t.Errorf("IsTerminal(%q) = true, want false", s)
		}
	}
}

func TestFromDLRStat(t *testing.T) {
	cases := []struct {
		stat   string
		want   Status
		wantOK bool
	}{
		{"DELIVRD", Delivered, true},
		{"EXPIRED", Expired, true},
		{"REJECTD", Failed, true},
		{"UNDELIV", Failed, true},
		{"UNKNOWN", "", false},
		{"ACCEPTD", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		got, ok := FromDLRStat(tc.stat)
		if got != tc.want || ok != tc.wantOK {
			t.Errorf("FromDLRStat(%q) = (%q, %v), want (%q, %v)",
				tc.stat, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestToSMSC(t *testing.T) {
	cases := []struct {
		status Status
		stat   string
		dlvrd  string
	}{
		{Delivered, "DELIVRD", "001"},
		{Failed, "UNDELIV", "000"},
		{Expired, "EXPIRED", "000"},
		{Rejected, "REJECTD", "000"},
		{Sent, "UNKNOWN", "000"},
		{Pending, "UNKNOWN", "000"},
	}
	for _, tc := range cases {
		stat, dlvrd := ToSMSC(tc.status)
		if stat != tc.stat || dlvrd != tc.dlvrd {
			t.Errorf("ToSMSC(%q) = (%q, %q), want (%q, %q)",
				tc.status, stat, dlvrd, tc.stat, tc.dlvrd)
		}
	}
}

// Round-trip: every DLR stat that maps to a status maps back to a receipt
// representation through ToSMSC.
func TestFromDLRStat_ToSMSC_RoundTrip(t *testing.T) {
	for _, stat := range []string{"DELIVRD", "EXPIRED", "REJECTD", "UNDELIV"} {
		s, ok := FromDLRStat(stat)
		if !ok {
			t.Fatalf("FromDLRStat(%q) not ok", stat)
		}
		outStat, _ := ToSMSC(s)
		if !IsTerminal(s) {
			t.Errorf("status %q derived from DLR stat must be terminal", s)
		}
		if outStat == "UNKNOWN" {
			t.Errorf("ToSMSC(%q) = UNKNOWN, want a receipt code", s)
		}
	}
}
