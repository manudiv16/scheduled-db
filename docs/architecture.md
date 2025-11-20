# Scheduled-DB Architecture

## Overview

Scheduled-DB is a distributed job scheduling system built on Raft consensus. It provides reliable, fault-tolerant job execution across a cluster of nodes with automatic leader election and failover.

## High-Level Architecture

```mermaid
graph TB
    subgraph "Client Layer"
        API[HTTP API Clients]
    end
    
    subgraph "Cluster"
        subgraph "Node 1 - Leader"
            HTTP1[HTTP Server :8080]
            APP1[Application]
            WORKER1[Worker]
            QUEUE1[Slot Queue]
            FSM1[FSM State]
            RAFT1[Raft Leader]
            STORE1[(BoltDB)]
        end
        
        subgraph "Node 2 - Follower"
            HTTP2[HTTP Server :8080]
            APP2[Application]
            RAFT2[Raft Follower]
            FSM2[FSM State]
            STORE2[(BoltDB)]
        end
        
        subgraph "Node 3 - Follower"
            HTTP3[HTTP Server :8080]
            APP3[Application]
            RAFT3[Raft Follower]
            FSM3[FSM State]
            STORE3[(BoltDB)]
        end
    end
    
    subgraph "External Services"
        WEBHOOK[Webhook Endpoints]
        METRICS[Prometheus/OTLP]
    end
    
    API -->|POST /jobs| HTTP1
    API -->|POST /jobs| HTTP2
    API -->|POST /jobs| HTTP3
    
    HTTP2 -.->|Proxy to Leader| HTTP1
    HTTP3 -.->|Proxy to Leader| HTTP1
    
    HTTP1 --> APP1
    APP1 --> RAFT1
    RAFT1 --> FSM1
    FSM1 --> STORE1
    
    APP1 --> QUEUE1
    QUEUE1 --> WORKER1
    WORKER1 -->|Execute Jobs| WEBHOOK
    
    RAFT1 <-->|Consensus| RAFT2
    RAFT1 <-->|Consensus| RAFT3
    RAFT2 <-->|Replication| RAFT3
    
    RAFT2 --> FSM2
    RAFT3 --> FSM3
    FSM2 --> STORE2
    FSM3 --> STORE3
    
    APP1 -->|Metrics| METRICS
    APP2 -->|Metrics| METRICS
    APP3 -->|Metrics| METRICS
    
    style WORKER1 fill:#90EE90
    style RAFT1 fill:#FFD700
    style RAFT2 fill:#87CEEB
    style RAFT3 fill:#87CEEB
```

## Core Components

### 1. HTTP API Layer

**Location:** `internal/api/`

Handles incoming HTTP requests and provides REST endpoints:

- `POST /jobs` - Create new job
- `GET /jobs/{id}` - Retrieve job details
- `DELETE /jobs/{id}` - Delete job
- `GET /health` - Health check
- `POST /join` - Join cluster
- `GET /debug/cluster` - Cluster information

**Key Features:**
- Automatic proxy to leader for write operations
- CORS support
- Request validation
- Metrics instrumentation

### 2. Raft Consensus Layer

**Location:** `internal/store/`

Implements distributed consensus using HashiCorp Raft:

```mermaid
sequenceDiagram
    participant Client
    participant Follower
    participant Leader
    participant FSM
    participant BoltDB
    
    Client->>Follower: POST /jobs
    Follower->>Leader: Proxy Request
    Leader->>Leader: Validate Job
    Leader->>Raft: Apply Command
    Raft->>FSM: Apply Log Entry
    FSM->>FSM: Update State
    FSM->>BoltDB: Persist
    Raft-->>Followers: Replicate Log
    Leader->>Client: 200 OK
```

**Components:**
- **Store**: Raft wrapper and cluster management
- **FSM (Finite State Machine)**: State transitions for jobs and slots
- **Transport**: Network communication between nodes
- **Snapshot**: State persistence and recovery

### 3. Job Scheduling Layer

**Location:** `internal/slots/`

Organizes jobs into time-based slots for efficient execution:

```mermaid
graph LR
    subgraph "Slot Queue"
        S0[Slot 0<br/>0-9s]
        S1[Slot 1<br/>10-19s]
        S2[Slot 2<br/>20-29s]
        S3[Slot 3<br/>30-39s]
    end
    
    subgraph "Jobs"
        J1[Job 1<br/>t=5s]
        J2[Job 2<br/>t=15s]
        J3[Job 3<br/>t=15s]
        J4[Job 4<br/>t=25s]
    end
    
    J1 --> S0
    J2 --> S1
    J3 --> S1
    J4 --> S2
    
    S0 -->|Ready| WORKER[Worker]
    S1 -.->|Waiting| WORKER
    S2 -.->|Waiting| WORKER
    S3 -.->|Empty| WORKER
```

