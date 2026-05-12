import anime from 'animejs';

export function respectsReducedMotion(): boolean {
  if (typeof window === 'undefined') return false;
  const matchMedia = window.matchMedia('(prefers-reduced-motion: reduce)');
  return matchMedia.matches;
}

export function bootSequence(
  titleEl: HTMLElement | null,
  subtitleEl: HTMLElement | null,
  buttons: HTMLElement[],
  wheelEl: HTMLElement | null
) {
  if (respectsReducedMotion()) {
    [titleEl, subtitleEl, ...buttons, wheelEl].forEach(el => {
      if (el) el.style.opacity = '1';
    });
    return null;
  }

  const tl = anime.timeline({
    easing: 'easeOutCubic',
    duration: 800
  });

  if (wheelEl) {
    tl.add({
      targets: wheelEl,
      opacity: [0, 0.35],
      duration: 1000
    });
  }

  if (titleEl) {
    tl.add({
      targets: titleEl,
      opacity: [0, 1],
      scale: [0.95, 1],
      duration: 600,
      easing: 'easeOutElastic(1, 0.8)'
    }, '-=400');
  }

  if (subtitleEl) {
    tl.add({
      targets: subtitleEl,
      opacity: [0, 1],
      translateY: [10, 0],
      duration: 400
    }, '-=400');
  }

  if (buttons.length > 0) {
    tl.add({
      targets: buttons,
      opacity: [0, 1],
      translateY: [10, 0],
      delay: anime.stagger(100)
    }, '-=200');
  }

  return tl;
}

export function staggerReveal(
  targets: HTMLElement | HTMLElement[] | string | NodeListOf<Element>,
  grid?: [number, number],
  from: 'center' | 'first' | 'last' | number = 'center'
) {
  if (respectsReducedMotion()) {
    anime.set(targets, { opacity: 1, translateY: 0, scale: 1 });
    return null;
  }

  const staggerOpts: any = { start: 100 };
  if (grid) {
    staggerOpts.grid = grid;
    staggerOpts.from = from;
  } else {
    staggerOpts.from = from;
  }

  return anime({
    targets,
    opacity: [0, 1],
    scale: [0.95, 1],
    translateY: [20, 0],
    delay: anime.stagger(80, staggerOpts),
    easing: 'easeOutCubic',
    duration: 600
  });
}

export function replicationBeam(
  target: HTMLElement | string,
  pathEl: SVGPathElement,
  delayOffset: number = 0
) {
  if (respectsReducedMotion()) {
    anime.set(target, { opacity: 0 });
    return null;
  }

  const path = anime.path(pathEl);
  
  return anime({
    targets: target,
    translateX: path('x'),
    translateY: path('y'),
    opacity: [
      { value: 1, duration: 50 },
      { value: 1, duration: 750 },
      { value: 0, duration: 50 }
    ],
    easing: 'linear',
    duration: 850,
    delay: delayOffset,
    loop: true
  });
}
