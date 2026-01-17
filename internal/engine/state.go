package engine

type WorkflowState int

const (
	StatePending WorkflowState = iota
	StateRunning
	StateCompleted
	StateFailed
)

func (s WorkflowState) String() string {
	switch s {
	case StatePending:
		return "pending"
	case StateRunning:
		return "running"
	case StateCompleted:
		return "completed"
	case StateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

type State struct {
	Status      WorkflowState
	CurrentStep int
	TotalSteps  int
	Error       error
}

func NewState(totalSteps int) *State {
	return &State{
		Status:      StatePending,
		CurrentStep: 0,
		TotalSteps:  totalSteps,
	}
}

func (s *State) Start() {
	s.Status = StateRunning
}

func (s *State) Complete() {
	s.Status = StateCompleted
}

func (s *State) Fail(err error) {
	s.Status = StateFailed
	s.Error = err
}

func (s *State) NextStep() {
	s.CurrentStep++
}

func (s *State) IsRunning() bool {
	return s.Status == StateRunning
}
