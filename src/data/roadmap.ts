export interface RoadmapItem {
  id: string;
  status: 'done' | 'active' | 'planned';
  title: string;
  features: string[];
}

export const roadmap: RoadmapItem[] = [
  {
    id: 'core',
    status: 'done',
    title: 'Core Engine',
    features: ['Raft Consensus', 'Hierarchical Timing Wheels', 'BoltDB Storage', 'Job Types (unico & recurrente)']
  },
  {
    id: 'dx',
    status: 'active',
    title: 'Developer Experience',
    features: ['REST API', 'Pre-built Binaries', 'Docker Compose Setup', 'CLI Flags & Env Vars']
  },
  {
    id: 'k8s',
    status: 'planned',
    title: 'Kubernetes Native',
    features: ['K8s Operator (CRD)', 'StatefulSet Auto-join', 'Raft State Backup/Restore', 'Prometheus Auto-scaling']
  },
  {
    id: 'ecosystem',
    status: 'planned',
    title: 'Ecosystem',
    features: ['Go Client Library', 'Python Client Library', 'Admin Dashboard UI']
  }
];
