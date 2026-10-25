package safetysvc

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/comms"
	"github.com/jnnseya-cpu/tryst/backend/internal/svcauth"
)

// Communications page: the catalogue of everything TRYST sends, channel coverage, what is
// never sent and why, the delivery log (from the member platform, which does the sending)
// and test sends. Any staff member can read it; only a lead can fire a test, and a test
// only ever goes to the lead's own staff address or is recorded in sandbox.

// CommsSource is the member platform's delivery log and test sender.
type CommsSource interface {
	Snapshot(ctx context.Context) (comms.Snapshot, string, error)
	Test(ctx context.Context, event, to string) ([]map[string]string, error)
}

// RemoteComms reads the log over the internal API (D-59).
type RemoteComms struct{ C *svcauth.Client }

func (c RemoteComms) Snapshot(ctx context.Context) (comms.Snapshot, string, error) {
	var out struct {
		Log           comms.Snapshot `json:"log"`
		EmailProvider string         `json:"email_provider"`
	}
	err := c.C.Do(ctx, "GET", "/internal/v1/comms", nil, &out)
	return out.Log, out.EmailProvider, err
}

func (c RemoteComms) Test(ctx context.Context, event, to string) ([]map[string]string, error) {
	var out struct {
		Results []map[string]string `json:"results"`
	}
	err := c.C.Do(ctx, "POST", "/internal/v1/comms/test", map[string]string{"event": event, "to": to}, &out)
	return out.Results, err
}

func (s *Service) handleComms(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.staffOr401(w, r); !ok {
		return
	}
	resp := map[string]any{"catalogue": comms.Catalogue, "categories": comms.Categories, "not_sent": comms.NotSent,
		"coverage": comms.Coverage(), "log": comms.Snapshot{Recent: []comms.Delivery{}, Totals: map[string]map[string]int{}}}
	if s.Comms != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		snap, provider, err := s.Comms.Snapshot(ctx)
		if err != nil {
			log.Printf("comms log: %v", err)
			resp["log_unavailable"] = true
		} else {
			resp["log"], resp["email_provider"] = snap, provider
		}
	}
	writeJSON(w, 200, resp)
}

func (s *Service) handleCommsTest(w http.ResponseWriter, r *http.Request) {
	st, ok := s.staffOr401(w, r)
	if !ok {
		return
	}
	if !st.Has("lead") {
		fail(w, 403, "lead_required", "Only a safety lead can send test messages.")
		return
	}
	var in struct {
		Event string `json:"event"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&in)
	if _, ok := comms.Lookup(in.Event); !ok {
		fail(w, 404, "not_found", "Not found")
		return
	}
	if s.Comms == nil {
		fail(w, 503, "comms_unavailable", "Test sends aren't available on this server.")
		return
	}
	results, err := s.Comms.Test(r.Context(), in.Event, st.ID)
	if err != nil {
		log.Printf("comms test: %v", err)
		fail(w, 502, "comms_unavailable", "We couldn't reach the sending service. Try again in a minute.")
		return
	}
	s.mu.Lock()
	s.audit(r.Context(), st.ID, "comms_test", "", in.Event)
	s.mu.Unlock()
	writeJSON(w, 200, map[string]any{"results": results})
}
