document.addEventListener('DOMContentLoaded', () => {
  const form = document.querySelector('.request-form');
  const submitButton = document.getElementById('submit-button');
  const nameInput = document.getElementById('name');
  const phoneInput = document.getElementById('phone');
  const brandSelect = document.getElementById('brand-select');
  const brandCustom = document.getElementById('brand_custom');
  const brandHidden = document.getElementById('brand');
  const brandFieldGroup = document.getElementById('brand-field-group');
  const descriptionInput = document.getElementById('description');
  const photosInput = document.getElementById('photos');
  const photoSummary = document.getElementById('photo-summary');
  const turnstileWidget = document.querySelector('.cf-turnstile');
  const turnstileStatus = document.getElementById('turnstile-status');
  const turnstileRetry = document.getElementById('turnstile-retry');
  const turnstileEnabled = !!turnstileWidget;
  let turnstilePassed = false;
  let turnstileFailed = false;
  let selectedFiles = [];

  function setTurnstileState(passed, failed, message) {
    turnstilePassed = passed;
    turnstileFailed = failed;
    if (turnstileStatus) turnstileStatus.textContent = message;
    if (turnstileRetry) turnstileRetry.hidden = !failed;
    validateRequiredFields();
  }

  if (turnstileEnabled) {
    window.vfTurnstileSuccess = (token) => {
      if (token) {
        setTurnstileState(true, false, 'Проверка пройдена.');
      } else {
        setTurnstileState(false, true, 'Не удалось подтвердить проверку Cloudflare. Обновите страницу и попробуйте снова.');
      }
    };
    window.vfTurnstileError = () => {
      setTurnstileState(false, true, 'Проверка Cloudflare недоступна. Проверьте подключение и обновите страницу.');
    };
    window.vfTurnstileExpired = () => {
      setTurnstileState(false, false, 'Проверка истекла. Пройдите её ещё раз.');
    };
    window.vfTurnstileTimeout = () => {
      setTurnstileState(false, false, 'Проверка заняла слишком много времени. Пройдите её ещё раз.');
    };

    window.addEventListener('error', (event) => {
      if (event.target instanceof HTMLScriptElement &&
          event.target.src.startsWith('https://challenges.cloudflare.com/turnstile/')) {
        setTurnstileState(false, true, 'Не удалось загрузить проверку Cloudflare. Проверьте подключение и обновите страницу.');
      }
    }, true);

    if (!window.turnstile) {
      setTurnstileState(false, true, 'Не удалось загрузить проверку Cloudflare. Проверьте подключение и обновите страницу.');
    }

    window.setTimeout(() => {
      if (!turnstilePassed && !turnstileFailed) {
        setTurnstileState(false, true, 'Проверка Cloudflare не завершилась. Проверьте подключение и обновите страницу.');
      }
    }, 60000);

    if (turnstileRetry) {
      turnstileRetry.addEventListener('click', () => window.location.reload());
    }
  }

  function applyFiles(files) {
    if (!photosInput) {
      return;
    }
    if (typeof DataTransfer === 'undefined') {
      photosInput.value = '';
      return;
    }
    const dataTransfer = new DataTransfer();
    files.forEach((file) => dataTransfer.items.add(file));
    photosInput.files = dataTransfer.files;
  }

  function setSelectedFiles(files) {
    selectedFiles = files;
    applyFiles(selectedFiles);
    renderFileSummary();
  }

  function removeFile(index) {
    const nextFiles = selectedFiles.filter((_, itemIndex) => itemIndex !== index);
    setSelectedFiles(nextFiles);
  }

  if (photosInput && !photosInput.multiple) {
    photosInput.multiple = true;
    photosInput.setAttribute('multiple', 'multiple');
  }

  function formatPhone(value) {
    const digits = value.replace(/\D/g, '').slice(0, 11);
    if (!digits) return '';

    const normalized = digits.startsWith('8') || digits.startsWith('7') ? digits.slice(1) : digits;
    let result = '+7';

    if (normalized.length > 0) {
      result += ' (' + normalized.slice(0, Math.min(3, normalized.length));
    }
    if (normalized.length > 3) {
      result += ') ' + normalized.slice(3, Math.min(6, normalized.length));
    }
    if (normalized.length > 6) {
      result += '-' + normalized.slice(6, Math.min(8, normalized.length));
    }
    if (normalized.length > 8) {
      result += '-' + normalized.slice(8, Math.min(10, normalized.length));
    }
    return result;
  }

  function updateBrandState() {
    if (!brandSelect || !brandCustom || !brandHidden) return;
    const selected = brandSelect.value;
    const customVisible = selected === 'Другой бренд';
    const customWrap = document.getElementById('custom-brand-field');
    if (customWrap) customWrap.hidden = !customVisible;
    if (brandFieldGroup) brandFieldGroup.classList.toggle('is-custom-brand', customVisible);
    if (customVisible) {
      brandCustom.required = true;
      brandHidden.value = brandCustom.value.trim();
    } else {
      brandCustom.required = false;
      brandHidden.value = selected;
      brandCustom.value = '';
    }
  }

  function syncBrandField() {
    if (!brandSelect || !brandCustom || !brandHidden) return;
    if (brandSelect.value === 'Другой бренд') {
      brandHidden.value = brandCustom.value.trim();
      return;
    }
    brandHidden.value = brandSelect.value;
  }

  function validateRequiredFields() {
    if (!submitButton) return;

    const nameFilled = !!(nameInput && nameInput.value.trim());
    const phoneFilled = !!(phoneInput && phoneInput.value.trim());
    const brandFilled = !!(brandHidden && brandHidden.value.trim());

    if (nameInput) nameInput.classList.toggle('invalid', !nameFilled);
    if (phoneInput) phoneInput.classList.toggle('invalid', !phoneFilled);
    if (brandSelect) brandSelect.classList.toggle('invalid', !brandFilled);

    submitButton.disabled = !(nameFilled && phoneFilled && brandFilled && (!turnstileEnabled || turnstilePassed));
  }

  function renderFileSummary() {
    if (!photosInput || !photoSummary) return;
    const files = selectedFiles.length ? selectedFiles : Array.from(photosInput.files || []);
    if (!files.length) {
      photoSummary.innerHTML = '';
      return;
    }

    if (files.length > 5) {
      photoSummary.innerHTML = '<span class="photo-error">Можно прикрепить не более 5 фотографий.</span>';
      return;
    }

    const maxSize = 5 * 1024 * 1024;
    if (files.some((file) => file.size > maxSize)) {
      photoSummary.innerHTML = '<span class="photo-error">Размер файла превышает допустимый лимит.</span>';
      return;
    }

    photoSummary.innerHTML = '';
    files.forEach((file, index) => {
      const row = document.createElement('div');
      row.className = 'photo-item';
      row.innerHTML = `
        <span class="photo-item-name">${file.name} (${(file.size / 1024 / 1024).toFixed(1)} МБ)</span>
        <button type="button" class="photo-item-remove" data-index="${index}" aria-label="Удалить файл ${file.name}">Удалить</button>
      `;
      const removeButton = row.querySelector('.photo-item-remove');
      removeButton.addEventListener('click', () => removeFile(index));
      photoSummary.appendChild(row);
    });
  }

  if (phoneInput) {
    phoneInput.addEventListener('input', () => {
      const formatted = formatPhone(phoneInput.value);
      phoneInput.value = formatted;
      validateRequiredFields();
    });
  }

  if (brandSelect) {
    brandSelect.addEventListener('change', () => {
      updateBrandState();
      validateRequiredFields();
    });
  }

  if (brandCustom) {
    brandCustom.addEventListener('input', () => {
      if (brandSelect && brandSelect.value === 'Другой бренд') {
        brandHidden.value = brandCustom.value.trim();
      }
      validateRequiredFields();
    });
  }

  [nameInput, descriptionInput].forEach((input) => {
    if (!input) return;
    input.addEventListener('input', validateRequiredFields);
  });

  if (photosInput) {
    photosInput.addEventListener('change', () => {
      const incoming = Array.from(photosInput.files || []);
      if (!incoming.length) {
        selectedFiles = [];
        renderFileSummary();
        return;
      }

      const merged = [...selectedFiles, ...incoming];
      const unique = [];
      const seen = new Set();

      merged.forEach((file) => {
        const key = `${file.name}-${file.size}-${file.lastModified}`;
        if (!seen.has(key)) {
          seen.add(key);
          unique.push(file);
        }
      });

      if (unique.length > 5) {
        if (photoSummary) {
          photoSummary.innerHTML = '<span class="photo-error">Можно прикрепить не более 5 фотографий.</span>';
        }
        if (selectedFiles.length) {
          applyFiles(selectedFiles);
        }
        return;
      }

      if (unique.some((file) => file.size > 5 * 1024 * 1024)) {
        if (photoSummary) {
          photoSummary.innerHTML = '<span class="photo-error">Размер файла превышает допустимый лимит.</span>';
        }
        if (selectedFiles.length) {
          applyFiles(selectedFiles);
        }
        return;
      }

      setSelectedFiles(unique);
    });
  }

  if (form) {
    form.addEventListener('submit', (event) => {
      syncBrandField();
      if (turnstileEnabled && !turnstilePassed) {
        event.preventDefault();
        if (!turnstileFailed && turnstileStatus) {
          turnstileStatus.textContent = 'Пройдите проверку Cloudflare, чтобы отправить заявку.';
        }
        return;
      }
      if (submitButton) {
        submitButton.disabled = true;
        submitButton.textContent = 'Отправляем...';
      }
    });
  }

  if (brandSelect && brandHidden && brandHidden.value) {
    const matches = Array.from(brandSelect.options).some((option) => option.value === brandHidden.value);
    if (matches) {
      brandSelect.value = brandHidden.value;
    } else {
      brandSelect.value = 'Другой бренд';
      brandCustom.value = brandHidden.value;
    }
    updateBrandState();
  }

  validateRequiredFields();
});
