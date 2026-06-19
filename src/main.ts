import './style.css';

// Update the year in the footer, if present.
const yearEl = document.querySelector<HTMLElement>('[data-year]');
if (yearEl) {
  yearEl.textContent = String(new Date().getFullYear());
}

// Smooth-scroll for in-page anchor links (progressive enhancement).
document.querySelectorAll<HTMLAnchorElement>('a[href^="#"]').forEach((link) => {
  link.addEventListener('click', (event) => {
    const id = link.getAttribute('href')?.slice(1);
    if (!id) return;
    const target = document.getElementById(id);
    if (!target) return;
    event.preventDefault();
    target.scrollIntoView({ behavior: 'smooth' });
  });
});
