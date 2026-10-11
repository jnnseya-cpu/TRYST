package profilesvc

import (
	"context"
	"errors"
	"log"
)

var errNoPhoto = errors.New("no photo")

// Standing is the enforcement state the safety service applies (03 §8; ToS cl. 17).
//   - suspend, permanent: the member can't use the platform; their conversations close and
//     they disappear from Tonight.
//   - shadow_limit: the member keeps their conversations but isn't shown in Tonight.
//   - warn, good: no restriction.

func removed(standing string) bool { return standing == "suspend" || standing == "permanent" }

// SetStanding records an enforcement outcome. Called by the safety service.
func (s *Service) SetStanding(account, standing string) {
	s.mu.Lock()
	if standing == "good" || standing == "warn" {
		delete(s.standing, account)
	} else {
		s.standing[account] = standing
	}
	s.mu.Unlock()
	var err error
	if standing == "good" || standing == "warn" {
		err = s.st.Delete(context.Background(), "standing", account)
	} else {
		err = s.st.Put(context.Background(), "standing", account, []byte(`"`+standing+`"`))
	}
	if err != nil {
		log.Printf("persist standing failed: %v", err)
	}
}

func (s *Service) loadStanding(ctx context.Context) error {
	docs, err := s.st.All(ctx, "standing")
	if err != nil {
		return err
	}
	for id, b := range docs {
		if len(b) > 2 {
			s.standing[id] = string(b[1 : len(b)-1])
		}
	}
	return nil
}

// Counterpart returns the other member of a thread the caller belongs to.
func (s *Service) Counterpart(me, threadID string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.th.threads[threadID]
	if !ok || !t.has(me) {
		return "", false
	}
	return t.other(me), true
}

// Summary is a member's display name and city (never the exact area) for the console.
func (s *Service) Summary(account string) (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.data[account]
	if p == nil {
		return "", ""
	}
	area := ""
	if a, ok := areaByID(p.AreaID); ok {
		area = a.Label(true)
	}
	return p.DisplayName, area
}

// MainPhoto returns a member's main photo for a moderator who recorded a reason.
func (s *Service) MainPhoto(ctx context.Context, account string) ([]byte, string, error) {
	s.mu.Lock()
	p := s.data[account]
	var main *photo
	if p != nil {
		for _, ph := range p.Photos {
			if ph.Main {
				main = ph
			}
		}
		if main == nil && len(p.Photos) > 0 {
			main = p.Photos[0]
		}
	}
	s.mu.Unlock()
	if main == nil {
		return nil, "", errNoPhoto
	}
	b, err := s.photoBytes(ctx, account, main)
	return b, main.ContentType, err
}
