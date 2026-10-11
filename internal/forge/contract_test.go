package forge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func validDeck() Deck {
	return Deck{Name: "Transport fixture", Commanders: []Card{{Name: "Isamaru, Hound of Konda", Quantity: 1}}, Main: []Card{{Name: "Plains", Quantity: 99}}}
}
func TestDeckTransportValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Deck)
	}{{"injected section", func(d *Deck) { d.Main[0].Name = "Plains\n[Commander]" }}, {"zero quantity", func(d *Deck) { d.Main[0].Quantity = 0 }}, {"missing card", func(d *Deck) { d.Main[0].Quantity = 98 }}, {"duplicate", func(d *Deck) { d.Main = []Card{{Name: "Plains", Quantity: 49}, {Name: "Plains", Quantity: 50}} }}, {"too many commanders", func(d *Deck) {
		d.Commanders = append(d.Commanders, Card{Name: "A", Quantity: 1}, Card{Name: "B", Quantity: 1})
	}}} {
		t.Run(tc.name, func(t *testing.T) {
			d := validDeck()
			tc.change(&d)
			if d.Validate() == nil {
				t.Fatal("invalid transport accepted")
			}
		})
	}
	d := validDeck()
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}
func TestFiveDefaultSnapshots(t *testing.T) {
	paths, err := filepath.Glob("../../services/forge/defaults/*.json")
	if err != nil || len(paths) != 5 {
		t.Fatal("expected exactly five defaults", err)
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var d Deck
		if err = json.Unmarshal(b, &d); err != nil {
			t.Fatal(err)
		}
		if err = d.Validate(); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
}

func TestQueuedConcessionIsSessionIntentNotPromptReply(t *testing.T) {
	a := Action{Request: "concede-request", Session: "engine-incarnation", Action: "requestConcede"}
	if err := a.Validate(); err != nil {
		t.Fatal(err)
	}
	a.Revision = 1
	if a.Validate() == nil {
		t.Fatal("ambiguous prompt-shaped concession intent")
	}
	a.Revision = 0
	a.Action = "reply"
	if a.Validate() == nil {
		t.Fatal("ordinary reply bypassed revision check")
	}
}

func TestPhaseStopIsSessionPreference(t *testing.T) {
	enabled := false
	a := Action{Request: "phase-stop-request", Session: "engine-incarnation", Action: "setPhaseStop", Seat: "CPU", Phase: "COMBAT_DAMAGE", Enabled: &enabled}
	if err := a.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Action){
		func(a *Action) { a.Revision = 1 }, func(a *Action) { a.Prompt = "current-prompt" }, func(a *Action) { a.Seat = "opponent" }, func(a *Action) { a.Phase = "attack" }, func(a *Action) { a.Enabled = nil }, func(a *Action) { a.Selected = []int{1} }, func(a *Action) { a.Action = "ok" },
	} {
		invalid := a
		change(&invalid)
		if invalid.Validate() == nil {
			t.Errorf("accepted invalid phase preference: %+v", invalid)
		}
	}
}

func TestDiagnosticFailureReferenceDoesNotAcceptRawText(t *testing.T) {
	for _, code := range []string{"adapter_failure", "adapter_failure_assignCombat_NullPointerException", "unsupported_gui_chooseDirection"} {
		if !validFailureCode(code) {
			t.Errorf("rejected safe reference %s", code)
		}
	}
	for _, code := range []string{"adapter_failure: hidden card", "adapter_failure_\nsecret", "unknown_code"} {
		if validFailureCode(code) {
			t.Errorf("accepted unsafe reference %q", code)
		}
	}
}

func TestDeckPrintingTransport(t *testing.T) {
	d := validDeck()
	d.Main[0].Printing = "4e5c0438-e691-4c5f-9d04-1861f9588a09"
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	d.Main[0].Printing = "https://example.test/image"
	if d.Validate() == nil {
		t.Fatal("arbitrary printing URL accepted")
	}
}
