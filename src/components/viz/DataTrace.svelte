<script lang="ts">
  import { replicationBeam } from '../../lib/animate';
  
  interface Props {
    pathId: string;
    pathData: string;
    delay?: number;
    tone?: 'cyan' | 'magenta' | 'phosphor';
  }
  let { pathId, pathData, delay = 0, tone = 'cyan' }: Props = $props();
  
  let packet: SVGElement | undefined = $state();
  let pathEl: SVGPathElement | undefined = $state();

  let packetColor = $derived(
    tone === 'cyan' ? 'var(--neon-cyan)' :
    tone === 'magenta' ? 'var(--neon-magenta)' :
    'var(--phosphor)'
  );

  $effect(() => {
    if (packet && pathEl) {
      const anim = replicationBeam(packet as HTMLElement, pathEl, delay);
      return () => {
        if (anim) anim.pause();
      };
    }
  });
</script>

<g class="data-trace">
  <!-- The Track -->
  <path id={pathId} bind:this={pathEl} d={pathData} fill="none" stroke="var(--border-hud)" stroke-width="1" stroke-dasharray="2 4" />
  
  <!-- The Packet -->
  <circle bind:this={packet} r="3" fill={packetColor} style="opacity: 0; filter: drop-shadow(0 0 4px {packetColor});" />
</g>
