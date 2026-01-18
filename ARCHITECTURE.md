# Orkflow Architecture

## System Overview

Orkflow is a YAML-driven orchestration engine that transforms declarative agent configurations into executable multi-agent workflows.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              CLI (Cobra)                                     │
│                    orka run · orka validate · orka sessions                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐                   │
│  │    Parser    │───▶│   Executor   │───▶│ Agent Runner │                   │
│  │  (YAML → Go) │    │ (Orchestrate)│    │  (LLM Calls) │                   │
│  └──────────────┘    └──────┬───────┘    └──────┬───────┘                   │
│                              │                   │                           │
│                     ┌────────▼────────┐  ┌──────▼───────┐                   │
│                     │  Shared Memory  │  │    Tools     │                   │
│                     │ (Pub/Sub Store) │  │ calc·file·mcp│                   │
│                     └─────────────────┘  └──────────────┘                   │
│                                                                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                           LLM Providers                                      │
│            OpenAI  ·  Gemini  ·  Anthropic  ·  Ollama  ·  Generic           │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Directory Structure

```
Orkflow/
├── cmd/orka/main.go           # CLI entrypoint
│
├── internal/
│   ├── cli/                   # CLI commands (Cobra)
│   │   ├── root.go            # Global flags, config
│   │   ├── run.go             # orka run command
│   │   ├── validate.go        # orka validate command
│   │   ├── sessions.go        # orka sessions command
│   │   ├── completion.go      # Shell completions
│   │   └── ui.go              # Colors, progress bar, emojis
│   │
│   ├── parser/                # YAML parsing
│   │   └── yaml.go            # Parse + validate workflow configs
│   │
│   ├── engine/                # Workflow orchestration
│   │   ├── executor.go        # Sequential/parallel execution
│   │   ├── state.go           # Execution state tracking
│   │   └── stats.go           # Time/cost tracking
│   │
│   ├── agent/                 # LLM interaction
│   │   ├── agent.go           # Agent runner, prompt builder
│   │   ├── collaborative.go   # Real-time messaging mode
│   │   ├── llm.go             # Client factory
│   │   ├── openai.go          # OpenAI client
│   │   ├── gemini.go          # Google Gemini client
│   │   ├── anthropic.go       # Anthropic Claude client
│   │   ├── ollama.go          # Ollama (local) client
│   │   └── generic.go         # OpenAI-compatible APIs
│   │
│   ├── tools/                 # Built-in tools
│   │   ├── registry.go        # Tool registration
│   │   ├── executor.go        # Tool call parsing
│   │   ├── calc.go            # Math expressions
│   │   ├── file.go            # Filesystem operations
│   │   └── script.go          # Tengo script execution
│   │
│   ├── mcp/                   # Model Context Protocol
│   │   ├── client.go          # MCP server connection
│   │   └── tools.go           # MCP → Tool adapter
│   │
│   ├── memory/                # Data storage
│   │   ├── session.go         # Persistent session store
│   │   ├── shared.go          # Inter-agent shared memory
│   │   └── channel.go         # Real-time message channel
│   │
│   ├── logging/               # Execution logs
│   │   └── logger.go          # File-based logger
│   │
│   └── vectorstore/           # Semantic search (optional)
│       └── chromem.go         # ChromaDB integration
│
├── pkg/types/                 # Shared types
│   ├── agent.go               # Agent struct
│   └── config.go              # WorkflowConfig struct
│
└── examples/                  # Example workflows
```

---

## Core Components

### 1. Parser (`internal/parser/yaml.go`)

**Purpose:** Validate and parse YAML workflow files into Go structs.

```go
func Parse(filename string) (*types.WorkflowConfig, error)
```

**Validates:**
- Required fields (agents, workflow)
- Agent IDs match workflow references
- Workflow type is valid (sequential/parallel)
- Model references exist

---

### 2. Executor (`internal/engine/executor.go`)

**Purpose:** Orchestrate agent execution based on workflow type.

```go
type Executor struct {
    Config       *types.WorkflowConfig
    Runner       *agent.Runner
    State        *State
    SharedMemory *memory.SharedMemory
    MCPClient    *mcp.Client
    Stats        *ExecutionStats
}
```

**Execution Modes:**

| Mode | Method | Behavior |
|------|--------|----------|
| Sequential | `executeSequential()` | Agents run one after another |
| Parallel | `executeParallel()` | Agents run concurrently, then aggregate |
| Collaborative | `RunCollaborativeAgent()` | Agents chat in real-time |

---

### 3. Agent Runner (`internal/agent/agent.go`)

**Purpose:** Execute individual agents via LLM clients.

```go
func (r *Runner) RunAgent(agentDef *types.Agent) (string, error)
```

**Features:**
- Automatic retry with exponential backoff
- Tool call detection and execution
- Shared memory publish/subscribe
- Logging integration

---

### 4. Collaborative Mode (`internal/agent/collaborative.go`)

**Purpose:** Enable real-time agent-to-agent messaging.

```go
func (r *Runner) RunCollaborativeAgent(agentDef *types.Agent, channel *memory.MessageChannel) (string, error)
```

**Flow:**
1. Agent subscribes to message channel
2. Loop for `max_turns`:
   - Collect incoming messages
   - Generate LLM response
   - Parse outgoing `<message>` tags
   - Send messages to channel
   - Check for `<DONE/>` signal
3. Return final output

---

