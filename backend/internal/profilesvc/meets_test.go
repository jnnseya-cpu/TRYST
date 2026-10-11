package profilesvc

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func confirmedMeet(t *testing.T) (*world, *LogAlerting, string) {
	w, tid := mutualPair(t)
	al := &LogAlerting{}
	w.s.Alerts = al
	start := w.now.Add(time.Hour).Format(time.RFC3339)
	end := w.now.Add(4 * time.Hour).Format(time.RFC3339)
	c, m := w.do("ana", "POST", "/v1/threads/"+tid+"/meets", fmt.Sprintf(`{"start":"%s","end":"%s"}`, start, end))
	if c != 201 {
		t.Fatalf("propose: %d %v", c, m)
	}
	id := m["id"].(string)
	if c, _ := w.do("eve", "GET", "/v1/meets/"+id, ""); c != 404 {
		t.Fatal("outsider sees the meet")
	}
	if c, _ := w.do("ana", "POST", "/v1/meets/"+id+"/confirm", ""); c != 409 {
		t.Fatal("proposer confirmed their own meet")
	}
	if c, out := w.do("ben", "POST", "/v1/meets/"+id+"/confirm", ""); c != 200 || out["state"] != "confirmed" {
		t.Fatalf("confirm: %d %v", c, out)
	}
	return w, al, id
}

func TestMeetProposalValidation(t *testing.T) {
	w, tid := mutualPair(t)
	bad := []string{
		`{"start":"2020-01-01T20:00:00Z","end":"2020-01-01T22:00:00Z"}`,                                                    // past
		fmt.Sprintf(`{"start":"%s","end":"%s"}`, w.now.Format(time.RFC3339), w.now.Add(13*time.Hour).Format(time.RFC3339)), // too long
		fmt.Sprintf(`{"start":"%s","end":"%s"}`, w.now.Add(2*time.Hour).Format(time.RFC3339), w.now.Format(time.RFC3339)),  // ends first
	}
	for _, b := range bad {
		if c, _ := w.do("ana", "POST", "/v1/threads/"+tid+"/meets", b); c != 422 {
			t.Fatalf("%s accepted: %d", b, c)
		}
	}
	if c, out := w.do("ana", "POST", "/v1/threads/"+tid+"/meets", `{"start":"x","end":"y","place":"Bar"}`); c != 422 || strings.Contains(fmt.Sprint(out), "Bar") {
		t.Fatal("server must never take a place")
	}
}

func TestMissedCheckInEscalates(t *testing.T) {
	w, al, id := confirmedMeet(t)
	if c, _ := w.do("ana", "POST", "/v1/meets/"+id+"/safety", `{"every_minutes":30,"pin":"2468","duress_pin":"2468","trusted_contact":"+447700900111"}`); c != 422 {
		t.Fatal("same PINs accepted")
	}
	if c, _ := w.do("ana", "POST", "/v1/meets/"+id+"/safety", `{"every_minutes":30,"pin":"2468","duress_pin":"1357"}`); c != 422 {
		t.Fatal("no trusted contact and no safety line accepted")
	}
	if c, out := w.do("ana", "POST", "/v1/meets/"+id+"/safety", `{"every_minutes":30,"pin":"2468","duress_pin":"1357","trusted_contact":"+447700900111","safety_line":true}`); c != 200 || out["safety_active"] != true {
		t.Fatalf("start: %d %v", c, out)
	}
	// Check-ins start at the meet start (1 h from now). 30 min + 10 min grace after that.
	w.now = w.now.Add(time.Hour + 35*time.Minute)
	w.s.SweepSafety(context.Background())
	if len(al.Trusted) != 0 {
		t.Fatal("alerted inside grace")
	}
	w.now = w.now.Add(6 * time.Minute)
	w.s.SweepSafety(context.Background())
	if len(al.Trusted) != 1 || al.Trusted[0] != "+447700900111" {
		t.Fatalf("trusted contact not alerted: %v", al.Trusted)
	}
	w.now = w.now.Add(16 * time.Minute)
	w.s.SweepSafety(context.Background())
	if len(al.Line) != 1 || al.Line[0].MemberAlias != "Ana" || al.Line[0].CallbackPhone != "" {
		t.Fatalf("safety line: %+v", al.Line)
	}
}

func TestDuressLooksNormalAndCannotEndTheMeet(t *testing.T) {
	w, al, id := confirmedMeet(t)
	w.do("ana", "POST", "/v1/meets/"+id+"/safety", `{"every_minutes":60,"pin":"2468","duress_pin":"1357","trusted_contact":"friend@example.com","safety_line":true}`)
	w.now = w.now.Add(90 * time.Minute)
	c1, normal := w.do("ana", "POST", "/v1/meets/"+id+"/checkin", `{"pin":"2468"}`)
	c2, duress := w.do("ana", "POST", "/v1/meets/"+id+"/checkin", `{"pin":"1357"}`)
	if c1 != 200 || c2 != 200 || fmt.Sprint(normal) != fmt.Sprint(duress) {
		t.Fatalf("duress reply differs: %v vs %v", normal, duress)
	}
	if len(al.Trusted) != 1 || len(al.Line) != 1 {
		t.Fatalf("duress must alert at once: trusted=%v line=%v", al.Trusted, al.Line)
	}
	if c, _ := w.do("ana", "POST", "/v1/meets/"+id+"/checkin", `{"pin":"0000"}`); c != 422 {
		t.Fatal("wrong PIN accepted")
	}
	// "Home safe" with the duress PIN looks the same but ends nothing.
	_, out := w.do("ana", "POST", "/v1/meets/"+id+"/checkin", `{"pin":"1357","end":true}`)
	if out["ended"] != true {
		t.Fatal("duress end must look like a normal end")
	}
	if _, m := w.do("ana", "GET", "/v1/meets/"+id, ""); m["safety_active"] != true {
		t.Fatal("duress PIN ended the check-ins")
	}
	// The real PIN ends it and drops the contact details.
	w.do("ana", "POST", "/v1/meets/"+id+"/checkin", `{"pin":"2468","end":true}`)
	if _, m := w.do("ana", "GET", "/v1/meets/"+id, ""); m["safety_active"] == true {
		t.Fatal("not ended")
	}
	ss := w.s.mt.sessions[id+"|acc-ana"]
	if ss.Trusted != "" {
		t.Fatal("trusted contact kept after the meet")
	}
}

func TestPanic(t *testing.T) {
	w, al, id := confirmedMeet(t)
	_, out := w.do("ben", "POST", "/v1/safety/panic", fmt.Sprintf(`{"meet_id":"%s","lat":51.5362,"lon":-0.1033}`, id))
	if out["call"] != "999" || !strings.Contains(out["silent_solution"].(string), "55") {
		t.Fatalf("panic reply: %v", out)
	}
	if len(al.Panics) != 1 || al.Panics[0].Location != "51.5362,-0.1033" || al.Panics[0].MemberAlias != "Ben" {
		t.Fatalf("emergency alert: %+v", al.Panics)
	}
	// Cancelled meets can't start check-ins.
	w.do("ana", "POST", "/v1/meets/"+id+"/cancel", "")
	if c, _ := w.do("ana", "POST", "/v1/meets/"+id+"/safety", `{"every_minutes":30,"pin":"2468","duress_pin":"1357","safety_line":true}`); c != 409 {
		t.Fatal("check-ins on a cancelled meet")
	}
}
