package model

import "time"

type Message struct {
	Role string    `json:"role"`
	Text string    `json:"text"`
	Time time.Time `json:"time"`
}

type Session struct {
	ID       string    `json:"id"`
	Harness  string    `json:"harness"`
	Project  string    `json:"project"`
	Path     string    `json:"path,omitempty"`
	Started  time.Time `json:"started"`
	Updated  time.Time `json:"updated"`
	Messages []Message `json:"messages,omitempty"`
}

func (s *Session) Touch(t time.Time) {
	if t.IsZero() {
		return
	}
	if s.Started.IsZero() || t.Before(s.Started) {
		s.Started = t
	}
	if s.Updated.IsZero() || t.After(s.Updated) {
		s.Updated = t
	}
}
