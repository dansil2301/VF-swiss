document.addEventListener('DOMContentLoaded', () => {
  const modalMap = new Map();

  const modals = document.querySelectorAll('.modal');
  modals.forEach((modal) => {
    const id = modal.id;
    modalMap.set(id, modal);

    const triggers = document.querySelectorAll('[data-modal-open="' + id + '"]');
    triggers.forEach((trigger) => {
      trigger.addEventListener('click', () => openModal(modal));
    });

    modal.querySelectorAll('[data-close-modal]').forEach((button) => {
      button.addEventListener('click', () => closeModal(modal));
    });
  });

  function openModal(modal) {
    if (!modal) return;
    modal.classList.add('is-open');
    modal.setAttribute('aria-hidden', 'false');
    document.body.classList.add('modal-open');
    const dialog = modal.querySelector('.modal-dialog');
    if (dialog) {
      dialog.focus();
    }
  }

  function closeModal(modal) {
    if (!modal) return;
    modal.classList.remove('is-open');
    modal.setAttribute('aria-hidden', 'true');
    document.body.classList.remove('modal-open');
  }

  document.addEventListener('keydown', (event) => {
    if (event.key !== 'Escape') return;
    modals.forEach((modal) => {
      if (modal.classList.contains('is-open')) {
        closeModal(modal);
      }
    });
  });

  document.addEventListener('click', (event) => {
    const target = event.target;
    if (!(target instanceof HTMLElement)) return;
    if (target.classList.contains('modal-backdrop')) {
      const parentModal = target.closest('.modal');
      if (parentModal) closeModal(parentModal);
    }
  });
});
