<script lang="ts">
  import { onMount } from 'svelte';
  import { fly, fade } from 'svelte/transition';
  import { Play } from "lucide-svelte";

  let mounted = $state(false);
  let particlesRef: HTMLDivElement;

  onMount(() => {
    mounted = true;

    // Create floating particles
    const count = 25;
    for (let i = 0; i < count; i++) {
      const el = document.createElement('div');
      const size = Math.random() * 3 + 1;
      const color = Math.random() > 0.5 ? '#22d3ee' : '#a78bfa';
      el.className = 'absolute rounded-full';
      el.style.cssText = `
        width: ${size}px;
        height: ${size}px;
        background: ${color};
        left: ${Math.random() * 100}%;
        top: ${Math.random() * 100}%;
        opacity: ${Math.random() * 0.3 + 0.05};
        animation: float ${3 + Math.random() * 4}s ease-in-out infinite alternate;
        animation-delay: ${Math.random() * 3}s;
      `;
      particlesRef?.appendChild(el);
    }

    return () => {
      if (particlesRef) particlesRef.innerHTML = '';
    };
  });
</script>

<svelte:head>
  <style>
    @keyframes float {
      0% { transform: translate(0, 0); }
      100% { transform: translate(${Math.random() > 0.5 ? '' : '-'}${Math.random() * 30 + 10}px, ${Math.random() > 0.5 ? '' : '-'}${Math.random() * 30 + 10}px); }
    }
  </style>
</svelte:head>

<section class="relative flex min-h-[100dvh] items-center justify-center overflow-hidden px-4 pt-16">
  <!-- Background gradients -->
  <div class="pointer-events-none absolute inset-0 z-0">
    <div class="absolute inset-0 bg-[radial-gradient(ellipse_80%_50%_at_50%_-20%,rgba(34,211,238,0.12),transparent)]"></div>
    <div class="absolute inset-0 bg-[radial-gradient(ellipse_60%_40%_at_80%_60%,rgba(167,139,250,0.08),transparent)]"></div>
  </div>

  <!-- Particles -->
  <div bind:this={particlesRef} class="pointer-events-none absolute inset-0 z-0"></div>

  <!-- Content -->
  <div class="relative z-10 mx-auto max-w-3xl text-center">
    {#if mounted}
      <h1 class="mb-4 text-4xl font-extrabold tracking-tight sm:text-5xl md:text-6xl lg:text-7xl" in:fly={{ y: 30, duration: 700, delay: 200 }}>
        <span class="text-brand-text">Scheduled</span><span class="bg-gradient-to-r from-brand-cyan to-brand-purple bg-clip-text text-transparent">-DB</span>
      </h1>

      <p class="mx-auto mb-8 max-w-xl text-base leading-relaxed text-brand-text-secondary sm:text-lg md:text-xl" in:fly={{ y: 20, duration: 700, delay: 500 }}>
        A distributed job scheduling system built on Raft consensus. Reliable, fault-tolerant job execution with automatic leader election and failover.
      </p>

      <div class="flex flex-wrap items-center justify-center gap-3 sm:gap-4" in:fly={{ y: 15, duration: 600, delay: 800 }}>
        <a
          href="#quickstart"
          class="inline-flex items-center gap-2 rounded-lg bg-gradient-to-r from-brand-cyan to-brand-blue px-6 py-3 text-sm font-semibold text-brand-bg transition-all hover:-translate-y-0.5 hover:shadow-lg hover:shadow-brand-cyan/25 sm:text-base"
        >
          <Play size={18} fill="currentColor" />
          Get Started
        </a>
        <a
          href="https://github.com/manudiv16/scheduled-db"
          target="_blank"
          rel="noopener"
          class="inline-flex items-center gap-2 rounded-lg border border-brand-border px-6 py-3 text-sm font-semibold text-brand-text transition-all hover:border-brand-cyan hover:bg-brand-cyan/5 sm:text-base"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/></svg>
          GitHub
        </a>
      </div>
    {/if}
  </div>
</section>
