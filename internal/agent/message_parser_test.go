package agent

import (
	"testing"
)

func TestParseOutgoingMessages_DirectMessage(t *testing.T) {
	response := `Here's my analysis.
<message to="developer">
I think we should use React for this component.
</message>
Let me know your thoughts.`

	messages := ParseOutgoingMessages(response)
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].To != "developer" {
		t.Errorf("expected To 'developer', got '%s'", messages[0].To)
	}
	if messages[0].Content != "I think we should use React for this component." {
		t.Errorf("unexpected content: %s", messages[0].Content)
	}
}

func TestParseOutgoingMessages_MultipleMessages(t *testing.T) {
	response := `<message to="agent1">First message</message>
Some text in between
<message to="agent2">Second message</message>`

	messages := ParseOutgoingMessages(response)
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].To != "agent1" {
		t.Error("first message should be to agent1")
	}
	if messages[1].To != "agent2" {
		t.Error("second message should be to agent2")
	}
}

func TestParseOutgoingMessages_Broadcast(t *testing.T) {
	response := `<broadcast>
This is for everyone!
</broadcast>`

	messages := ParseOutgoingMessages(response)
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].To != "*" {
		t.Errorf("expected To '*', got '%s'", messages[0].To)
	}
	if messages[0].Content != "This is for everyone!" {
		t.Errorf("unexpected content: %s", messages[0].Content)
	}
}

func TestParseOutgoingMessages_Mixed(t *testing.T) {
	response := `<message to="agent1">Direct</message>
<broadcast>Broadcast</broadcast>
<message to="agent2">Another direct</message>`

	messages := ParseOutgoingMessages(response)
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}
}

func TestParseOutgoingMessages_NoMessages(t *testing.T) {
	response := `This is just regular text without any message tags.`

	messages := ParseOutgoingMessages(response)
	if len(messages) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(messages))
	}
}

func TestContainsDoneSignal(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"<DONE/>", true},
		{"<DONE />", true},
		{"Some text <DONE/> more text", true},
		{"No done signal here", false},
		{"<done/>", false}, // Case sensitive
		{"DONE", false},
	}

	for _, tt := range tests {
		result := ContainsDoneSignal(tt.input)
		if result != tt.expected {
			t.Errorf("ContainsDoneSignal(%q) = %v, expected %v", tt.input, result, tt.expected)
		}
	}
}

func TestStripMessageTags(t *testing.T) {
	response := `Here's my final answer.
<message to="agent1">Some message</message>
<broadcast>A broadcast</broadcast>
The conclusion.
<DONE/>`

	result := StripMessageTags(response)
	expected := `Here's my final answer.
[To agent1]: Some message
[Broadcast]: A broadcast
The conclusion.`

	if result != expected {
		t.Errorf("StripMessageTags failed.\nGot:\n%s\n\nExpected:\n%s", result, expected)
	}
}

func TestExtractFinalOutput(t *testing.T) {
	conversation := []string{
		"First response <message to=\"other\">msg1</message>",
		"Second response <message to=\"other\">msg2</message>",
		"Final response with important content <DONE/>",
	}

	result := ExtractFinalOutput(conversation)
	expected := `First response [To other]: msg1

Second response [To other]: msg2

Final response with important content`

	if result != expected {
		t.Errorf("ExtractFinalOutput failed.\nGot:\n%s\n\nExpected:\n%s", result, expected)
	}
}

func TestExtractFinalOutput_Empty(t *testing.T) {
	result := ExtractFinalOutput([]string{})
	if result != "" {
		t.Errorf("expected empty string, got: %s", result)
	}
}

func TestParseOutgoingMessages_MultilineContent(t *testing.T) {
	response := `<message to="developer">
Here's a code example:

func hello() {
    fmt.Println("Hello")
}

Please review this.
</message>`

	messages := ParseOutgoingMessages(response)
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if !contains(messages[0].Content, "func hello()") {
		t.Error("multiline content not preserved")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
