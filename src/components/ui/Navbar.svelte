<script lang="ts">
  let open = $state(false);

  function close() {
    open = false;
  }
</script>

<nav class="hud-nav" class:menu-open={open}>
  <div class="nav-inner">
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
        <span class="hud-text">SYSTEM ONLINE</span>
      </div>
      <a href="#features" class="nav-link" onclick={close}>
        <span class="link-prefix">01 //</span>
        <span class="link-text hud-text">FEATURES</span>
      </a>
      <a href="#architecture" class="nav-link" onclick={close}>
        <span class="link-prefix">02 //</span>
        <span class="link-text hud-text">ARCHITECTURE</span>
      </a>
      <a href="#roadmap" class="nav-link" onclick={close}>
        <span class="link-prefix">03 //</span>
        <span class="link-text hud-text">ROADMAP</span>
      </a>
      <a href="#quickstart" class="nav-link" onclick={close}>
        <span class="link-prefix">04 //</span>
        <span class="link-text hud-text">QUICK START</span>
      </a>
      <a href="#api" class="nav-link" onclick={close}>
        <span class="link-prefix">05 //</span>
        <span class="link-text hud-text">API</span>
      </a>
      <a href="https://github.com/manudiv16/scheduled-db" target="_blank" rel="noopener" class="nav-link nav-github" onclick={close}>
        <span class="link-prefix">GH //</span>
        <span class="link-text hud-text">GITHUB</span>
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
    padding: 0 var(--spacing-xl);
  }

  .logo {
    display: flex;
    align-items: center;
    gap: var(--spacing-sm);
    flex-shrink: 0;
    min-height: 44px;
    padding: 0 var(--spacing-xs);
  }

  .logo-text {
    font-family: var(--font-display);
    font-weight: 700;
    letter-spacing: 0.05em;
    font-size: 0.95rem;
  }

  /* Desktop nav */
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
    margin-right: var(--spacing-sm);
  }

  .status-dot {
    width: 8px;
    height: 8px;
    background-color: var(--phosphor);
    border-radius: 50%;
    box-shadow: 0 0 8px rgba(124, 255, 95, 0.6);
    flex-shrink: 0;
  }

  /* Desktop link style */
  .nav-link {
    color: var(--ink-secondary);
    font-size: 0.8rem;
    transition: all 0.2s var(--ease-out);
    display: flex;
    align-items: center;
    gap: var(--spacing-xs);
  }

  .link-prefix {
    display: none;
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
    justify-content: center;
    align-items: center;
    gap: 5px;
    background: none;
    border: none;
    cursor: pointer;
    width: 44px;
    height: 44px;
    padding: 0;
    z-index: 102;
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

  /* Mobile */
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
      background: var(--bg-void);
      border-left: 2px solid var(--neon-cyan);
      flex-direction: column;
      justify-content: center;
      align-items: flex-start;
      padding: var(--spacing-xl);
      gap: 0;
      z-index: 99;
      isolation: isolate;
      transform: translateY(-20px);
      opacity: 0;
      pointer-events: none;
      transition: transform 0.3s var(--ease-out), opacity 0.3s var(--ease-out);
    }

    .nav-links.open {
      transform: translateY(0);
      opacity: 1;
      pointer-events: auto;
    }

    .nav-status {
      margin-bottom: var(--spacing-xl);
      margin-right: 0;
    }

    .nav-link {
      display: flex;
      align-items: center;
      gap: var(--spacing-md);
      padding: var(--spacing-md) 0;
      width: 100%;
      border-bottom: 1px solid rgba(255, 255, 255, 0.04);
      color: var(--ink-secondary);
    }

    .link-prefix {
      display: inline;
      font-family: var(--font-mono);
      font-size: 0.75rem;
      color: var(--ink-mute);
      min-width: 40px;
    }

    .link-text {
      font-size: 1.1rem;
      letter-spacing: 0.12em;
    }

    .nav-link:hover {
      color: var(--neon-cyan);
    }

    .nav-link:hover .link-prefix {
      color: var(--neon-cyan);
    }

    .logo-text {
      font-size: 0.85rem;
    }

    .nav-inner {
      padding: 0 var(--spacing-md);
    }
  }
</style>
