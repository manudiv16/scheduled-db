<script lang="ts">
  let open = $state(false);

  function close() {
    open = false;
  }
</script>

<nav class="hud-nav">
  <div class="container nav-inner">
    <a href="/scheduled-db/" class="logo" onclick={close}>
      <div class="logo-icon">
        <svg width="24" height="24" viewBox="0 0 32 32" fill="none" stroke="var(--neon-cyan)">
          <circle cx="16" cy="16" r="14" stroke-width="2" stroke-dasharray="4 2"/>
          <circle cx="16" cy="16" r="8" stroke-width="1"/>
          <circle cx="16" cy="16" r="2" fill="var(--neon-cyan)"/>
          <line x1="16" y1="16" x2="16" y2="4" stroke-width="2"/>
        </svg>
      </div>
      <span class="logo-text neon-text-cyan">SCHEDULED-DB</span>
    </a>

    <button class="hamburger" onclick={() => open = !open} aria-label="Toggle menu" aria-expanded={open}>
      <span class="bar" class:open></span>
      <span class="bar" class:open></span>
      <span class="bar" class:open></span>
    </button>

    <div class="nav-links" class:open>
      <div class="nav-status">
        <span class="status-dot anim-blink"></span>
        <span class="hud-text">ONLINE</span>
      </div>
      <a href="#features" class="nav-link hud-text" onclick={close}>FEATURES</a>
      <a href="#architecture" class="nav-link hud-text" onclick={close}>ARCHITECTURE</a>
      <a href="#roadmap" class="nav-link hud-text" onclick={close}>ROADMAP</a>
      <a href="#quickstart" class="nav-link hud-text" onclick={close}>QUICK START</a>
      <a href="#api" class="nav-link hud-text" onclick={close}>API</a>
      <a href="https://github.com/manudiv16/scheduled-db" target="_blank" rel="noopener" class="nav-github" onclick={close}>
        <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/></svg>
      </a>
    </div>
  </div>
</nav>

<style>
  .hud-nav {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    height: 48px;
    background: rgba(4, 6, 12, 0.95);
    border-bottom: 1px solid var(--border-hud);
    z-index: 100;
    backdrop-filter: blur(8px);
  }

  .nav-inner {
    display: flex;
    justify-content: space-between;
    align-items: center;
    height: 100%;
    max-width: 1200px;
    margin: 0 auto;
    padding: 0 var(--spacing-lg);
  }

  .logo {
    display: flex;
    align-items: center;
    gap: var(--spacing-sm);
    flex-shrink: 0;
  }

  .logo-text {
    font-family: var(--font-display);
    font-weight: 700;
    letter-spacing: 0.05em;
    font-size: 0.95rem;
  }

  .nav-links {
    display: flex;
    align-items: center;
    gap: var(--spacing-md);
  }

  .nav-status {
    display: flex;
    align-items: center;
    gap: var(--spacing-xs);
    color: var(--phosphor);
  }

  .status-dot {
    width: 8px;
    height: 8px;
    background-color: var(--phosphor);
    border-radius: 50%;
    box-shadow: 0 0 8px rgba(124, 255, 95, 0.6);
    flex-shrink: 0;
  }

  .nav-link {
    color: var(--ink-secondary);
    font-size: 0.8rem;
  }

  .nav-link:hover {
    color: var(--neon-cyan);
    text-shadow: 0 0 8px rgba(0, 224, 255, 0.4);
  }

  .nav-github {
    color: var(--ink-secondary);
    display: flex;
    align-items: center;
  }

  .nav-github:hover {
    color: var(--ink-primary);
  }

  /* Hamburger — hidden on desktop */
  .hamburger {
    display: none;
    flex-direction: column;
    gap: 5px;
    background: none;
    border: none;
    cursor: pointer;
    padding: var(--spacing-xs);
    z-index: 101;
  }

  .hamburger:focus-visible {
    outline: 2px solid var(--neon-cyan);
    outline-offset: 2px;
  }

  .bar {
    width: 22px;
    height: 2px;
    background: var(--ink-primary);
    transition: all 0.3s var(--ease-out);
    display: block;
  }

  .bar.open:nth-child(1) {
    transform: rotate(45deg) translate(5px, 5px);
  }
  .bar.open:nth-child(2) {
    opacity: 0;
  }
  .bar.open:nth-child(3) {
    transform: rotate(-45deg) translate(5px, -5px);
  }

  @media (max-width: 768px) {
    .hamburger {
      display: flex;
    }

    .nav-links {
      position: fixed;
      top: 48px;
      left: 0;
      right: 0;
      bottom: 0;
      background: rgba(4, 6, 12, 0.98);
      flex-direction: column;
      align-items: flex-start;
      padding: var(--spacing-xl) var(--spacing-lg);
      gap: var(--spacing-lg);
      transform: translateX(100%);
      transition: transform 0.3s var(--ease-out);
    }

    .nav-links.open {
      transform: translateX(0);
    }

    .nav-link {
      font-size: 1.1rem;
    }

    .nav-status .hud-text {
      font-size: 0.8rem;
    }

    .logo-text {
      font-size: 0.85rem;
    }
  }
</style>
