---
version: "alpha"
name: Scheduled-DB
description: Distributed job scheduling system — dark developer tooling aesthetic with cyan/purple gradient accents
colors:
  bg-primary: "#0a0e1a"
  bg-secondary: "#111827"
  bg-card: "#1e293b"
  bg-card-hover: "#263348"
  text-primary: "#f1f5f9"
  text-secondary: "#94a3b8"
  text-muted: "#64748b"
  accent-cyan: "#22d3ee"
  accent-purple: "#a78bfa"
  accent-blue: "#3b82f6"
  accent-green: "#34d399"
  accent-amber: "#fbbf24"
  accent-pink: "#f472b6"
  accent-red: "#f87171"
  border-color: "#334155"
typography:
  hero-title:
    fontFamily: Inter
    fontSize: clamp(2.5rem, 7vw, 5rem)
    fontWeight: 800
    letterSpacing: "-0.02em"
    lineHeight: 1.1
  section-title:
    fontFamily: Inter
    fontSize: 2rem
    fontWeight: 700
  section-subtitle:
    fontFamily: Inter
    fontSize: 1.1rem
    lineHeight: 1.6
  body:
    fontFamily: Inter
    fontSize: 0.9rem
    lineHeight: 1.6
  body-lg:
    fontFamily: Inter
    fontSize: 1.25rem
    lineHeight: 1.6
  code:
    fontFamily: JetBrains Mono
    fontSize: 0.875rem
    lineHeight: 1.7
  nav-link:
    fontFamily: Inter
    fontSize: 0.85rem
    fontWeight: 500
  badge:
    fontFamily: Inter
    fontSize: 0.7rem
    fontWeight: 600
    letterSpacing: "0.03em"
  label-caps:
    fontFamily: Inter
    fontSize: 0.65rem
    fontWeight: 700
    letterSpacing: "0.08em"
rounded:
  sm: "4px"
  md: "8px"
  lg: "12px"
  xl: "16px"
  full: "999px"
spacing:
  xs: "0.4rem"
  sm: "0.65rem"
  md: "1rem"
  lg: "1.5rem"
  xl: "2rem"
  "2xl": "3rem"
  "3xl": "5rem"
components:
  btn-primary:
    backgroundColor: "linear-gradient(135deg, #22d3ee, #3b82f6)"
    textColor: "#0a0e1a"
    rounded: "{rounded.md}"
    padding: "0.75rem 1.75rem"
    height: "auto"
  btn-primary-hover:
    backgroundColor: "linear-gradient(135deg, #22d3ee, #3b82f6)"
  btn-secondary:
    backgroundColor: "transparent"
    textColor: "#f1f5f9"
    rounded: "{rounded.md}"
    padding: "0.75rem 1.75rem"
  btn-secondary-hover:
    backgroundColor: "rgba(34, 211, 238, 0.05)"
  feature-card:
    backgroundColor: "rgba(30, 41, 59, 0.5)"
    rounded: "{rounded.lg}"
    padding: "1.75rem"
  roadmap-card:
    backgroundColor: "rgba(30, 41, 59, 0.5)"
    rounded: "{rounded.xl}"
    padding: "1.75rem"
  arch-node:
    backgroundColor: "rgba(30, 41, 59, 0.5)"
    rounded: "{rounded.lg}"
    padding: "1.25rem 1.5rem"
  badge-pill:
    backgroundColor: "rgba(34, 211, 238, 0.1)"
    textColor: "#22d3ee"
    rounded: "{rounded.full}"
    padding: "0.3rem 0.75rem"
  method-badge-post:
    backgroundColor: "rgba(52, 211, 153, 0.15)"
    textColor: "#34d399"
    rounded: "{rounded.sm}"
    padding: "0.15rem 0.5rem"
  method-badge-get:
    backgroundColor: "rgba(34, 211, 238, 0.15)"
    textColor: "#22d3ee"
    rounded: "{rounded.sm}"
    padding: "0.15rem 0.5rem"
  method-badge-delete:
    backgroundColor: "rgba(248, 113, 113, 0.15)"
    textColor: "#f87171"
    rounded: "{rounded.sm}"
    padding: "0.15rem 0.5rem"
  nav:
    backgroundColor: "rgba(10, 14, 26, 0.75)"
    height: "64px"
  code-block:
    backgroundColor: "#1e293b"
    rounded: "{rounded.md}"
    padding: "1.5rem"
  status-badge-in-progress:
    backgroundColor: "rgba(34, 211, 238, 0.12)"
    textColor: "#22d3ee"
    rounded: "{rounded.full}"
    padding: "0.2rem 0.6rem"
  status-badge-planned:
    backgroundColor: "rgba(148, 163, 184, 0.1)"
    textColor: "#94a3b8"
    rounded: "{rounded.full}"
    padding: "0.2rem 0.6rem"
