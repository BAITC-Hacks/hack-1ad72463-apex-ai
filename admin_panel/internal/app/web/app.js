'use strict';
const $ = (id) => document.getElementById(id);
const state = { user: null, vendors: [], revision: '', metadata: null, formats: [], languages: [], edit: null, deletion: null, preview: null };
let toastTimer;
function toast(message, error = false) { $('toast').textContent = message; $('toast').classList.toggle('error', error); $('toast').hidden = false; clearTimeout(toastTimer); toastTimer = setTimeout(() => { $('toast').hidden = true; }, 5000); }
function authView(password = false) { $('auth-view').hidden = false; $('workspace').hidden = true; $('login-form').hidden = password; $('password-form').hidden = !password; document.querySelectorAll('dialog[open]').forEach(d => d.close()); }
async function api(path, { method = 'GET', data, revision } = {}) {
  const headers = {};
  if (method !== 'GET') headers['X-CSRF-Token'] = state.user?.csrf_token || '';
  if (revision) headers['If-Match'] = '"' + revision + '"';
  let body;
  if (data instanceof FormData) body = data;
  else if (data !== undefined) { headers['Content-Type'] = 'application/json'; body = JSON.stringify(data); }
  const response = await fetch('/admin/api/' + path, { method, headers, body, credentials: 'same-origin' });
  const payload = await response.json().catch(() => null);
  if (!response.ok) {
    if (response.status === 401 && path !== 'login') { state.user = null; authView(); }
    const error = new Error(payload?.error?.message || 'Сервер недоступен. Повторите запрос.'); error.code = payload?.error?.code; throw error;
  }
  return payload;
}
async function busy(button, task, errorId) { const old = button.textContent; button.disabled = true; button.textContent = 'Подождите…'; if (errorId) $(errorId).textContent = ''; try { await task(); } catch (error) { if (errorId) $(errorId).textContent = error.message; else toast(error.message, true); } finally { button.disabled = false; button.textContent = old; } }
async function enter() { if (state.user.must_change_password) { authView(true); return; } $('auth-view').hidden = true; $('workspace').hidden = false; $('username').textContent = state.user.username; await loadCatalog(); }
$('login-form').addEventListener('submit', event => { event.preventDefault(); const form = event.currentTarget; busy(form.querySelector('button[type=submit]'), async () => { state.user = await api('login', { method: 'POST', data: { username: form.username.value.trim(), password: form.password.value } }); form.password.value = ''; await enter(); }, 'login-error'); });
$('password-form').addEventListener('submit', event => { event.preventDefault(); const form = event.currentTarget; busy(form.querySelector('button[type=submit]'), async () => { if (form.new_password.value !== form.confirm_password.value) throw new Error('Пароли не совпадают.'); await api('password', { method: 'POST', data: { current_password: form.current_password.value, new_password: form.new_password.value } }); state.user = null; form.reset(); authView(); toast('Пароль обновлён. Войдите с новым паролем.'); }, 'password-error'); });
async function logout() { try { await api('logout', { method: 'POST' }); state.user = null; authView(); } catch (error) { toast(error.message, true); } }
$('logout').addEventListener('click', logout); $('password-back').addEventListener('click', logout); $('change-password').addEventListener('click', () => authView(true));
function options(select, values, first) { const previous = select.value; select.replaceChildren(new Option(first, '')); values.forEach(value => select.add(new Option(value, value))); if (values.includes(previous)) select.value = previous; }
function unique(values) { return [...new Set(values)].sort((a, b) => a.localeCompare(b, 'ru')); }
async function loadCatalog() {
  $('catalog-error').hidden = true; $('refresh').disabled = true;
  try {
    const data = await api('vendors'); state.vendors = data.vendors; state.revision = data.revision; state.metadata = data.metadata; state.formats = data.event_formats; state.languages = data.languages;
    const cities = unique(state.vendors.map(v => v.city)), categories = unique(state.vendors.flatMap(v => v.categories));
    $('stat-total').textContent = state.vendors.length; $('stat-cities').textContent = cities.length; $('stat-categories').textContent = categories.length; $('stat-synthetic').textContent = state.vendors.filter(v => v.synthetic).length;
    options($('city-filter'), cities, 'Все города'); options($('category-filter'), categories, 'Все категории');
    for (const [id, values] of [['cities-list', cities], ['categories-list', categories]]) $(id).replaceChildren(...values.map(value => new Option(value, value)));
    const window = data.metadata.calendar_window; $('calendar-hint').textContent = 'Календарь занятости: ' + window.from + ' — ' + window.to; $('busy-hint').textContent = 'YYYY-MM-DD: с ' + window.from + ' по ' + window.to + '. Разделяйте даты символом |, запятой или новой строкой.';
    renderRows();
  } catch (error) { $('catalog-error').textContent = error.message; $('catalog-error').hidden = false; } finally { $('refresh').disabled = false; }
}
function element(tag, className, text) { const node = document.createElement(tag); if (className) node.className = className; if (text !== undefined) node.textContent = text; return node; }
function renderRows() {
  const query = $('search').value.toLocaleLowerCase('ru').trim(), city = $('city-filter').value, category = $('category-filter').value;
  const vendors = state.vendors.filter(v => (!city || v.city === city) && (!category || v.categories.includes(category)) && [v.anon_name, v.id, v.city, ...v.categories].join(' ').toLocaleLowerCase('ru').includes(query)).sort((a, b) => a.anon_name.localeCompare(b.anon_name, 'ru'));
  const fragment = document.createDocumentFragment();
  vendors.forEach(v => {
    const tr = element('tr'), who = element('td'), person = element('div', 'person-cell'), avatar = element('span', 'vendor-avatar', v.anon_name.split(/\s+/).slice(0, 2).map(s => s[0]).join('').toUpperCase()), name = element('div');
    name.append(element('strong', '', v.anon_name), element('small', '', v.id)); person.append(avatar, name); who.append(person); tr.append(who);
    const cats = element('td'); v.categories.forEach(c => cats.append(element('span', 'category-pill', c))); tr.append(cats, element('td', '', v.city), element('td', 'price', v.price_from_kzt.toLocaleString('ru-RU') + ' ₸'));
    const tags = element('td'); if (v.synthetic) tags.append(element('span', 'data-pill synthetic', 'Синтетика')); if (v.price_imputed) tags.append(element('span', 'data-pill imputed', 'Цена ~')); if (v.city_imputed) tags.append(element('span', 'data-pill imputed', 'Город ~')); if (!tags.childNodes.length) tags.append(element('span', 'data-origin', 'Исходный профиль')); tr.append(tags);
    const actions = element('td', 'row-actions'), edit = element('button', '', 'Изменить'), remove = element('button', 'delete-action', 'Удалить'); edit.setAttribute('aria-label', 'Изменить ' + v.anon_name); remove.setAttribute('aria-label', 'Удалить ' + v.anon_name); edit.addEventListener('click', () => openVendor(v)); remove.addEventListener('click', () => openDelete(v)); actions.append(edit, remove); tr.append(actions); fragment.append(tr);
  });
  $('vendor-rows').replaceChildren(fragment); $('visible-count').textContent = vendors.length; $('table-summary').textContent = 'Показано ' + vendors.length + ' из ' + state.vendors.length + ' профилей'; $('empty-state').hidden = vendors.length !== 0;
}
['search', 'city-filter', 'category-filter'].forEach(id => $(id).addEventListener('input', renderRows)); $('refresh').addEventListener('click', loadCatalog); $('reset-filters').addEventListener('click', () => { ['search', 'city-filter', 'category-filter'].forEach(id => { $(id).value = ''; }); renderRows(); });
function checkboxes(id, field, values, selected) { $(id).replaceChildren(...values.map(value => { const label = element('label'), input = document.createElement('input'); input.type = 'checkbox'; input.name = field; input.value = value; input.checked = selected.includes(value); label.append(input, document.createTextNode(value)); return label; })); }
function openVendor(v = null) {
  if (!state.metadata) return toast('Сначала загрузите каталог.', true);
  state.edit = v; const form = $('vendor-form'); form.reset(); $('vendor-error').textContent = ''; $('vendor-dialog-title').textContent = v ? 'Редактировать профиль' : 'Новый подрядчик';
  for (const field of ['id', 'anon_name', 'city', 'description']) form.elements[field].value = v?.[field] || '';
  form.elements.id.readOnly = Boolean(v); form.elements.price_from_kzt.value = v?.price_from_kzt || ''; form.elements.categories.value = v?.categories.join(' | ') || ''; form.elements.max_hours.value = v?.max_hours ?? ''; form.elements.busy_dates.value = v?.busy_dates.join(' | ') || '';
  ['synthetic', 'city_imputed', 'price_imputed'].forEach(field => { form.elements[field].checked = v?.[field] || false; });
  checkboxes('format-checkboxes', 'event_formats', state.formats, v?.event_formats || []); checkboxes('language-checkboxes', 'languages', state.languages, v?.languages || []); $('vendor-dialog').showModal();
}
$('add-vendor').addEventListener('click', () => openVendor());
$('vendor-form').addEventListener('submit', event => { event.preventDefault(); const form = event.currentTarget; busy(form.querySelector('button[type=submit]'), async () => {
  const data = new FormData(form), formats = data.getAll('event_formats'), languages = data.getAll('languages');
  if (!formats.length || !languages.length) throw new Error('Выберите хотя бы один формат мероприятия и один язык.');
  const price = Number(data.get('price_from_kzt')); if (!Number.isSafeInteger(price) || price <= 0) throw new Error('Цена должна быть положительным целым числом.');
  const vendor = { id: data.get('id').trim(), anon_name: data.get('anon_name').trim(), city: data.get('city').trim(), price_from_kzt: price, categories: data.get('categories').split('|').map(v => v.trim()).filter(Boolean), event_formats: formats, languages, max_hours: data.get('max_hours') === '' ? null : Number(data.get('max_hours')), busy_dates: data.get('busy_dates').split(/[|,\n]/).map(v => v.trim()).filter(Boolean), description: data.get('description').trim(), synthetic: data.has('synthetic'), price_imputed: data.has('price_imputed'), city_imputed: data.has('city_imputed') };
  await api(state.edit ? 'vendors/' + encodeURIComponent(state.edit.id) : 'vendors', { method: state.edit ? 'PUT' : 'POST', data: vendor, revision: state.revision }); $('vendor-dialog').close(); toast(state.edit ? 'Профиль обновлён.' : 'Подрядчик добавлен.'); await loadCatalog();
}, 'vendor-error'); });
function openDelete(v) { state.deletion = v; $('delete-name').textContent = v.anon_name + ' · ' + v.id; $('delete-error').textContent = ''; $('delete-dialog').showModal(); }
$('confirm-delete').addEventListener('click', event => busy(event.currentTarget, async () => { await api('vendors/' + encodeURIComponent(state.deletion.id), { method: 'DELETE', revision: state.revision }); $('delete-dialog').close(); toast('Профиль удалён из каталога.'); await loadCatalog(); }, 'delete-error'));
function resetPreview() { state.preview = null; $('import-preview').hidden = true; $('confirm-import').disabled = true; $('import-error').textContent = ''; }
$('open-import').addEventListener('click', () => { $('csv-file').value = ''; $('update-existing').checked = false; resetPreview(); $('import-dialog').showModal(); }); $('csv-file').addEventListener('change', resetPreview);
function uploadData() { const file = $('csv-file').files[0]; if (!file) throw new Error('Выберите CSV-файл.'); if (file.size > 5 * 1024 * 1024) throw new Error('Максимальный размер файла — 5 МБ.'); const data = new FormData(); data.append('file', file); data.append('update_existing', $('update-existing').checked ? 'true' : 'false'); return data; }
function importButton() { $('confirm-import').disabled = !state.preview || (state.preview.existing_count > 0 && !$('update-existing').checked); }
$('preview-import').addEventListener('click', event => busy(event.currentTarget, async () => { resetPreview(); state.preview = await api('import/preview', { method: 'POST', data: uploadData() }); $('import-new').textContent = state.preview.new_count; $('import-existing').textContent = state.preview.existing_count; $('import-sample').textContent = 'Первые строки: ' + state.preview.sample.map(v => v.anon_name + ' (' + v.id + ')').join(', '); $('import-preview').hidden = false; importButton(); }, 'import-error'));
$('update-existing').addEventListener('change', importButton);
$('confirm-import').addEventListener('click', event => busy(event.currentTarget, async () => { if (!state.preview) throw new Error('Сначала проверьте файл.'); await api('import', { method: 'POST', data: uploadData(), revision: state.preview.revision }); $('import-dialog').close(); toast('CSV загружен. Каталог обновлён.'); resetPreview(); await loadCatalog(); }, 'import-error').then(importButton));
document.querySelectorAll('[data-close]').forEach(button => button.addEventListener('click', () => button.closest('dialog').close()));
const actionNames = { create: 'Добавление профиля', update: 'Изменение профиля', delete: 'Удаление профиля', import: 'Загрузка CSV', password_changed: 'Смена пароля' };
async function loadAudit() { try { const events = await api('audit'); $('audit-list').replaceChildren(...events.map(event => { const item = element('article', 'audit-item'), time = element('time', '', new Date(event.created_at).toLocaleString('ru-RU')), content = element('div'); content.append(element('p', '', actionNames[event.action] || event.action), element('small', '', event.actor + (event.vendor_ids.length ? ' · ' + event.vendor_ids.join(', ') : ''))); item.append(time, content); return item; })); if (!events.length) $('audit-list').append(element('p', 'loading', 'Изменений пока нет.')); } catch (error) { toast(error.message, true); } }
function tab(audit) { $('catalog-view').hidden = audit; $('audit-view').hidden = !audit; $('nav-audit').classList.toggle('active', audit); $('nav-catalog').classList.toggle('active', !audit); $('crumb-title').textContent = audit ? 'История' : 'Каталог'; if (audit) loadAudit(); }
$('nav-catalog').addEventListener('click', () => tab(false)); $('nav-audit').addEventListener('click', () => tab(true)); $('refresh-audit').addEventListener('click', loadAudit);
(async () => { try { state.user = await api('session'); await enter(); } catch { authView(); } })();
