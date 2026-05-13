<script lang="ts">
  import { Check, Loader2, CircleDot } from "lucide-svelte";
  import { fly, fade } from "svelte/transition";
  import type { Milestone, MilestoneStatus } from "../data/timeline";

  interface Props {
    items: Milestone[];
  }

  let { items }: Props = $props();

  let visible = $state<boolean[]>(new Array(items.length).fill(false));
  let containerRef: HTMLDivElement;

  const statusConfig: Record<MilestoneStatus, { label: string; icon: typeof Check; color: string; bg: string; border: string }> = {
    'completed': { label: 'Completed', icon: Check, color: 'text-emerald-400', bg: 'bg-emerald-500/10', border: 'border-emerald-500/30' },
    'in-progress': { label: 'In Progress', icon: Loader2, color: 'text-amber-400', bg: 'bg-amber-500/10', border: 'border-amber-500/30' },
    'planned': { label: 'Planned', icon: CircleDot, color: 'text-slate-400', bg: 'bg-slate-500/10', border: 'border-slate-500/30' },
  };

  $effect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            const idx = Number(entry.target.getAttribute('data-index'));
            if (!isNaN(idx)) {
              visible[idx] = true;
            }
          }
        });
      },
      { threshold: 0.15 }
    );

    const nodes = containerRef?.querySelectorAll('.timeline-item');
    nodes?.forEach((node) => observer.observe(node));

    return () => observer.disconnect();
  });
</script>

<div bind:this={containerRef} class="relative mx-auto max-w-4xl">
  <!-- Central line (desktop) -->
  <div class="absolute left-8 top-0 bottom-0 w-0.5 bg-gradient-to-b from-brand-border via-brand-border to-transparent md:left-1/2 md:-translate-x-px"></div>

  <div class="flex flex-col gap-8 md:gap-12">
    {#each items as item, i}
      <div
        class="timeline-item relative flex items-start md:items-center"
        data-index={i}
      >
        <!-- Desktop alternating layout -->
        <div class="hidden md:grid md:grid-cols-[1fr_auto_1fr] md:gap-8 md:items-center md:w-full">
          <!-- Left side -->
          <div class="text-right" class:order-1={i % 2 === 0} class:order-3={i % 2 !== 0}>
            {#if visible[i]}
              <div
                in:fly={{ x: i % 2 === 0 ? -30 : 30, duration: 600, delay: 100 }}
              >
                <span class="text-sm font-semibold text-brand-text-muted">{item.date}</span>
              </div>
            {/if}
          </div>

          <!-- Center node -->
          <div class="order-2 relative flex items-center justify-center">
            {#if visible[i]}
              <div
                class="flex h-10 w-10 items-center justify-center rounded-full border-2 {statusConfig[item.status].border} {statusConfig[item.status].bg}"
                in:fly={{ y: 20, duration: 500, delay: i * 100 }}
              >
                <svelte:component this={statusConfig[item.status].icon} size={18} class={statusConfig[item.status].color + (item.status === 'in-progress' ? ' animate-spin' : '')} />
              </div>
            {/if}
          </div>

          <!-- Right side -->
          <div class="text-left" class:order-3={i % 2 === 0} class:order-1={i % 2 !== 0}>
            {#if visible[i]}
              <div
                in:fly={{ x: i % 2 === 0 ? 30 : -30, duration: 600, delay: 200 }}
              >
                <div class="rounded-xl border border-brand-border bg-brand-card p-5 transition-all hover:border-brand-cyan/30 hover:shadow-lg hover:shadow-brand-cyan/5">
                  <div class="mb-2 flex items-center gap-2">
                    <span class="rounded-full px-2 py-0.5 text-xs font-semibold {statusConfig[item.status].bg} {statusConfig[item.status].color} border {statusConfig[item.status].border}">
                      {statusConfig[item.status].label}
                    </span>
                  </div>
                  <h3 class="text-lg font-bold text-brand-text">{item.title}</h3>
                  <p class="mt-1 text-sm leading-relaxed text-brand-text-secondary">{item.description}</p>
                </div>
              </div>
            {/if}
          </div>
        </div>

        <!-- Mobile layout -->
        <div class="flex items-start gap-4 md:hidden">
          <div class="relative flex flex-col items-center">
            {#if visible[i]}
              <div
                class="flex h-9 w-9 items-center justify-center rounded-full border-2 {statusConfig[item.status].border} {statusConfig[item.status].bg}"
                in:fly={{ y: 20, duration: 500, delay: i * 100 }}
              >
                <svelte:component this={statusConfig[item.status].icon} size={16} class={statusConfig[item.status].color + (item.status === 'in-progress' ? ' animate-spin' : '')} />
              </div>
            {/if}
          </div>
          {#if visible[i]}
            <div class="flex-1" in:fly={{ x: 20, duration: 600, delay: 150 }}>
              <span class="text-xs font-semibold uppercase tracking-wider text-brand-text-muted">{item.date}</span>
              <div class="mt-1 rounded-xl border border-brand-border bg-brand-card p-4">
                <div class="mb-1.5 flex items-center gap-2">
                  <span class="rounded-full px-2 py-0.5 text-xs font-semibold {statusConfig[item.status].bg} {statusConfig[item.status].color} border {statusConfig[item.status].border}">
                    {statusConfig[item.status].label}
                  </span>
                </div>
                <h3 class="text-base font-bold text-brand-text">{item.title}</h3>
                <p class="mt-1 text-sm leading-relaxed text-brand-text-secondary">{item.description}</p>
              </div>
            </div>
          {/if}
        </div>
      </div>
    {/each}
  </div>
</div>
