<script lang="ts">
  import { onMount } from 'svelte';
  import anime from 'animejs';

  const tabs = [
    { id: 'docker', label: 'Docker Compose' },
    { id: 'k8s', label: 'Kubernetes' },
    { id: 'binary', label: 'Local Binary' },
  ];

  const codeExamples: Record<string, string> = {
    docker: `# Start 3-node cluster with load balancer
make dev-up

# Create a test job
curl -X POST http://localhost:80/jobs \\
  -H "Content-Type: application/json" \\
  -d '{
    "type": "unico",
    "timestamp": "2024-12-25T10:00:00Z",
    "webhook_url": "https://webhook.site/your-id"
  }'

# Check cluster status
curl http://localhost:80/health | jq

# Stop cluster
make dev-down`,

    k8s: `# Deploy to Kubernetes
make k8s-deploy

# Check status
kubectl get pods -l app=scheduled-db

# Port forward for local access
kubectl port-forward svc/scheduled-db-api 8080:8080

# Create a recurring job
curl -X POST http://localhost:8080/jobs \\
  -H "Content-Type: application/json" \\
  -d '{
    "type": "recurrente",
    "cron_expression": "0 0 * * *",
    "webhook_url": "https://example.com/daily"
  }'`,

    binary: `# Build
make build

# Run single node
./scheduled-db --node-id=node-1

# Run with custom config
./scheduled-db \\
  --node-id=node-1 \\
  --http-port=8080 \\
  --raft-port=7000 \\
  --data-dir=./data \\
  --slot-gap=10s`,
  };

  let activeTab = $state('docker');
  let codeRef: HTMLElement;

  function switchTab(id: string) {
    if (id === activeTab) return;
    activeTab = id;

    if (codeRef) {
      anime({
        targets: codeRef,
        opacity: [0, 1],
        translateY: [10, 0],
        duration: 350,
        easing: 'easeOutCubic',
      });
    }
  }

  onMount(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            anime({
              targets: '.quickstart-wrapper',
              opacity: [0, 1],
              translateY: [30, 0],
              duration: 700,
              easing: 'easeOutCubic',
            });
            observer.disconnect();
          }
        });
      },
      { threshold: 0.15 }
    );

    const el = document.querySelector('.quickstart-wrapper');
    if (el) observer.observe(el);
  });
</script>

<div class="quickstart-wrapper" style="opacity: 0;">
  <div class="tabs">
    {#each tabs as tab}
      <button
        class="tab"
        class:active={activeTab === tab.id}
        onclick={() => switchTab(tab.id)}
      >
        {tab.label}
      </button>
    {/each}
  </div>
  <div class="code-block" bind:this={codeRef}>
    <pre><code>{codeExamples[activeTab]}</code></pre>
  </div>
</div>

<style>
  .quickstart-wrapper {
    max-width: 750px;
    margin: 0 auto;
  }

  .tabs {
    display: flex;
    gap: 0;
    border-bottom: 1px solid #334155;
  }

  .tab {
    padding: 0.65rem 1.25rem;
    font-size: 0.875rem;
    font-weight: 500;
    font-family: inherit;
    color: #64748b;
    cursor: pointer;
    border: 1px solid transparent;
    border-bottom: none;
    border-radius: 6px 6px 0 0;
    background: transparent;
    transition: all 0.2s;
  }

  .tab:hover {
    color: #94a3b8;
  }

  .tab.active {
    color: #22d3ee;
    background: #1e293b;
    border-color: #334155;
  }

  .code-block {
    border: 1px solid #334155;
    border-top: none;
    border-radius: 0 0 8px 8px;
    overflow: hidden;
  }

  pre {
    margin: 0;
    padding: 1.5rem;
    background: #1e293b;
    overflow-x: auto;
    font-size: 0.85rem;
    line-height: 1.7;
  }

  code {
    font-family: 'JetBrains Mono', 'Fira Code', monospace;
    color: #e2e8f0;
  }
</style>
