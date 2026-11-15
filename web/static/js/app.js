const form = document.querySelector('.request-form');
const photosInput = document.querySelector('#photos');
const summary = document.querySelector('#photo-summary');
const submitButton = document.querySelector('#submit-button');
const nameInput = document.querySelector('#name');
const phoneInput = document.querySelector('#phone');
const emailInput = document.querySelector('#email');
const maxFiles = 5;
const maxSingle = 5 * 1024 * 1024;

function formatBytes(size) {
  return `${(size / 1024 / 1024).toFixed(1)} МБ`;
}

function updateSummary() {
  if (!photosInput || !summary) return;
  const files = Array.from(photosInput.files || []);
  if (!files.length) {
    summary.textContent = '';
    return;
  }

  if (files.length > maxFiles) {
    summary.textContent = 'Можно прикрепить не более 5 фотографий.';
    return;
  }

  const oversized = files.find((file) => file.size > maxSingle);
  if (oversized) {
    summary.textContent = 'Размер файла превышает допустимый лимит.';
    return;
  }

  summary.textContent = files.map((file) => `${file.name} (${formatBytes(file.size)})`).join(', ');
}

if (photosInput) {
  if (!photosInput.multiple) {
    photosInput.multiple = true;
  }

  photosInput.addEventListener('change', () => {
    const files = Array.from(photosInput.files || []);
    if (!files.length) {
      updateSummary();
      return;
    }

    if (files.length > maxFiles) {
      photosInput.value = '';
      summary.textContent = 'Можно прикрепить не более 5 фотографий.';
      return;
    }

    if (files.some((file) => file.size > maxSingle)) {
      photosInput.value = '';
      summary.textContent = 'Размер файла превышает допустимый лимит.';
      return;
    }

    updateSummary();
  });
}

function validateRequiredFields() {
  const nameFilled = nameInput && nameInput.value.trim() !== '';
  const phoneFilled = phoneInput && phoneInput.value.trim() !== '';
  const emailFilled = emailInput && emailInput.value.trim() !== '';

  if (nameInput) nameInput.classList.toggle('invalid', !nameFilled);
  if (phoneInput) phoneInput.classList.toggle('invalid', !phoneFilled);
  if (emailInput) emailInput.classList.toggle('invalid', !emailFilled);

  if (submitButton) submitButton.disabled = !(nameFilled && phoneFilled && emailFilled);
}

if (nameInput) nameInput.addEventListener('input', validateRequiredFields);
if (phoneInput) phoneInput.addEventListener('input', validateRequiredFields);
if (emailInput) emailInput.addEventListener('input', validateRequiredFields);
validateRequiredFields();

if (form) {
  form.addEventListener('submit', () => {
    if (submitButton) {
      submitButton.disabled = true;
      submitButton.textContent = 'Отправляем...';
    }
  });
}
