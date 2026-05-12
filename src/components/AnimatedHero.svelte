<!--
  AnimatedHero.svelte — Hero section principal con animaciones
  - Título animado letra por letra con anime.js stagger
  - Partículas de fondo generadas dinámicamente (estilo pixel cuadrado)
  - Botones CTA con animación de entrada secuencial
  - IntersectionObserver no necesario (siempre visible al cargar)
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import anime from 'animejs';

  let titleRef: HTMLElement;
  let subtitleRef: HTMLElement;
  let buttonsRef: HTMLElement;
  let particlesRef: HTMLElement;

  onMount(() => {
    if (!titleRef) return;

    // Animación letra por letra del título con stagger de 50ms
    anime({
      targets: titleRef.querySelectorAll('.letter'),
      opacity: [0, 1],
      translateY: [40, 0],
      delay: anime.stagger(50, { start: 300 }),
      duration: 800,
      easing: 'easeOutExpo',
    });

    // Fade-in del subtítulo con delay de 1.2s
    anime({
      targets: subtitleRef,
      opacity: [0, 1],
      translateY: [20, 0],
      duration: 800,
      delay: 1200,
      easing: 'easeOutExpo',
    });

    // Animación secuencial de los botones CTA
    anime({
      targets: buttonsRef.querySelectorAll('.btn'),
      opacity: [0, 1],
      translateY: [20, 0],
      delay: anime.stagger(100, { start: 1600 }),
      duration: 600,
      easing: 'easeOutExpo',
    });

    // Generar 30 partículas cuadradas (estilo pixel) con posiciones y tamaños aleatorios
    const particles = Array.from({ length: 30 }, () => {
      const el = document.createElement('div');
      el.className = 'particle';
      const size = Math.random() * 4 + 2;
      el.style.cssText = `
        position: absolute;
        width: ${size}px;
        height: ${size}px;
        background: ${Math.random() > 0.5 ? '#22d3ee' : '#a78bfa'};
        border-radius: 1px;
        opacity: 0;
        left: ${Math.random() * 100}%;
        top: ${Math.random() * 100}%;
      `;
      return el;
    });

    particles.forEach((p) => particlesRef?.appendChild(p));

    // Animación continua de partículas: movimiento aleatorio con loop
    anime({
      targets: particlesRef?.querySelectorAll('.particle'),
      opacity: [0, () => Math.random() * 0.5 + 0.1],
      translateX: () => anime.random(-40, 40),
      translateY: () => anime.random(-40, 40),
      duration: () => anime.random(3000, 5000),
      delay: anime.stagger(80, { start: 400 }),
      direction: 'alternate',
      loop: true,
      easing: 'easeInOutSine',
    });
  });

  const title = 'Scheduled-DB';
  const letters = title.split('');
</script>

<div class="hero">
  <div class="hero-bg"></div>
  <div bind:this={particlesRef} class="particles"></div>
  <div class="hero-content">
    <!-- Badges de características principales -->
    <div class="hero-badges">
      <span class="badge">Open Source</span>
      <span class="badge">Distributed</span>
      <span class="badge">Fault-Tolerant</span>
    </div>
    <h1 bind:this={titleRef}>
      {#each letters as letter, i}
        <span class="letter" style="opacity: 0; display: inline-block;">
          {#if letter === '-'}
            <span class="gradient">-</span>
          {:else}
            {letter}
          {/if}
        </span>
      {/each}
      <span class="letter gradient" style="opacity: 0; display: inline-block;">DB</span>
    </h1>
    <p bind:this={subtitleRef} style="opacity: 0;">
      A distributed job scheduling system built on Raft consensus. Reliable, fault-tolerant job execution with automatic leader election, failover, and hierarchical timing wheels.
    </p>
    <div bind:this={buttonsRef} class="hero-buttons">
      <a href="#quickstart" class="btn btn-primary" style="opacity: 0;">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"/></svg>
        Get Started
      </a>
      <a href="https://github.com/manudiv16/scheduled-db" target="_blank" rel="noopener" class="btn btn-secondary" style="opacity: 0;">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/></svg>
        GitHub
      </a>
    </div>
  </div>
</div>

<style>
  .hero {
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    text-align: center;
    position: relative;
    overflow: hidden;
    padding-top: 60px;
  }

  .hero-bg {
    position: absolute;
    inset: 0;
    background:
      radial-gradient(ellipse 80% 50% at 50% -20%, rgba(34, 211, 238, 0.12), transparent),
      radial-gradient(ellipse 60% 40% at 80% 60%, rgba(167, 139, 250, 0.08), transparent);
    z-index: 0;
  }

  .particles {
    position: absolute;
    inset: 0;
    z-index: 0;
    pointer-events: none;
  }

  .hero-content {
    position: relative;
    z-index: 1;
  }

  /* Badges de características sobre el título */
  .hero-badges {
    display: flex;
    gap: 0.5rem;
    justify-content: center;
    flex-wrap: wrap;
    margin-bottom: 1.25rem;
  }

  .badge {
    font-size: 0.75rem;
    font-weight: 600;
    padding: 0.3rem 0.75rem;
    border-radius: 999px;
    background: rgba(34, 211, 238, 0.1);
    color: #22d3ee;
    border: 1px solid rgba(34, 211, 238, 0.2);
    letter-spacing: 0.03em;
  }

  h1 {
    font-size: clamp(2.5rem, 7vw, 5rem);
    font-weight: 800;
    letter-spacing: -0.02em;
    line-height: 1.1;
    margin-bottom: 1rem;
  }

  .gradient {
    background: linear-gradient(135deg, #22d3ee, #a78bfa);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
  }

  p {
    font-size: 1.25rem;
    color: #94a3b8;
    max-width: 640px;
    margin: 0 auto 2rem;
    line-height: 1.6;
  }

  .hero-buttons {
    display: flex;
    gap: 1rem;
    justify-content: center;
    flex-wrap: wrap;
  }

  .btn {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.75rem 1.75rem;
    border-radius: 8px;
    font-weight: 600;
    font-size: 0.95rem;
    transition: all 0.25s;
    cursor: pointer;
    border: none;
    text-decoration: none;
  }

  .btn-primary {
    background: linear-gradient(135deg, #22d3ee, #3b82f6);
    color: #0a0e1a;
  }

  .btn-primary:hover {
    transform: translateY(-2px);
    box-shadow: 0 8px 24px rgba(34, 211, 238, 0.25);
  }

  .btn-secondary {
    background: transparent;
    color: #f1f5f9;
    border: 1px solid #334155;
  }

  .btn-secondary:hover {
    border-color: #22d3ee;
    background: rgba(34, 211, 238, 0.05);
  }

  @media (max-width: 768px) {
    h1 {
      font-size: 2.25rem;
    }
    p {
      font-size: 1rem;
      padding: 0 1rem;
    }
  }
</style>
