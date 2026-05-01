<script lang="ts">
  import { onMount } from 'svelte';
  import anime from 'animejs';

  let svgRef: HTMLElement;

  onMount(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            anime({
              targets: svgRef.querySelectorAll('.node'),
              opacity: [0, 1],
              scale: [0.5, 1],
              delay: anime.stagger(200),
              duration: 800,
              easing: 'easeOutBack',
            });

            anime({
              targets: svgRef.querySelectorAll('.connection'),
              strokeDashoffset: [anime.setDashoffset, 0],
              opacity: [0, 0.6],
              delay: anime.stagger(150, { start: 600 }),
              duration: 1000,
              easing: 'easeInOutSine',
            });

            anime({
              targets: svgRef.querySelectorAll('.pulse'),
              scale: [1, 1.3],
              opacity: [0.6, 0],
              duration: 2000,
              delay: anime.stagger(400, { start: 1500 }),
              loop: true,
              easing: 'easeOutSine',
            });

            anime({
              targets: svgRef.querySelectorAll('.data-flow'),
              translateX: [0, 10],
              opacity: [0.3, 0.8, 0.3],
              duration: 2500,
              loop: true,
              direction: 'alternate',
              easing: 'easeInOutSine',
              delay: anime.stagger(300, { start: 2000 }),
            });

            observer.disconnect();
          }
        });
      },
      { threshold: 0.2 }
    );

    observer.observe(svgRef);
  });
</script>

<div bind:this={svgRef} class="arch-container">
  <svg viewBox="0 0 600 400" xmlns="http://www.w3.org/2000/svg" class="arch-svg">
    <!-- Connections -->
    <line class="connection" x1="300" y1="120" x2="180" y2="260" stroke="#22d3ee" stroke-width="2" opacity="0"/>
    <line class="connection" x1="300" y1="120" x2="420" y2="260" stroke="#22d3ee" stroke-width="2" opacity="0"/>
    <line class="connection" x1="180" y1="260" x2="420" y2="260" stroke="#a78bfa" stroke-width="1.5" stroke-dasharray="5,5" opacity="0"/>

    <!-- Leader Node -->
    <g class="node" style="opacity: 0; transform-origin: 300px 100px;">
      <circle class="pulse" cx="300" cy="100" r="42" fill="none" stroke="#fbbf24" stroke-width="2"/>
      <circle cx="300" cy="100" r="36" fill="#1e293b" stroke="#fbbf24" stroke-width="2.5"/>
      <text x="300" y="95" text-anchor="middle" fill="#fbbf24" font-size="11" font-weight="bold">LEADER</text>
      <text x="300" y="112" text-anchor="middle" fill="#94a3b8" font-size="9">Node 1</text>
    </g>

    <!-- Follower 1 -->
    <g class="node" style="opacity: 0; transform-origin: 180px 260px;">
      <circle class="pulse" cx="180" cy="260" r="38" fill="none" stroke="#3b82f6" stroke-width="2"/>
      <circle cx="180" cy="260" r="32" fill="#1e293b" stroke="#3b82f6" stroke-width="2"/>
      <text x="180" y="255" text-anchor="middle" fill="#3b82f6" font-size="10" font-weight="bold">FOLLOWER</text>
      <text x="180" y="272" text-anchor="middle" fill="#94a3b8" font-size="9">Node 2</text>
    </g>

    <!-- Follower 2 -->
    <g class="node" style="opacity: 0; transform-origin: 420px 260px;">
      <circle class="pulse" cx="420" cy="260" r="38" fill="none" stroke="#3b82f6" stroke-width="2"/>
      <circle cx="420" cy="260" r="32" fill="#1e293b" stroke="#3b82f6" stroke-width="2"/>
      <text x="420" y="255" text-anchor="middle" fill="#3b82f6" font-size="10" font-weight="bold">FOLLOWER</text>
      <text x="420" y="272" text-anchor="middle" fill="#94a3b8" font-size="9">Node 3</text>
    </g>

    <!-- Client -->
    <g class="node" style="opacity: 0; transform-origin: 520px 100px;">
      <rect x="485" y="75" width="70" height="50" rx="8" fill="#1e293b" stroke="#34d399" stroke-width="2"/>
      <text x="520" y="100" text-anchor="middle" fill="#34d399" font-size="10" font-weight="bold">CLIENT</text>
      <text x="520" y="115" text-anchor="middle" fill="#94a3b8" font-size="8">HTTP API</text>
    </g>

    <!-- Worker -->
    <g class="node" style="opacity: 0; transform-origin: 80px 100px;">
      <rect x="35" y="75" width="80" height="50" rx="8" fill="#1e293b" stroke="#f472b6" stroke-width="2"/>
      <text x="75" y="97" text-anchor="middle" fill="#f472b6" font-size="10" font-weight="bold">WORKER</text>
      <text x="75" y="113" text-anchor="middle" fill="#94a3b8" font-size="8">Job Executor</text>
    </g>

    <!-- Data flow indicators -->
    <circle class="data-flow" cx="240" cy="185" r="3" fill="#22d3ee" opacity="0"/>
    <circle class="data-flow" cx="360" cy="185" r="3" fill="#22d3ee" opacity="0"/>
    <circle class="data-flow" cx="300" cy="260" r="3" fill="#a78bfa" opacity="0"/>

    <!-- Connection line: Client -> Leader -->
    <line class="connection" x1="485" y1="100" x2="336" y2="100" stroke="#34d399" stroke-width="1.5" opacity="0"/>
    <!-- Connection line: Leader -> Worker -->
    <line class="connection" x1="264" y1="100" x2="115" y2="100" stroke="#f472b6" stroke-width="1.5" opacity="0"/>

    <!-- Labels -->
    <text x="300" y="360" text-anchor="middle" fill="#64748b" font-size="11">Raft Consensus Protocol</text>
    <text x="410" y="135" text-anchor="middle" fill="#34d399" font-size="8" opacity="0.7">REST API</text>
    <text x="190" y="135" text-anchor="middle" fill="#f472b6" font-size="8" opacity="0.7">Webhooks</text>
    <text x="240" y="210" text-anchor="middle" fill="#22d3ee" font-size="8" opacity="0.7">Replication</text>
    <text x="360" y="210" text-anchor="middle" fill="#22d3ee" font-size="8" opacity="0.7">Replication</text>
  </svg>
</div>

<style>
  .arch-container {
    max-width: 700px;
    margin: 0 auto;
    padding: 1rem;
  }

  .arch-svg {
    width: 100%;
    height: auto;
  }

  .connection {
    stroke-linecap: round;
  }
</style>