**Components:**
- **SlotQueue**: Min-heap based priority queue
- **PersistentSlotQueue**: Raft-backed slot persistence
- **Worker**: Job execution engine (leader-only)

### 4. Service Discovery

**Location:** `internal/discovery/`

Multiple strategies for cluster formation:

```mermaid
graph TB
    DM[Discovery Manager]
    
    subgraph "Strategies"
        STATIC[Static<br/>Peer List]
        K8S[Kubernetes<br/>API]
        DNS[DNS<br/>SRV Records]
        GOSSIP[Gossip<br/>Memberlist]
    end
    
    DM --> STATIC
    DM --> K8S
    DM --> DNS
    DM --> GOSSIP
    
    STATIC -->|Join| RAFT[Raft Cluster]
    K8S -->|Join| RAFT
    DNS -->|Join| RAFT
    GOSSIP -->|Join| RAFT
```

## Data Flow

### Job Creation Flow

```mermaid
sequenceDiagram
    participant Client
    participant API
    participant Store
    participant Raft
    participant FSM
    participant Queue
    participant Worker
    
    Client->>API: POST /jobs
    API->>API: Validate Request
    API->>Store: CreateJob()
    Store->>Store: Check IsLeader()
    Store->>Raft: Apply(CreateJob)
    Raft->>FSM: Apply(LogEntry)
    FSM->>FSM: jobs[id] = job
    FSM-->>Store: Return Job
    Store->>Queue: AddJob(job)
    Queue->>Queue: Calculate Slot
    Queue->>Queue: Add to Heap
    Store-->>API: Success
    API-->>Client: 200 OK
    
    Note over Worker: Polls every 1s
    Worker->>Queue: GetNextSlot()
    Queue-->>Worker: Slot with Jobs
    Worker->>Worker: Check Time
    Worker->>Worker: Execute Job
    Worker->>Webhook: POST webhook
```

### Job Execution Flow

```mermaid
stateDiagram-v2
    [*] --> Created: Job Created
    Created --> Queued: Added to Slot
    Queued --> Ready: Time Reached
    Ready --> Executing: Worker Picks Up
    Executing --> Completed: Success
    Executing --> Failed: Error
    Completed --> [*]
    Failed --> [*]
    
    note right of Queued
        Job waits in slot
        until execution time
    end note
    
    note right of Executing
        Only leader executes
        Webhook called if configured
    end note
```

### Recurring Job Flow

```mermaid
graph TD
    START[Job Created] --> QUEUE[Add to Queue]
    QUEUE --> WAIT[Wait for Time]
    WAIT --> EXEC[Execute Job]
    EXEC --> CALC[Calculate Next Run]
    CALC --> CHECK{Has last_date?}
    CHECK -->|No| RESCHEDULE[Reschedule Job]
    CHECK -->|Yes| COMPARE{Next > last_date?}
    COMPARE -->|No| RESCHEDULE
    COMPARE -->|Yes| DELETE[Delete Job]
    RESCHEDULE --> QUEUE
    DELETE --> END[Job Completed]
```

## State Management

### FSM State Structure

```mermaid
classDiagram
    class FSM {
        +map~string,Job~ jobs
        +map~int64,SlotData~ slots
        +Apply(logEntry) interface
        +Snapshot() FSMSnapshot
        +Restore(reader) error
        +GetJob(id) Job
        +GetAllJobs() map
    }
    
    class Job {
        +string ID
        +JobType Type
        +int64 Timestamp
        +string CronExpr
        +int64 LastDate
        +int64 CreatedAt
        +string WebhookURL
        +map Payload
        +Validate() error
    }
    
    class SlotData {
        +int64 Key
        +int64 MinTime
        +int64 MaxTime
        +[]string JobIDs
    }
    
    FSM --> Job: manages
    FSM --> SlotData: manages
```

### Snapshot Format

```json
{
  "jobs": {
    "job-id-1": {
      "id": "job-id-1",
      "type": "unico",
      "timestamp": 1704067200,
      "created_at": 1704060000,
      "webhook_url": "https://example.com/webhook"
    }
  },
  "slots": {
    "100": {
      "key": 100,
      "min_time": 1000,
      "max_time": 1099,
      "job_ids": ["job-id-1", "job-id-2"]
    }
  }
}
```

## Leadership and Failover

### Leader Election

```mermaid
sequenceDiagram
    participant N1 as Node 1
    participant N2 as Node 2
    participant N3 as Node 3
    
    Note over N1,N3: Initial State: No Leader
    
    N1->>N1: Election Timeout
    N1->>N2: RequestVote
    N1->>N3: RequestVote
    N2->>N1: Vote Granted
    N3->>N1: Vote Granted
    N1->>N1: Become Leader
    
    Note over N1: Start Worker
    Note over N1: Load Jobs into Queue
    
    N1->>N2: Heartbeat
    N1->>N3: Heartbeat
    N2->>N1: ACK
    N3->>N1: ACK
```

