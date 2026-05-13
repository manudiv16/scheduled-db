<script lang="ts">
  import { onMount } from 'svelte';
  import { fly, fade } from 'svelte/transition';
  import { Zap, Repeat, Clock, ShieldCheck, RefreshCw, Globe } from 'lucide-svelte';
  import type { Component } from 'svelte';

  interface Feature {
    icon: Component;
    title: string;
    description: string;
    color: string;
    bg: string;
  }

  const features: Feature[] = [
    {
      icon: Zap,
      title: 'Distributed Consensus',
      description: 'Built on HashiCorp Raft for strong consistency. Automatic leader election and log replication across all nodes.',
      color: 'text-brand-cyan',
      bg: 'bg-brand-cyan/10',
    },
    {
      icon: Repeat,
      title: 'Two Job Types',
      description: 'Unico: one-time execution at a specific timestamp. Recurrente: recurring execution with cron expressions.',
      color: 'text-brand-purple',
      bg: 'bg-brand-purple/10',
    },
    {
      icon: Clock,
      title: 'Time-Slotted Scheduling',
      description: 'Efficient job organization with configurable slot intervals. Optimized for high-throughput scheduling.',
      color: 'text-brand-blue',
      bg: 'bg-brand-blue/10',
    },
    {
      icon: ShieldCheck,
      title: 'Queue Size Limits',
      description: 'Configurable memory and job count limits to prevent OOM. Smart rejection with 507 Insufficient Storage.',
      color: 'text-brand-amber',
      bg: 'bg-brand-amber/10',
    },
    {
      icon: RefreshCw,
      title: 'High Availability',
      description: 'Automatic failover and graceful leader resignation. Zero-downtime cluster reconfiguration.',
      color: 'text-brand-green',
      bg: 'bg-brand-green/10',
    },
    {
      icon: Globe,
      title: 'Service Discovery',
      description: 'Multiple strategies: Kubernetes, DNS, Gossip, Static. Automatic cluster formation and node joining.',
      color: 'text-brand-pink',
      bg: 'bg-brand-pink/10',
    },
  ];

  let gridRef: HTMLDivElement;
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
    observer.observe(gridRef);
    return () => observer.disconnect();
  });
</script>

<div bind:this={gridRef} class="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
  {#each features as feature, i}
    {#if visible}
      <div
        class="group relative overflow-hidden rounded-xl border border-brand-border bg-brand-card p-6 transition-all duration-300 hover:-translate-y-1 hover:border-brand-cyan/30 hover:shadow-lg hover:shadow-brand-cyan/5"
        in:fly={{ y: 40, duration: 600, delay: i * 100 }}
      >
        <!-- Glow effect -->
        <div class="absolute inset-0 bg-gradient-radial from-transparent via-transparent to-transparent opacity-0 transition-opacity duration-300 group-hover:opacity-100" style="background: radial-gradient(circle at 50% 0%, {feature.color.replace('text-', '')}33, transparent 70%);"></div>

        <div class="relative">
          <div class="mb-4 inline-flex h-10 w-10 items-center justify-center rounded-lg {feature.bg} {feature.color}">
            <svelte:component this={feature.icon} size={20} />
          </div>
          <h3 class="text-lg font-bold text-brand-text">{feature.title}</h3>
          <p class="mt-2 text-sm leading-relaxed text-brand-text-secondary">{feature.description}</p>
        </div>
      </div>
    {/if}
  {/each}
</div>
