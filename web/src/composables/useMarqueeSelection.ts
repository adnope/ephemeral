import { onBeforeUnmount, type Ref } from "vue";

const interactiveSelector = "a, button, input, select, textarea, dialog";
const dragThreshold = 4;
const autoScrollEdge = 52;
const maxAutoScrollSpeed = 22;

type ScrollTarget = Window | HTMLElement;

export function useMarqueeSelection(
  container: Ref<HTMLElement | null>,
  active: Ref<boolean>,
  selectedIDs: Ref<Set<number>>,
) {
  let cleanup: (() => void) | null = null;

  function start(event: PointerEvent) {
    if (!active.value || event.button !== 0 || !container.value) return;
    const target = event.target as Element;
    if (target.closest(interactiveSelector)) return;

    const root = container.value;
    const scrollTarget = scrollingTarget(root);
    const anchor = contentPoint(scrollTarget, event.clientX, event.clientY);
    let pointerX = event.clientX;
    let pointerY = event.clientY;
    let dragged = false;
    let frame = 0;
    let running = true;
    const box = document.createElement("div");
    box.className = "selection-box";
    document.body.appendChild(box);

    const render = () => {
      frame = 0;
      if (!running) return;
      autoScroll(scrollTarget, pointerX, pointerY);

      const fixedAnchor = viewportPoint(scrollTarget, anchor.x, anchor.y);
      const left = Math.min(fixedAnchor.x, pointerX);
      const top = Math.min(fixedAnchor.y, pointerY);
      const right = Math.max(fixedAnchor.x, pointerX);
      const bottom = Math.max(fixedAnchor.y, pointerY);
      const width = right - left;
      const height = bottom - top;
      box.style.transform = `translate3d(${left}px, ${top}px, 0)`;
      box.style.width = `${width}px`;
      box.style.height = `${height}px`;

      if (width > dragThreshold || height > dragThreshold) dragged = true;
      if (dragged) updateSelection(root, selectedIDs, left, top, right, bottom);

      if (autoScrollVelocity(scrollTarget, pointerX, pointerY) !== 0) {
        frame = requestAnimationFrame(render);
      }
    };

    const schedule = () => {
      if (!frame) frame = requestAnimationFrame(render);
    };
    const move = (moveEvent: PointerEvent) => {
      pointerX = moveEvent.clientX;
      pointerY = moveEvent.clientY;
      schedule();
    };
    const stop = () => {
      running = false;
      cleanup?.();
      if (dragged) {
        window.addEventListener(
          "click",
          (click) => click.stopImmediatePropagation(),
          { capture: true, once: true },
        );
      }
    };
    cleanup = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", stop);
      window.removeEventListener("pointercancel", stop);
      window.removeEventListener("blur", stop);
      scrollEventTarget(scrollTarget).removeEventListener("scroll", schedule);
      if (frame) cancelAnimationFrame(frame);
      box.remove();
      cleanup = null;
    };

    event.preventDefault();
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", stop);
    window.addEventListener("pointercancel", stop);
    window.addEventListener("blur", stop);
    scrollEventTarget(scrollTarget).addEventListener("scroll", schedule, {
      passive: true,
    });
    schedule();
  }

  onBeforeUnmount(() => cleanup?.());
  return { start };
}

function scrollingTarget(element: HTMLElement): ScrollTarget {
  let candidate: HTMLElement | null = element;
  while (candidate) {
    const style = getComputedStyle(candidate);
    if (
      /(auto|scroll)/.test(style.overflowY) &&
      candidate.scrollHeight > candidate.clientHeight
    ) {
      return candidate;
    }
    candidate = candidate.parentElement;
  }
  return window;
}

function contentPoint(target: ScrollTarget, clientX: number, clientY: number) {
  if (target === window) {
    return { x: clientX + window.scrollX, y: clientY + window.scrollY };
  }
  const element = target as HTMLElement;
  const rect = element.getBoundingClientRect();
  return {
    x: clientX - rect.left + element.scrollLeft,
    y: clientY - rect.top + element.scrollTop,
  };
}

function viewportPoint(target: ScrollTarget, x: number, y: number) {
  if (target === window) {
    return { x: x - window.scrollX, y: y - window.scrollY };
  }
  const element = target as HTMLElement;
  const rect = element.getBoundingClientRect();
  return {
    x: rect.left + x - element.scrollLeft,
    y: rect.top + y - element.scrollTop,
  };
}

function scrollEventTarget(target: ScrollTarget): Window | HTMLElement {
  return target === window ? window : target;
}

function scrollBounds(target: ScrollTarget) {
  if (target === window) return { top: 0, bottom: window.innerHeight };
  const rect = (target as HTMLElement).getBoundingClientRect();
  return { top: rect.top, bottom: rect.bottom };
}

function autoScrollVelocity(
  target: ScrollTarget,
  _pointerX: number,
  pointerY: number,
) {
  const bounds = scrollBounds(target);
  if (pointerY < bounds.top + autoScrollEdge) {
    const ratio = Math.min(
      1,
      (bounds.top + autoScrollEdge - pointerY) / autoScrollEdge,
    );
    return -Math.ceil(maxAutoScrollSpeed * ratio);
  }
  if (pointerY > bounds.bottom - autoScrollEdge) {
    const ratio = Math.min(
      1,
      (pointerY - (bounds.bottom - autoScrollEdge)) / autoScrollEdge,
    );
    return Math.ceil(maxAutoScrollSpeed * ratio);
  }
  return 0;
}

function autoScroll(target: ScrollTarget, pointerX: number, pointerY: number) {
  const velocity = autoScrollVelocity(target, pointerX, pointerY);
  if (!velocity) return;
  if (target === window) window.scrollBy(0, velocity);
  else (target as HTMLElement).scrollTop += velocity;
}

function updateSelection(
  root: HTMLElement,
  selectedIDs: Ref<Set<number>>,
  left: number,
  top: number,
  right: number,
  bottom: number,
) {
  const next = new Set<number>();
  for (const element of root.querySelectorAll<HTMLElement>("[data-item-id]")) {
    const rect = element.getBoundingClientRect();
    const overlaps = !(
      left > rect.right ||
      right < rect.left ||
      top > rect.bottom ||
      bottom < rect.top
    );
    const id = Number.parseInt(element.dataset.itemId ?? "", 10);
    if (overlaps && Number.isSafeInteger(id)) next.add(id);
  }
  if (!sameSelection(selectedIDs.value, next)) selectedIDs.value = next;
}

function sameSelection(left: Set<number>, right: Set<number>) {
  if (left.size !== right.size) return false;
  for (const value of left) if (!right.has(value)) return false;
  return true;
}
