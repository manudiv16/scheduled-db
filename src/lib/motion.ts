export function useInView(node: HTMLElement, callback: (inView: boolean) => void) {
  let observer: IntersectionObserver | null = null;

  if (typeof IntersectionObserver !== 'undefined') {
    observer = new IntersectionObserver((entries) => {
      const entry = entries[0];
      callback(entry.isIntersecting);
    }, {
      threshold: 0.1
    });
    
    observer.observe(node);
  } else {
    callback(true);
  }

  return {
    destroy() {
      if (observer) {
        observer.disconnect();
      }
    }
  };
}
