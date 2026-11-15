document.addEventListener('DOMContentLoaded', () => {
  const carousels = document.querySelectorAll('[data-carousel]');

  carousels.forEach((carousel) => {
    const slides = Array.from(carousel.querySelectorAll('.carousel-slide'));
    const track = carousel.querySelector('.carousel-track');
    const prevButton = carousel.querySelector('[data-carousel-prev]');
    const nextButton = carousel.querySelector('[data-carousel-next]');
    const counter = carousel.querySelector('[data-carousel-counter]');

    if (!track || !slides.length) return;

    let index = 0;
    let intervalId;

    function update() {
      track.style.transform = 'translateX(-' + (index * 100) + '%)';
      slides.forEach((slide, slideIndex) => {
        const video = slide.querySelector('video');
        if (video) {
          if (slideIndex === index) {
            video.currentTime = 0;
            video.play().catch(() => {});
          } else {
            video.pause();
          }
        }
      });

      if (counter) {
        const current = String(index + 1).padStart(2, '0');
        const total = String(slides.length).padStart(2, '0');
        counter.textContent = current + ' / ' + total;
      }
    }

    function next() {
      index = (index + 1) % slides.length;
      update();
    }

    function prev() {
      index = (index - 1 + slides.length) % slides.length;
      update();
    }

    function start() {
      if (intervalId) clearInterval(intervalId);
      intervalId = window.setInterval(next, 5000);
    }

    prevButton && prevButton.addEventListener('click', () => {
      prev();
      start();
    });

    nextButton && nextButton.addEventListener('click', () => {
      next();
      start();
    });

    let touchStartX = 0;
    carousel.addEventListener('touchstart', (event) => {
      touchStartX = event.touches[0].clientX;
    }, { passive: true });

    carousel.addEventListener('touchend', (event) => {
      const touchEndX = event.changedTouches[0].clientX;
      const diff = touchEndX - touchStartX;
      if (Math.abs(diff) > 40) {
        if (diff < 0) next();
        else prev();
      }
      start();
    }, { passive: true });

    carousel.addEventListener('mouseenter', () => {
      if (intervalId) clearInterval(intervalId);
    });
    carousel.addEventListener('mouseleave', start);

    update();
    start();
  });
});
