export type MilestoneStatus = 'completed' | 'in-progress' | 'planned';

export interface Milestone {
  date: string;
  title: string;
  description: string;
  status: MilestoneStatus;
}

export const timeline: Milestone[] = [
  {
    date: 'Q1 2025',
    title: 'Raft Consensus',
    description: 'HashiCorp Raft integration for strong consistency, leader election, and automatic failover across cluster nodes.',
    status: 'completed',
  },
  {
    date: 'Q1 2025',
    title: 'HTTP API',
    description: 'RESTful API with full CRUD operations for job management, health checks, and cluster status.',
    status: 'completed',
  },
  {
    date: 'Q2 2025',
    title: 'Job Scheduling',
    description: 'Unico (one-time) and Recurrente (cron-based) job types with time-slotted scheduling engine.',
    status: 'completed',
  },
  {
    date: 'Q2 2025',
    title: 'Worker System',
    description: 'Job execution engine with retry policies, dead letter queues, and concurrent job processing.',
    status: 'in-progress',
  },
  {
    date: 'Q3 2025',
    title: 'Web Dashboard',
    description: 'Real-time monitoring dashboard with cluster health metrics, job visualizations, and alerting.',
    status: 'planned',
  },
  {
    date: 'Q4 2025',
    title: 'Multi-Cluster',
    description: 'Cross-cluster replication and federation for global deployments with geo-aware scheduling.',
    status: 'planned',
  },
];
