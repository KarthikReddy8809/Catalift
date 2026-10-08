``` mermaid
sequenceDiagram
    participant Page as Step page in the Portal
    participant CP as ControlPlane
    participant R as Runner
    participant W as Worker (paused run)

    Note over W: a waitForExternalEvent step is AwaitingInput
    Page->>CP: runTask('some_lookup_task', params)<br/>POST /api/tasks/.../executions
    CP->>R: run and poll
    CP-->>Page: TaskExecution with output
    Page->>Page: operator reviews and submits the form
    Page->>CP: signal(payload)<br/>POST .../executions/{id}/signal
    CP->>W: raise event, run continues
```