---

# Design System: Scheduled-DB

## Overview

A dark, developer-focused aesthetic for a distributed systems tooling landing page. The design conveys technical sophistication through deep navy backgrounds, electric cyan and purple gradient accents, and precise geometric elements. The atmosphere is clean and structured — like a well-organized terminal with modern UI sensibilities.

The visual language draws from developer tooling conventions: monospace code blocks, structured data tables, and system architecture diagrams. Gradient accents signal interactive elements and important information hierarchy. Glassmorphism on the navbar and cards adds depth without breaking the dark theme.

**Target audience:** Backend engineers, DevOps, and platform engineers evaluating distributed job scheduling solutions.

## Colors

The palette is built on a deep navy foundation with vibrant accent colors that each serve a semantic role.

- **Deep Space** (`#0a0e1a`) — Primary background, page canvas
- **Night** (`#111827`) — Secondary background, footer border
- **Slate Card** (`#1e293b`) — Card and container fills, code blocks
- **Slate Hover** (`#263348`) — Card hover state background
- **Cloud White** (`#f1f5f9`) — Primary text, headings
- **Mist** (`#94a3b8`) — Secondary text, descriptions, body copy
- **Fog** (`#64748b`) — Muted text, labels, metadata
- **Electric Cyan** (`#22d3ee`) — Primary accent, links, active states, gradients
- **Neon Purple** (`#a78bfa`) — Secondary accent, gradient endpoint, feature highlights
- **Signal Blue** (`#3b82f6`) — Tertiary accent, gradient endpoint, follower nodes
- **Success Green** (`#34d399`) — POST method badges, success indicators, webhooks
- **Warning Amber** (`#fbbf24`) — Leader node indicators, queue limit warnings
- **Accent Pink** (`#f472b6`) — Worker node indicators, service discovery
- **Alert Red** (`#f87171`) — DELETE method badges, split-brain warnings
- **Steel Border** (`#334155`) — Card borders, dividers, table borders

**Gradient:** Primary gradient flows from Electric Cyan (`#22d3ee`) to Neon Purple (`#a78bfa`) at 135deg. Used for section titles, logo highlight, and hero title accent.

**Usage rules:**
- Never use pure black (`#000000`) — use Deep Space for backgrounds
- Accent colors are reserved for interactive elements, status indicators, and data visualization
- Each feature card uses a unique accent color via CSS custom property (`--accent`)
- Method badges in the API table use semantic colors: green for POST, cyan for GET, red for DELETE

## Typography

Two font families create clear visual separation between prose and code.

**Inter** — Primary sans-serif for all UI text, headings, and body copy. Weight-driven hierarchy from 500 (nav links) to 800 (hero title).

**JetBrains Mono** — Monospace for code blocks, inline code, API endpoints, and method badges. Provides technical authenticity and readability for terminal-style content.

**Hierarchy:**
- Hero title: 800 weight, tight letter-spacing (`-0.02em`), responsive scale from 2.5rem to 5rem
- Section titles: 700 weight, gradient text effect, 2rem fixed
- Body text: 400 weight default, 1.6 line-height for readability
- Code: 0.875rem, 1.7 line-height for comfortable reading of multi-line commands
- Labels and badges: 600–700 weight, uppercase with letter-spacing for scannability

**Banned:** Serif fonts, decorative display fonts. The monospace/sans-serif pairing is intentional for a developer tooling context.

## Layout

Single-column layout with centered content containment. Maximum width of 1200px ensures readability on ultrawide displays.

**Spacing strategy:** Generous vertical rhythm between sections (5rem padding on desktop, 3rem on mobile). Cards use consistent internal padding (1.75rem). Grid gaps of 1.5rem create breathing room between cards.

**Section structure:** Each section follows the same pattern — centered title, centered subtitle (max-width 600px), then the component (grid, diagram, table, or code block).

**Responsive behavior:**
- Below 768px: nav links hide, feature grid collapses to single column, hero title scales down to 2.25rem
- Feature grid uses `auto-fit` with `minmax(320px, 1fr)` for fluid responsiveness
- Architecture diagram nodes stack vertically on mobile

**Grid patterns:**
- Feature cards: `repeat(auto-fit, minmax(300px, 1fr))`
- Roadmap cards: `repeat(auto-fit, minmax(300px, 1fr))`
- Architecture nodes: flex row with wrap, centered

## Elevation & Depth

The design uses a flat-first approach with subtle depth cues for hierarchy.

**Glassmorphism:** Navbar and cards use `backdrop-filter: blur(10–16px)` with semi-transparent backgrounds. This creates a frosted glass effect that reveals the dark background beneath while maintaining readability.

