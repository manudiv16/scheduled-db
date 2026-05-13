export const apiEndpoints = [
  { method: 'POST', path: '/jobs', description: 'Create job' },
  { method: 'GET', path: '/jobs/{id}', description: 'Get job details' },
  { method: 'DELETE', path: '/jobs/{id}', description: 'Delete job' },
  { method: 'GET', path: '/jobs/{id}/status', description: 'Get job execution status' },
  { method: 'GET', path: '/jobs/{id}/executions', description: 'Get execution history' },
  { method: 'GET', path: '/jobs?status={status}', description: 'List jobs by status' },
  { method: 'POST', path: '/jobs/{id}/cancel', description: 'Cancel job' },
  { method: 'GET', path: '/health', description: 'Health check' },
  { method: 'GET', path: '/debug/cluster', description: 'Cluster info' },
  { method: 'POST', path: '/join', description: 'Join cluster' },
];

export type FeatureCategory = 'consensus' | 'scheduling' | 'storage' | 'ops';

export interface Feature {
  id: string;
  title: string;
  description: string;
  category: FeatureCategory;
  size: 'sm' | 'md' | 'lg';
}

export const features: Feature[] = [
  {
    id: 'raft',
    title: 'Distributed Consensus',
    description: 'HashiCorp Raft implementation for CP consistency, automated leader election, and robust log replication across the cluster.',
    category: 'consensus',
    size: 'lg'
  },
  {
    id: 'split-brain',
    title: 'Split-Brain Prevention',
    description: 'RA-style quorum requirements prevent minority partitions from accepting writes.',
    category: 'consensus',
    size: 'sm'
  },
  {
    id: 'timing-wheel',
    title: 'Hierarchical Timing Wheel',
    description: 'O(1) scheduling via bounded hierarchical wheels. Zero heap allocations during execution.',
    category: 'scheduling',
    size: 'lg'
  },
  {
    id: 'job-types',
    title: 'Dual Job Types',
    description: 'Support for immediate execution (unico) and cron-expression based recurring (recurrente) jobs.',
    category: 'scheduling',
    size: 'sm'
  },
  {
    id: 'cold-spilling',
    title: 'Cold Storage Spilling',
    description: 'Automatically spills distant future slots to BoltDB to conserve memory, reloading them as they approach.',
    category: 'storage',
    size: 'md'
  },
  {
    id: 'persistence',
    title: 'Disk Persistence',
    description: 'All Raft logs, snapshots, and cold storage are backed by BoltDB.',
    category: 'storage',
    size: 'sm'
  },
  {
    id: 'rest-api',
    title: 'REST API',
    description: 'JSON API for cluster management, job scheduling, and status tracking.',
    category: 'ops',
    size: 'sm'
  },
  {
    id: 'service-discovery',
    title: 'Service Discovery',
    description: 'Multi-strategy peer discovery: Kubernetes, DNS, Gossip, or static configuration.',
    category: 'ops',
    size: 'sm'
  },
  {
    id: 'health-checks',
    title: 'Tiered Health Checks',
    description: 'Deep health endpoints verify Raft status, disk I/O, and queue latency.',
    category: 'ops',
    size: 'sm'
  }
];
