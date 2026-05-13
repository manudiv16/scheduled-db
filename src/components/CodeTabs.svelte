<script lang="ts">
  import { Tabs } from "bits-ui";
  import { onMount } from 'svelte';
  import { fly, fade } from 'svelte/transition';

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

  let wrapperRef: HTMLDivElement;
  let visible = $state(false);

  onMount(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            visible = true;
            observer.disconnect();
          }
        });
      },
      { threshold: 0.15 }
    );
    observer.observe(wrapperRef);
    return () => observer.disconnect();
  });
</script>

<div bind:this={wrapperRef} class="mx-auto max-w-3xl">
  {#if visible}
    <div in:fly={{ y: 30, duration: 600 }}>
      <Tabs.Root value="docker" class="w-full">
        <Tabs.List class="flex w-full border-b border-brand-border">
          {#each tabs as tab}
            <Tabs.Trigger
              value={tab.id}
              class="flex-1 rounded-t-lg px-4 py-3 text-sm font-medium text-brand-text-muted transition-all hover:text-brand-text data-[state=active]:border-b-2 data-[state=active]:border-brand-cyan data-[state=active]:bg-brand-card data-[state=active]:text-brand-cyan"
            >
              {tab.label}
            </Tabs.Trigger>
          {/each}
        </Tabs.List>

        {#each tabs as tab}
          <Tabs.Content value={tab.id} class="rounded-b-lg border border-t-0 border-brand-border bg-brand-card">
            <pre class="m-0 overflow-x-auto p-5 text-sm leading-relaxed"><code class="bg-transparent p-0 text-brand-text-secondary">{codeExamples[tab.id]}</code></pre>
          </Tabs.Content>
        {/each}
      </Tabs.Root>
    </div>
  {/if}
</div>
