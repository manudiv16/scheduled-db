<script lang="ts">
  import SectionHeader from '../ui/SectionHeader.svelte';
  import TerminalWindow from '../ui/TerminalWindow.svelte';
  import HudButton from '../ui/HudButton.svelte';

  let activeTab = $state('docker');

  const content = {
    docker: {
      cmd: 'make dev-up',
      output: [
        'Building docker images...',
        'Creating network scheduled-db_default',
        'Creating container node-01 ... done',
        'Creating container node-02 ... done',
        'Creating container node-03 ... done',
        '[ node-01 ] [INFO] raft: cluster leadership acquired',
        '[ node-02 ] [INFO] raft: joining cluster',
        'Cluster ready. UI accessible at http://localhost:80'
      ]
    },
    k8s: {
      cmd: 'make k8s-deploy',
      output: [
        'kubectl apply -k k8s/',
        'namespace/scheduled-db created',
        'service/scheduled-db created',
        'statefulset.apps/scheduled-db created',
        'Waiting for pods to be ready...',
        'pod/scheduled-db-0 condition met',
        'pod/scheduled-db-1 condition met',
        'pod/scheduled-db-2 condition met'
      ]
    },
    local: {
      cmd: 'make build && ./scheduled-db --node-id=node-local',
      output: [
        'go build -o scheduled-db cmd/scheduled-db/main.go',
        'Starting Scheduled-DB node-local',
        '[INFO] Loaded configuration',
        '[INFO] raft: initializing single node cluster',
        '[INFO] HTTP API listening on :8080'
      ]
    }
  };

  let currentContent = $derived(content[activeTab as keyof typeof content]);
</script>

<section id="quickstart" class="container quickstart-section">
  <SectionHeader 
    code="SYS-04" 
    title="INITIALIZATION SEQUENCE" 
    subtitle="Deploy the cluster in your preferred environment"
  />

  <div class="terminal-wrapper">
    <div class="tab-controls">
      <HudButton variant={activeTab === 'docker' ? 'primary' : 'ghost'} onclick={() => activeTab = 'docker'}>DOCKER COMPOSE</HudButton>
      <HudButton variant={activeTab === 'k8s' ? 'primary' : 'ghost'} onclick={() => activeTab = 'k8s'}>KUBERNETES</HudButton>
      <HudButton variant={activeTab === 'local' ? 'primary' : 'ghost'} onclick={() => activeTab = 'local'}>LOCAL BINARY</HudButton>
    </div>

    <TerminalWindow title="operator@cluster ~ %">
      <div class="cmd-line">
        <span class="prompt">$</span>
        <span class="command">{currentContent.cmd}</span>
      </div>
      <div class="output-lines">
        {#each currentContent.output as line}
          <div class="out-line text-[var(--ink-secondary)]">{line}</div>
        {/each}
        <div class="cursor-line">
          <span class="cursor anim-blink">▍</span>
        </div>
      </div>
    </TerminalWindow>
  </div>
</section>

<style>
  .quickstart-section {
    padding: var(--spacing-2xl) 0;
  }

  .terminal-wrapper {
    max-width: 800px;
    margin: 0 auto;
    display: flex;
    flex-direction: column;
    gap: var(--spacing-md);
  }

  .tab-controls {
    display: flex;
    gap: var(--spacing-sm);
    flex-wrap: wrap;
  }

  .cmd-line {
    display: flex;
    gap: var(--spacing-md);
    margin-bottom: var(--spacing-md);
  }

  .prompt {
    color: var(--phosphor);
  }

  .command {
    color: #fff;
  }

  .output-lines {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 0.95rem;
  }

  .cursor-line {
    margin-top: var(--spacing-sm);
  }
  
  .cursor {
    color: var(--phosphor);
  }
</style>
