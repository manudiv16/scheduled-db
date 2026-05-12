export interface RoadmapItem {
  id: string;
  status: 'past' | 'active' | 'future';
  title: string;
  quarter: string;
  features: string[];
}

export const roadmap: RoadmapItem[] = [
  {
    id: 'core',
    status: 'past',
    title: 'Core Engine',
    quarter: 'Q1',
    features: ['Raft Consensus', 'Timing Wheels', 'BoltDB Storage']
  },
  {
    id: 'dx',
    status: 'active',
    title: 'Developer Experience',
    quarter: 'NOW',
    features: ['REST API', 'Pre-built Binaries', 'Docker Compose']
  },
  {
    id: 'k8s',
    status: 'future',
    title: 'Kubernetes Native',
    quarter: 'Q3',
    features: ['K8s Operator', 'StatefulSet Auto-join', 'Metrics Auto-scaling']
  },
  {
    id: 'ecosystem',
    status: 'future',
    title: 'Ecosystem',
    quarter: 'Q4',
    features: ['Go Client', 'Python Client', 'Dashboard UI']
  }
];
