```Orkflow/
├── cmd/
│   └── orka/              # CLI entry point (Cobra)
│       └── main.go
│
├── internal/
│   ├── cli/               # Cobra commands
│   │   ├── root.go        # Root command
│   │   ├── run.go         # `orka run workflow.yaml`
│   │   └── validate.go    # `orka validate workflow.yaml`
│   │
│   ├── engine/            # Core orchestration engine
│   │   ├── executor.go    # Runs workflows
│   │   └── state.go       # State machine logic
│   │
│   ├── parser/            # YAML parsing & validation
│   │   ├── parser.go
│   │   └── validator.go
│   │
│   └── agent/             # Agent execution logic
│       ├── agent.go
│       └── context.go     # Shared context between agents
│
├── pkg/                   # Shared/exportable packages
│   └── types/             # Domain models (Agent, Workflow, etc.)
│       └── types.go
│
├── examples/              # Example YAML workflows
│   ├── sequential.yaml
│   └── parallel.yaml
│
├── go.mod
├── go.sum
├── Makefile
└── README.md```
