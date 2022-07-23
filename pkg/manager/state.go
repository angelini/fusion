package manager

import (
	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"
)

type NetLocation struct {
	Host string
	Port int
}

type nodeState struct {
	start    time.Time
	replicas []NetLocation
}

type State struct {
	log    *zap.Logger
	random *rand.Rand

	mutex sync.RWMutex
	nodes map[string]*nodeState
}

func NewState(log *zap.Logger) *State {
	return &State{
		log:    log,
		random: rand.New(rand.NewSource(time.Now().Unix())),
		nodes:  make(map[string]*nodeState),
	}
}

func (s *State) GetRoute(key string) *NetLocation {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	node, ok := s.nodes[key]
	if !ok {
		return nil
	}

	idx := s.random.Intn(len(node.replicas))
	return &node.replicas[idx]
}

func (s *State) AddRoute(key string, loc NetLocation) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	node, ok := s.nodes[key]
	if !ok {
		s.nodes[key] = &nodeState{
			start:    time.Now(),
			replicas: []NetLocation{loc},
		}
	}

	node.replicas = append(node.replicas, loc)
}
