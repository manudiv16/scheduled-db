<script lang="ts">
  import { Menu, X } from "lucide-svelte";
  import { fade, fly } from "svelte/transition";
  import { clickOutside } from "../utils/clickOutside";

  let open = $state(false);

  const navLinks = [
    { href: "#features", label: "Features" },
    { href: "#architecture", label: "Architecture" },
    { href: "#roadmap", label: "Roadmap" },
    { href: "#quickstart", label: "Quick Start" },
    { href: "#api", label: "API" },
  ];

  function toggleMenu() {
    open = !open;
  }

  function closeMenu() {
    open = false;
  }
</script>

<nav class="sticky top-0 z-50 w-full border-b border-white/5 bg-white/5 backdrop-blur-xl">
  <div class="mx-auto flex h-16 max-w-7xl items-center justify-between px-4 sm:px-6">
    <!-- Logo -->
    <a href="/scheduled-db/" class="flex items-center gap-2 text-lg font-bold text-brand-text transition-opacity hover:opacity-85">
      <svg width="28" height="28" viewBox="0 0 32 32" fill="none">
        <circle cx="16" cy="16" r="14" fill="none" stroke="url(#logo-grad)" stroke-width="2.5"/>
        <circle cx="16" cy="10" r="3" fill="#fbbf24"/>
        <circle cx="10" cy="20" r="2.5" fill="#3b82f6"/>
        <circle cx="22" cy="20" r="2.5" fill="#3b82f6"/>
        <line x1="16" y1="13" x2="10" y2="18" stroke="#22d3ee" stroke-width="1.5" opacity="0.7"/>
        <line x1="16" y1="13" x2="22" y2="18" stroke="#22d3ee" stroke-width="1.5" opacity="0.7"/>
        <line x1="10" y1="20" x2="22" y2="20" stroke="#a78bfa" stroke-width="1" stroke-dasharray="2,2" opacity="0.5"/>
        <defs>
          <linearGradient id="logo-grad" x1="0" y1="0" x2="32" y2="32">
            <stop offset="0%" stop-color="#22d3ee"/>
            <stop offset="100%" stop-color="#a78bfa"/>
          </linearGradient>
        </defs>
      </svg>
      <span>Scheduled<span class="bg-gradient-to-r from-brand-cyan to-brand-purple bg-clip-text text-transparent font-extrabold">-DB</span></span>
    </a>

    <!-- Desktop nav -->
    <div class="hidden items-center gap-1 md:flex">
      {#each navLinks as link}
        <a
          href={link.href}
          class="rounded-md px-3 py-2 text-sm font-medium text-brand-text-muted transition-colors hover:bg-white/5 hover:text-brand-text"
        >
          {link.label}
        </a>
      {/each}
      <a
        href="https://github.com/manudiv16/scheduled-db"
        target="_blank"
        rel="noopener"
        class="ml-2 flex h-9 w-9 items-center justify-center rounded-lg border border-transparent text-brand-text-muted transition-all hover:border-brand-border hover:bg-white/5 hover:text-brand-text"
        aria-label="GitHub"
      >
        <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/></svg>
      </a>
    </div>

    <!-- Mobile hamburger -->
    <button
      onclick={toggleMenu}
      class="flex h-11 w-11 items-center justify-center rounded-lg text-brand-text-muted transition-colors hover:bg-white/5 hover:text-brand-text md:hidden"
      aria-label="Toggle menu"
      aria-expanded={open}
    >
      {#if open}
        <X size={22} />
      {:else}
        <Menu size={22} />
      {/if}
    </button>
  </div>

  <!-- Mobile menu overlay -->
  {#if open}
    <div
      class="fixed inset-0 z-40 bg-black/60 backdrop-blur-sm md:hidden"
      onclick={closeMenu}
      transition:fade={{ duration: 200 }}
    ></div>

    <div
      class="fixed right-0 top-0 z-50 h-full w-[280px] border-l border-white/10 bg-brand-bg/95 backdrop-blur-xl md:hidden"
      use:clickOutside={() => open = false}
      transition:fly={{ x: 280, duration: 300 }}
    >
      <div class="flex h-full flex-col p-6">
        <div class="mb-8 flex items-center justify-between">
          <span class="text-lg font-bold text-brand-text">Menu</span>
          <button
            onclick={closeMenu}
            class="flex h-10 w-10 items-center justify-center rounded-lg text-brand-text-muted transition-colors hover:bg-white/5 hover:text-brand-text"
            aria-label="Close menu"
          >
            <X size={20} />
          </button>
        </div>

        <div class="flex flex-col gap-1">
          {#each navLinks as link, i}
            <a
              href={link.href}
              onclick={closeMenu}
              class="rounded-lg px-4 py-3 text-base font-medium text-brand-text-secondary transition-colors hover:bg-white/5 hover:text-brand-text"
              in:fly={{ x: 20, duration: 300, delay: i * 50 }}
            >
              {link.label}
            </a>
          {/each}
        </div>

        <div class="mt-auto pt-6">
          <a
            href="https://github.com/manudiv16/scheduled-db"
            target="_blank"
            rel="noopener"
            class="flex items-center gap-2 rounded-lg px-4 py-3 text-brand-text-muted transition-colors hover:bg-white/5 hover:text-brand-text"
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/></svg>
            <span class="text-sm font-medium">View on GitHub</span>
          </a>
        </div>
      </div>
    </div>
  {/if}
</nav>