### Graceful Leader Resignation

```mermaid
sequenceDiagram
    participant Leader
    participant Follower1
    participant Follower2
    
    Note over Leader: Receives SIGTERM
    
    Leader->>Leader: Stop Worker
    Leader->>Leader: RemovePeer(self)
    Leader->>Follower1: Update Config
    Leader->>Follower2: Update Config
    
    Note over Leader: Wait 2s
    
    Leader->>Leader: LeadershipTransfer()
    
    Note over Follower1,Follower2: Election Starts
    
    Follower1->>Follower1: Become Leader
    Follower1->>Follower1: Start Worker
    
    Note over Leader: Shutdown Complete
```

## Deployment Architectures

### Docker Compose (Development)

```mermaid
graph TB
    subgraph "Docker Network"
        NGINX[Nginx LB<br/>:80]
        
        subgraph "Node 1"
            N1[scheduled-db<br/>:8080]
        end
        
        subgraph "Node 2"
            N2[scheduled-db<br/>:8081]
        end
        
        subgraph "Node 3"
            N3[scheduled-db<br/>:8082]
        end
        
        PROM[Prometheus<br/>:9090]
        GRAF[Grafana<br/>:3000]
    end
    
    CLIENT[Client] --> NGINX
    NGINX --> N1
    NGINX --> N2
    NGINX --> N3
    
    N1 -.->|Metrics| PROM
    N2 -.->|Metrics| PROM
    N3 -.->|Metrics| PROM
    
    GRAF --> PROM
```

### Kubernetes (Production)

```mermaid
graph TB
    subgraph "Kubernetes Cluster"
        subgraph "StatefulSet"
            POD0[scheduled-db-0<br/>Leader]
            POD1[scheduled-db-1<br/>Follower]
            POD2[scheduled-db-2<br/>Follower]
        end
        
        subgraph "Services"
            HEADLESS[scheduled-db<br/>Headless Service]
            API[scheduled-db-api<br/>LoadBalancer]
        end
        
        subgraph "Storage"
            PVC0[PVC 1Gi]
            PVC1[PVC 1Gi]
            PVC2[PVC 1Gi]
        end
        
        subgraph "Monitoring"
            SM[ServiceMonitor]
            PROM[Prometheus]
        end
    end
    
    POD0 --> PVC0
    POD1 --> PVC1
    POD2 --> PVC2
    
    HEADLESS --> POD0
    HEADLESS --> POD1
    HEADLESS --> POD2
    
    API --> POD0
    API --> POD1
    API --> POD2
    
    SM --> POD0
    SM --> POD1
    SM --> POD2
    SM --> PROM
```

## Performance Characteristics

### Slot-Based Scheduling

- **Slot Gap**: Configurable (default 10s)
- **Slot Calculation**: `slot_key = timestamp / slot_gap`
- **Lookup Complexity**: O(log n) using min-heap
- **Memory**: O(jobs + slots)

### Raft Performance

- **Write Latency**: ~5-10ms (local network)
- **Replication**: Majority quorum (2/3 nodes)
- **Snapshot Frequency**: Configurable
- **Log Compaction**: Automatic

### Scalability

- **Horizontal**: Add more follower nodes
- **Vertical**: Increase resources per node
- **Job Throughput**: ~1000 jobs/sec (single leader)
- **Cluster Size**: Recommended 3-7 nodes

## Security Considerations

### Network Security

- TLS for Raft communication (configurable)
- RBAC for Kubernetes deployments
- Network policies for pod isolation

### Data Security

- Encrypted snapshots (optional)
- Webhook authentication (bearer tokens)
- Audit logging for job operations

### High Availability

- Minimum 3 nodes for quorum
- PodDisruptionBudget in Kubernetes
- Graceful leader resignation
- Split-brain prevention

## Monitoring and Observability

### Metrics Exposed

- Job creation/deletion rates
- Job execution success/failure
- Raft cluster health
- Leader election events
- Slot queue size
- Worker processing time

### Health Endpoints

- `/health` - Node health and role
- `/debug/cluster` - Cluster configuration
- `/metrics` - Prometheus metrics (port 9090)

### Logging Levels

- **DEBUG**: Internal operations, state changes
- **INFO**: Important events, job lifecycle
- **WARN**: Potential issues, degraded performance
- **ERROR**: Failures, critical errors

## References

- [Raft Consensus Algorithm](https://raft.github.io/)
- [HashiCorp Raft Implementation](https://github.com/hashicorp/raft)
- [Kubernetes StatefulSets](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/)