### 5. Shared Memory (`internal/memory/shared.go`)

**Purpose:** Pub/sub data store for inter-agent communication.

```go
type SharedMemory struct {
    data    map[string]interface{}
    waiters map[string][]chan struct{}
}
```

**Methods:**
- `Set(key, value)` - Publish data
- `Get(key)` - Retrieve data
- `WaitFor(key)` - Block until data available

**YAML Usage:**
```yaml
outputs: [research_notes]   # Publish
requires: [research_notes]  # Subscribe/wait
```

---

### 6. Message Channel (`internal/memory/channel.go`)

**Purpose:** Real-time message passing for collaborative agents.

```go
type MessageChannel struct {
    messages    []ChannelMessage
    subscribers map[string]chan ChannelMessage
}
```

**Methods:**
- `Send(from, to, content)` - Send message
- `Subscribe(agentID)` - Get inbox channel
- `GetMessagesFor(agentID)` - Get messages for agent

---

### 7. Tools (`internal/tools/`)

**Purpose:** Extend agent capabilities.

| Tool | File | Description |
|------|------|-------------|
| `calc` | `calc.go` | Evaluate math expressions |
| `file` | `file.go` | Read/write/list files |
| `script` | `script.go` | Run Tengo scripts |

**Tool Interface:**
```go
type Tool interface {
    Name() string
    Description() string
    Execute(input string) (string, error)
}
```

---

### 8. MCP Client (`internal/mcp/client.go`)

**Purpose:** Connect to external tool servers via Model Context Protocol.

```yaml
mcp_servers:
  filesystem:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]

agents:
  - id: dev
    toolsets: [filesystem]
```

---

### 9. LLM Providers (`internal/agent/*.go`)

| Provider | File | Models |
|----------|------|--------|
| OpenAI | `openai.go` | gpt-4o, gpt-4o-mini, gpt-3.5-turbo |
| Google | `gemini.go` | gemini-2.0-flash, gemini-1.5-pro |
| Anthropic | `anthropic.go` | claude-3-sonnet |
| Ollama | `ollama.go` | llama3, mistral, etc. |
| Generic | `generic.go` | Any OpenAI-compatible API |

---

## Data Flow

### Sequential Workflow

```
┌─────────┐    context    ┌─────────┐    context    ┌─────────┐
│ Agent 1 │──────────────▶│ Agent 2 │──────────────▶│ Agent 3 │
└─────────┘               └─────────┘               └─────────┘
     │                         │                         │
     ▼                         ▼                         ▼
┌─────────────────────────────────────────────────────────────┐
│                      Shared Memory                           │
└─────────────────────────────────────────────────────────────┘
```

### Parallel Workflow

```
     ┌─────────┐
     │ Agent A │────┐
     └─────────┘    │
                    ▼
     ┌─────────┐  ┌───────────┐  ┌────────────┐
     │ Agent B │──│ Wait All  │─▶│ Aggregator │
     └─────────┘  └───────────┘  └────────────┘
                    ▲
     ┌─────────┐    │
     │ Agent C │────┘
     └─────────┘
```

### Collaborative Workflow

```
┌─────────┐         MessageChannel         ┌─────────┐
│ Agent A │◀──────────────────────────────▶│ Agent B │
└────┬────┘   <message to="B">content</msg>└────┬────┘
     │                                          │
     └──────────────────┬───────────────────────┘
                        ▼
                   ┌─────────┐
                   │ <DONE/> │
                   └─────────┘
```

---

## Configuration Types

### Agent Definition (`pkg/types/agent.go`)

```go
type Agent struct {
    ID           string
    Role         string
    Goal         string
    Model        string
    Tools        []string
    Toolsets     []string
    Outputs      []string
    Requires     []string
    ListensTo    []string   // Collaborative mode
    CanBroadcast bool       // Allow <broadcast> messages
    MaxTurns     int        // Max collaborative turns
    SubAgents    []string   // Supervisor pattern
}
```

### Workflow Definition (`pkg/types/config.go`)

```go
type Workflow struct {
    Type     string   // "sequential" | "parallel"
    Steps    []Step   // For sequential
    Branches []string // For parallel
    Then     *Step    // Aggregator after parallel
    MaxTurns int      // For collaborative
}
```

---

## Session Persistence

Sessions are stored in `~/.orka/sessions/` as JSON:

```json
{
  "id": "8d6ddfb2",
  "workflow": "examples/sequential-workflow.yaml",
  "created_at": "2026-01-18T10:30:00Z",
  "messages": [
    {"agent": "researcher", "role": "assistant", "content": "..."},
    {"agent": "writer", "role": "assistant", "content": "..."}
  ]
}
```

**Commands:**
- `orka sessions list` - List all sessions
- `orka sessions show <id>` - View session details
- `orka run --continue` - Resume last session

---

## Error Handling

| Error Type | Handling |
|------------|----------|
| API failures | 3 retries with exponential backoff |
| Invalid YAML | Validation errors with line numbers |
| Missing API keys | Prompt for key, offer to save |
| Quota exceeded | Helpful suggestion box |
| Agent timeout | Configurable timeout per agent |

---

## Extensibility Points

1. **New LLM Provider:** Implement `LLMClient` interface in `internal/agent/`
2. **New Tool:** Implement `Tool` interface in `internal/tools/`
3. **New Workflow Pattern:** Extend `executor.go`
4. **External Tools:** Add MCP server configuration
