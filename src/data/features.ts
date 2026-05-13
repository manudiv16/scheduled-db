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

export const apiEndpoints = [
  {
    method: 'POST',
    path: '/jobs',
    desc: 'Create a new job (unico or recurrente)',
    example: '{"type":"unico", "action":"webhook", "target":"http://..."}'
  },
  {
    method: 'GET',
    path: '/jobs/{id}',
    desc: 'Retrieve job details and state',
    example: '{"id":"job-123", "status":"pending"}'
  },
  {
    method: 'DELETE',
    path: '/jobs/{id}/cancel',
    desc: 'Cancel a pending or recurring job',
    example: '{"status":"cancelled"}'
  },
  {
    method: 'GET',
    path: '/jobs/{id}/executions',
    desc: 'List execution history for a job',
    example: '[{"status":"success", "timestamp":"2026-05-13T10:00:00Z"}]'
  },
  {
    method: 'GET',
    path: '/metrics',
    desc: 'Prometheus metrics endpoint',
    example: '# HELP scheduled_db_jobs_total...'
  }
];
