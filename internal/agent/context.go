package agent

import (
	"fmt"
	"strings"
	"time"
)

type AgentOutput struct {
	AgentID   string
	Response  string
	Timestamp time.Time
}

type ContextManager struct {
	History []AgentOutput
}

func NewContextManager() *ContextManager {
	return &ContextManager{
		History: []AgentOutput{},
	}
}

func (cm *ContextManager) AddOutput(agentID string, response string) {
	cm.History = append(cm.History, AgentOutput{
		AgentID:   agentID,
		Response:  response,
		Timestamp: time.Now(),
	})
}

func (cm *ContextManager) GetContext() string {
	if len(cm.History) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Context from previous agents:\n\n")

	for _, output := range cm.History {
		sb.WriteString(fmt.Sprintf("[%s]:\n%s\n\n", output.AgentID, output.Response))
	}

	return sb.String()
}

func (cm *ContextManager) GetLastOutput() string {
	if len(cm.History) == 0 {
		return ""
	}
	return cm.History[len(cm.History)-1].Response
}

func (cm *ContextManager) Clear() {
	cm.History = []AgentOutput{}
}
