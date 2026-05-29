package agent

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

type ConversationMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ConversationContext struct {
	ConversationID       string                `json:"conversationId"`
	Messages             []ConversationMessage `json:"messages,omitempty"`
	KnownFields          AgentKnownFields      `json:"knownFields"`
	MissingFields        []string              `json:"missingFields,omitempty"`
	LastFollowUpQuestion string                `json:"lastFollowUpQuestion,omitempty"`
}

type conversationStore struct {
	mu     sync.Mutex
	states map[string]conversationState
	ttl    time.Duration
}

type conversationState struct {
	context   ConversationContext
	updatedAt time.Time
}

func newConversationStore(ttl time.Duration) *conversationStore {
	return &conversationStore{
		states: map[string]conversationState{},
		ttl:    ttl,
	}
}

func (s *conversationStore) get(id string) (ConversationContext, bool) {
	if s == nil || id == "" {
		return ConversationContext{}, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.states[id]
	if !ok {
		return ConversationContext{}, false
	}
	if s.ttl > 0 && time.Since(state.updatedAt) > s.ttl {
		delete(s.states, id)
		return ConversationContext{}, false
	}

	return state.context, true
}

func (s *conversationStore) save(context ConversationContext) {
	if s == nil || context.ConversationID == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ttl > 0 {
		now := time.Now()
		for id, state := range s.states {
			if now.Sub(state.updatedAt) > s.ttl {
				delete(s.states, id)
			}
		}
	}

	s.states[context.ConversationID] = conversationState{
		context:   context,
		updatedAt: time.Now(),
	}
}

func newConversationID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err == nil {
		return hex.EncodeToString(bytes)
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