**Hover elevation:** Cards lift with `translateY(-4px)` and gain a shadow (`0 12px 40px rgba(0, 0, 0, 0.3)`) on hover. The border color transitions to the card's accent color for a glow effect.

**Glow effects:** Feature cards have a radial gradient overlay (`mix-blend-mode: overlay`) that appears on hover, creating a colored glow from the top edge.

**No drop shadows** on static elements — depth is communicated through border colors, background contrast, and the glassmorphism blur effect.

## Shapes

Consistent rounded corners create a modern, approachable feel while maintaining technical credibility.

**Border radius scale:**
- `sm` (4px) — Inline code, method badges, role labels
- `md` (8px) — Buttons, code blocks, API table wrapper
- `lg` (12px) — Feature cards, architecture nodes
- `xl` (16px) — Roadmap cards, architecture cluster box
- `full` (999px) — Pill badges, status badges, hero badges

**Icon containers:** Square with rounded corners (10–12px radius), sized 42–44px. Each has a tinted background matching its accent color at 10% opacity.

**Cluster box:** Uses dashed border (`1px dashed rgba(34, 211, 238, 0.3)`) to visually group Raft cluster nodes while remaining distinct from solid-bordered cards.

## Components

### Buttons
- **Primary:** Gradient fill (cyan to blue), dark text, 8px radius. Hover lifts 2px with cyan glow shadow.
- **Secondary:** Transparent with steel border, white text. Hover adds cyan border and subtle cyan tint background.
- Both use inline-flex with icon + text, 0.5rem gap.

### Feature Cards
- Semi-transparent slate background with backdrop blur.
- Icon at top (emoji, 2rem), title, description, and invisible glow overlay.
- Hover: lift, border color changes to accent, glow overlay fades in.
- Each card has a unique `--accent` CSS variable for its glow color.

### Roadmap Cards
- Similar to feature cards but with 16px radius and structured header + list layout.
- Header contains icon container (44px, tinted background), title, and status badge.
- Status badges: "In Progress" (cyan with pulsing dot) or "Planned" (gray with static dot).
- List items use small square bullets (6px, 2px radius) colored to the card's accent.

### Architecture Nodes
- Compact cards (min-width 130px) with icon, label, role badge, and detail text.
- Leader node has amber border and pulsing indicator dot (top-right).
- Follower nodes have blue border.
- Connection lines use gradient strokes with animated flow dots.

### Code Blocks
- Terminal-style with tab bar at top. Tabs have rounded top corners.
- Active tab: cyan text, slate background, visible border.
- Code area: slate background, 1.5rem padding, JetBrains Mono font.
- Content transition: fade-in + slide-up on tab switch.

### Badge Pills
- Small rounded labels for status, methods, and hero features.
- Hero badges: cyan tint background, cyan text, full radius.
- Method badges: semantic colors (green/cyan/red) with matching tint backgrounds.
- Status badges: include animated dot for "In Progress" state.

### Navbar
- Fixed position, full width, 64px height.
- Glassmorphism: `blur(16px)` with 75% opacity dark background.
- Subtle bottom border (`rgba(51, 65, 85, 0.5)`).
- Logo: SVG cluster diagram + text with gradient highlight.
- Links: muted text, pill-shaped hover background.
- GitHub icon: 36px square, border appears on hover.

### API Table
- Full-width table with collapsed borders.
- Header: slate background, cyan text, 2px bottom border.
- Rows: subtle bottom borders, hover adds cyan tint background.
- Method column uses colored badges instead of plain text.
- Endpoint column uses monospace code styling with green text.

## Do's and Don'ts

### Do
- Use semantic color roles consistently (cyan for links/actions, green for creation, red for deletion)
- Apply glassmorphism for depth — never solid opaque overlays on the dark background
- Use gradient text sparingly — only for hero title, section titles, and logo highlight
- Animate with purpose — staggered entrances, hover lifts, and continuous pulses for live indicators
- Maintain the two-font system — Inter for UI, JetBrains Mono for code
- Use emoji icons for feature cards — they add personality to the technical aesthetic
- Include status badges with animated indicators for real-time feel

### Don't
- Never use pure black (`#000000`) — always use the defined background colors
- Don't add more than one accent color per card — each card has a single `--accent` variable
- Don't use drop shadows on static elements — reserve shadows for hover states only
- Don't mix serif or decorative fonts — stick to Inter + JetBrains Mono
- Don't use circular particles in the hero — they should be square/rectangular (pixel aesthetic)
- Don't add navigation links beyond the defined set (Features, Architecture, Roadmap, Quick Start, API, GitHub)
- Don't use bright/neon colors outside the defined palette
- Don't add rounded corners smaller than 4px or larger than 16px (except full-radius pills)
- Don't place content outside the 1200px max-width container
- Don't use solid borders on cards — use the defined steel border color with opacity
