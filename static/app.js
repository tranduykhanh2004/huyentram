// Minimal frontend JS cho trang linktree & admin
const currencyFormatter = new Intl.NumberFormat('vi-VN',{style:'currency',currency:'VND'});
const tokenKey = 'admin_token';
const tokenFromURL = new URLSearchParams(window.location.search).get('token');
if(tokenFromURL){ sessionStorage.setItem(tokenKey, tokenFromURL); }
const adminToken = sessionStorage.getItem(tokenKey) || '';

let allProducts = [];
let filterText = '';
let filterTab = 'my'; // 'my' = My Choice (no external link), 'shopee' = items with external_url
let filterCategory = 0; // 0 = all

function authedFetch(url, options={}){
  const opts = {...options};
  let headers = options.headers instanceof Headers ? options.headers : new Headers(options.headers || {});
  if(adminToken){
    headers.set('X-Admin-Token', adminToken);
  }
  opts.headers = headers;
  // ensure cookies are sent for same-origin requests and make auth explicit
  opts.credentials = opts.credentials || 'same-origin';
  return fetch(url, opts);
}

function escapeHtml(str){
  return String(str).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;').replace(/'/g,'&#039;');
}

function linkifyText(text){
  if(!text) return '';
  // escape first
  let out = escapeHtml(text);
  // linkify URLs
  out = out.replace(/(https?:\/\/[^\s]+)/g, '<a href="$1" target="_blank" rel="noreferrer">$1</a>');
  // linkify @handles (instagram)
  out = out.replace(/@([a-zA-Z0-9_\.]+)/g, function(_,uname){
    const url = 'https://www.instagram.com/' + uname;
    return '<a href="'+url+'" target="_blank" rel="noreferrer">@'+uname+'</a>';
  });
  // allow newlines -> <br>
  out = out.replace(/\r?\n/g, '<br>');
  return out;
}

// small modal helpers that return Promises
function showActionModal({message = '', placeholder = '', defaultValue = '', input = false}){
  return new Promise((resolve)=>{
    const modal = document.getElementById('action-modal');
    const msg = document.getElementById('action-modal-message');
    const inputEl = document.getElementById('action-modal-input');
    const btnOk = document.getElementById('action-modal-confirm');
    const btnCancel = document.getElementById('action-modal-cancel');
    msg.textContent = message || '';
    if(input){
      inputEl.classList.remove('hidden');
      inputEl.placeholder = placeholder || '';
      inputEl.value = defaultValue || '';
      setTimeout(()=> inputEl.focus(), 50);
    } else {
      inputEl.classList.add('hidden');
    }
    function cleanup(){
      modal.classList.add('hidden');
      btnOk.removeEventListener('click', onOk);
      btnCancel.removeEventListener('click', onCancel);
      document.removeEventListener('keydown', onKey);
    }
    function onOk(){
      const val = input ? inputEl.value : true;
      cleanup();
      resolve(val);
    }
    function onCancel(){ cleanup(); resolve(input ? null : false); }
    function onKey(e){ if(e.key === 'Escape'){ onCancel(); } if(e.key === 'Enter' && input){ onOk(); } }
    btnOk.addEventListener('click', onOk);
    btnCancel.addEventListener('click', onCancel);
    document.addEventListener('keydown', onKey);
    modal.classList.remove('hidden');
  });
}

function showConfirm(msg){ return showActionModal({message: msg, input:false}); }
function showPrompt(msg, defaultValue=''){ return showActionModal({message: msg, input:true, defaultValue}); }

function formatPrice(value){
  const num = Number(value);
  if(Number.isNaN(num) || num <= 0) return 'Liên hệ';
  return currencyFormatter.format(num);
}

async function loadProfile(populateForm=false){
  try{
    const fetchFn = populateForm ? authedFetch : fetch;
    const res = await fetchFn('/api/profile');
    if(!res.ok) return;
    const data = await res.json();
    const nameEl = document.getElementById('profile-name');
    const handleEl = document.getElementById('profile-username');
    const highlightEl = document.getElementById('profile-highlight');
    const bioEl = document.getElementById('profile-bio');
    const avatarEl = document.getElementById('profile-avatar');
  if(nameEl) nameEl.textContent = data.display_name || 'Shop nhỏ';
  if(handleEl) handleEl.textContent = data.username || '@shop';
  if(highlightEl) highlightEl.textContent = data.highlight || '';
  if(bioEl) bioEl.innerHTML = linkifyText(data.bio || '');
  if(avatarEl && data.avatar_url) avatarEl.src = data.avatar_url;
  // Render fixed social icons: Instagram (use profile username if present), Facebook, TikTok.
  const socialContainer = document.getElementById('social-icons');
  if (socialContainer) {
    socialContainer.innerHTML = '';
    const uname = (data.username || '').replace(/^@/, '');
    // try to pick URLs from data.socials when available
    const socialsMap = {};
    (data.socials || []).forEach(s => { if (s && s.name) socialsMap[s.name.toLowerCase()] = s; });
    const instaURL = socialsMap['instagram']?.url || (uname ? 'https://www.instagram.com/_huientram?igsh=NWVxb3NpbWRheTl2&utm_source=qr' + uname : 'https://www.instagram.com');
    const fbURL = socialsMap['facebook']?.url || 'https://www.facebook.com/ty.tung.180?mibextid=wwXIfr&rdid=06MPlWdqSOX9lOCp&share_url=https%3A%2F%2Fwww.facebook.com%2Fshare%2F1ZS1NpLBL3%2F%3Fmibextid%3DwwXIfr';
    const ttURL = socialsMap['tiktok']?.url || 'https://www.tiktok.com/@huyentram0206?_r=1&_t=ZS-91kThaRWClJ';

    const fixed = [
      { name: 'Instagram', url: instaURL, icon: 'instagram.svg' },
      { name: 'Facebook', url: fbURL, icon: 'facebook.svg' },
      { name: 'TikTok', url: ttURL, icon: 'tiktok.svg' },
    ];
    fixed.forEach(s => {
      const a = document.createElement('a');
      a.href = s.url || '#';
      a.target = '_blank';
      a.rel = 'noreferrer';
      a.className = 'social';
      a.setAttribute('aria-label', s.name || 'social');
      const img = document.createElement('img');
      img.src = '/static/img/' + s.icon;
      img.alt = s.name || '';
  img.width = 40;
  img.height = 40;
      img.style.objectFit = 'contain';
      a.appendChild(img);
      socialContainer.appendChild(a);
    });
  }

    if(populateForm){
      const profileForm = document.getElementById('profile-form');
      if(profileForm){
        profileForm.querySelector('[name="display_name"]').value = data.display_name || '';
        profileForm.querySelector('[name="username"]').value = data.username || '';
        profileForm.querySelector('[name="bio"]').value = data.bio || '';
        profileForm.querySelector('[name="highlight"]').value = data.highlight || '';
      }
    }
  }catch(err){
    console.error('profile load error', err);
  }
}

// ---------------- Socials admin helpers ----------------
async function loadAdminSocials(){
  try{
    const res = await authedFetch('/api/socials');
    if(!res.ok) return [];
    const data = await res.json();
    renderAdminSocials(data);
    return data;
  }catch(err){ console.error('loadAdminSocials error', err); return []; }
}

function renderAdminSocials(socs){
  const list = document.getElementById('social-list');
  if(!list) return;
  list.innerHTML = '';
  if(!socs || socs.length === 0){ list.innerHTML = '<div class="muted">Chưa có social nào.</div>'; return; }
  socs.forEach(s=>{
    const row = document.createElement('div');
    row.className = 'social-row';
    row.innerHTML = `
      <div style="display:flex;gap:12px;align-items:center">
        ${s.icon?`<img src="/static/img/${s.icon}" style="width:40px;height:40px;object-fit:contain">`:`<div style="width:40px;height:40px;background:#eee"></div>`}
        <div style="flex:1">
          <strong>${escapeHtml(s.name)}</strong>
          <div style="color:#666">${escapeHtml(s.url)}</div>
        </div>
        <div style="display:flex;gap:8px">
          <button class="btn-edit-social" data-id="${s.id}">Edit</button>
          <button class="btn-delete-social" data-id="${s.id}">Delete</button>
        </div>
      </div>`;
    list.appendChild(row);
  });

  list.querySelectorAll('.btn-delete-social').forEach(b=>{
    b.addEventListener('click', async (ev)=>{
      const id = ev.currentTarget.dataset.id;
      const ok = await showConfirm('Xóa social này?');
      if(!ok) return;
      const res = await authedFetch('/api/socials/'+id, {method:'DELETE'});
      if(res.ok){ loadAdminSocials(); loadProfile(true); }
      else{ const txt = await res.text().catch(()=>'<no body>'); alert('Xóa thất bại: '+txt); }
    });
  });

  list.querySelectorAll('.btn-edit-social').forEach(b=>{
    b.addEventListener('click', async (ev)=>{
      const id = ev.currentTarget.dataset.id;
      const res = await authedFetch('/api/socials');
      if(!res.ok) return;
      const data = await res.json();
      const s = data.find(x=>String(x.id) === String(id));
      if(!s) return;
      const form = document.getElementById('social-form');
      form.querySelector('[name="id"]').value = s.id;
      form.querySelector('[name="name"]').value = s.name;
      form.querySelector('[name="url"]').value = s.url;
      form.querySelector('[name="icon"]').value = s.icon || '';
      form.scrollIntoView({behavior:'smooth'});
    });
  });
}

async function loadStaticImgs(){
  try{
    const res = await authedFetch('/api/static-imgs');
    if(!res.ok) return [];
    const arr = await res.json();
    const sel = document.getElementById('social-icon-select');
    if(sel){
      sel.innerHTML = '<option value="">— chọn icon —</option>' + arr.map(n=>`<option value="${n}">${n}</option>`).join('');
    }
    return arr;
  }catch(err){ console.error('loadStaticImgs error', err); return []; }
}

// Category UI removed: public/categories are managed directly in the DB now.

async function listProducts(){
  const res = await fetch('/api/products');
  if(!res.ok){
    const txt = await res.text().catch(()=>'<no body>');
    console.error('listProducts failed', res.status, txt);
    allProducts = [];
    renderProducts();
    return;
  }
  const data = await res.json();
  allProducts = data.map(p => ({
    ...p,
    category_id: Number(p.category_id || 0),
    category: p.category || ''
  }));
  renderProducts();
}

function renderProducts(){
  const el = document.getElementById('products');
  if(!el) return;
  const normalized = filterText.trim().toLowerCase();
  const filtered = allProducts.filter(p=>{
    const matchText = !normalized || (p.title && p.title.toLowerCase().includes(normalized)) || (p.description && p.description.toLowerCase().includes(normalized));
    let matchTab = true;
    if(filterTab === 'my'){
      matchTab = (p.tag || 'mychoice') === 'mychoice';
    } else if(filterTab === 'shopee'){
      matchTab = (p.tag || '') === 'shopee';
    }
    let matchCategory = true;
    if(filterCategory && Number(p.category_id || 0) !== Number(filterCategory)){
      matchCategory = false;
    }
    return matchText && matchTab && matchCategory;
  });
  el.innerHTML = '';
  if(!filtered.length){
    el.innerHTML = `<div class="empty-state">Không tìm thấy sản phẩm phù hợp.</div>`;
    return;
  }
  filtered.forEach((p)=>{
    const card = document.createElement('div');
    card.className = 'link-card';
    card.dataset.id = p.id;
    card.innerHTML = `
      <div class="thumb">
        ${p.image_url
          ? `<img src="${p.image_url}" alt="${p.title}">`
          : `<span class="thumb-placeholder">No img</span>`}
      </div>
      <div class="info">
        <p class="title">${p.title}</p>
        <p class="desc">${p.description || 'Đang cập nhật mô tả chi tiết.'}</p>
        <span class="price">${formatPrice(p.price)}${p.category ? ` • ${p.category}` : ''}</span>
  ${p.tag === 'shopee' && p.external_url ? `<div style="margin-top:0.6rem"><a class="btn ghost" href="${p.external_url}" target="_blank" rel="noreferrer">Mua trên Shopee</a></div>` : ''}
      </div>`;
    card.addEventListener('click', ()=> showProductModal(p));
    el.appendChild(card);
  });
}

function showProductModal(p){
  const modal = document.getElementById('product-modal');
  const body = document.getElementById('modal-body');
  if(!modal || !body) return;
  body.innerHTML = `
    ${p.image_url
      ? `<img src="${p.image_url}" alt="${p.title}">`
      : `<div class="thumb-placeholder" style="height:280px;border-radius:16px">No image</div>`}
    <h3>${p.title}</h3>
    <p>${p.description||'Đang cập nhật mô tả chi tiết.'}</p>
    <p class="price" style="margin-top:1rem;font-size:1.2rem">${formatPrice(p.price)}</p>
    ${p.category ? `<p style="color:#7b8191">Danh mục: ${p.category}</p>` : ''}
  ${p.tag === 'shopee' && p.external_url ? `<div style="margin-top:0.8rem"><a class="btn primary" href="${p.external_url}" target="_blank" rel="noreferrer">Mua trên Shopee</a></div>` : (p.tag !== 'shopee' ? `<div style="margin-top:1.2rem"><a class="btn primary" href="https://www.instagram.com/${(document.getElementById('profile-username')?.textContent||'').replace(/^@/,'')}" target="_blank" rel="noreferrer">Nhắn Instagram để chốt</a></div>` : '')}
  `;
  modal.classList.remove('hidden');
  modal.classList.add('open');
}

// ----- Category admin helpers -----
async function loadCategories(){
  try{
    const res = await authedFetch('/api/categories');
    if(!res.ok) return [];
    const cats = await res.json();
    renderCategories(cats);
    // populate product category select
    const sel = document.getElementById('product-category');
    if(sel){
      sel.innerHTML = '<option value="0">— Chọn danh mục —</option>' + cats.map(c=>`<option value="${c.id}">${c.name}</option>`).join('');
    }
    return cats;
  }catch(err){ console.error('loadCategories error', err); return []; }
}

// Public-facing category chips (visible on index.html)
async function loadPublicCategories(){
  try{
    const res = await fetch('/api/categories');
    if(!res.ok) return [];
    const cats = await res.json();
    const chips = document.getElementById('category-chips');
    if(!chips) return cats;
    chips.innerHTML = '';
    const allBtn = document.createElement('button');
    allBtn.className = 'chip' + (filterCategory ? '' : ' active');
    allBtn.textContent = 'Tất cả';
    allBtn.addEventListener('click', ()=>{ filterCategory = 0; document.querySelectorAll('#category-chips .chip').forEach(x=>x.classList.remove('active')); allBtn.classList.add('active'); renderProducts(); });
    chips.appendChild(allBtn);
    cats.forEach(c=>{
      const b = document.createElement('button');
      b.className = 'chip' + (Number(filterCategory) === Number(c.id) ? ' active' : '');
      b.textContent = c.name;
      b.dataset.id = c.id;
      b.addEventListener('click', ()=>{
        filterCategory = Number(c.id);
        document.querySelectorAll('#category-chips .chip').forEach(x=>x.classList.remove('active'));
        b.classList.add('active');
        renderProducts();
      });
      chips.appendChild(b);
    });
    return cats;
  }catch(err){ console.error('loadPublicCategories error', err); return []; }
}

function renderCategories(cats){
  const list = document.getElementById('category-list');
  if(!list) return;
  list.innerHTML = '';
  if(!cats || cats.length === 0){
    list.innerHTML = '<div class="muted">Chưa có danh mục nào.</div>';
    return;
  }
  cats.forEach(c=>{
    const row = document.createElement('div');
    row.className = 'cat-row';
    row.innerHTML = `
      <div>${c.name}</div>
      <div class="actions">
        <button class="btn-edit" data-id="${c.id}" data-name="${escapeHtml(c.name)}">Sửa</button>
        <button class="btn-delete" data-id="${c.id}">Xóa</button>
      </div>`;
    list.appendChild(row);
  });
  // wire edit/delete
  list.querySelectorAll('.btn-delete').forEach(b=>{
    b.addEventListener('click', async (ev)=>{
      const id = ev.currentTarget.dataset.id;
      const ok = await showConfirm('Xóa danh mục? Sản phẩm liên quan sẽ mất danh mục.');
      if(!ok) return;
      const res = await authedFetch('/api/admin/delete-category',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({id: Number(id)})});
      if(res.ok){ loadCategories(); adminLoadProducts(); listProducts(); }
      else { const txt = await res.text().catch(()=>'<no body>'); alert('Xóa thất bại: '+txt); }
    });
  });
  list.querySelectorAll('.btn-edit').forEach(b=>{
    b.addEventListener('click', async (ev)=>{
      const id = ev.currentTarget.dataset.id;
      const name = ev.currentTarget.dataset.name;
      const newName = await showPrompt('Chỉnh tên danh mục', name);
      if(!newName) return;
      const res = await authedFetch('/api/categories/'+id, {method:'PUT', headers:{'Content-Type':'application/json'}, body: JSON.stringify({name: newName})});
      if(res.ok){ loadCategories(); adminLoadProducts(); listProducts(); }
      else{ const txt = await res.text().catch(()=>'<no body>'); alert('Cập nhật thất bại: '+txt); }
    });
  });
}

// Toggle external_url field visibility depending on selected tag
function toggleExternalField(){
  const tagSel = document.getElementById('product-tag');
  const extWrap = document.querySelector('.external-field');
  if(!tagSel || !extWrap) return;
  if(tagSel.value === 'shopee'){
    extWrap.classList.remove('hidden');
    extWrap.querySelector('input').required = true;
  } else {
    extWrap.classList.add('hidden');
    extWrap.querySelector('input').required = false;
    extWrap.querySelector('input').value = '';
  }
}


document.addEventListener('click', (e)=>{
  if(e.target && e.target.id === 'modal-close'){
    const modal = document.getElementById('product-modal');
    if(modal) modal.classList.add('hidden');
  }
});

// close modal on backdrop click or ESC
document.addEventListener('click', (e)=>{
  const modal = document.getElementById('product-modal');
  if(!modal) return;
  if(e.target === modal){
    modal.classList.add('hidden');
  }
});
document.addEventListener('keydown', (e)=>{
  if(e.key === 'Escape'){
    const modal = document.getElementById('product-modal');
    if(modal) modal.classList.add('hidden');
  }
});

// --- Admin helpers: render list với edit/delete, handle edit flow ---
async function adminLoadProducts(){
  const container = document.getElementById('admin-products');
  if(!container) return;
  const res = await authedFetch('/api/products');
  if(!res.ok){
    const txt = await res.text().catch(()=>'<no body>');
    console.error('adminLoadProducts failed', res.status, txt);
    container.innerHTML = `<div class="empty-state">Không tải được sản phẩm (status ${res.status})<br><small>${txt}</small></div>`;
    return;
  }
  const data = await res.json();
  container.innerHTML = '';
  for(const p of data){
    const row = document.createElement('div');
    row.className = 'card product';
    row.style.padding = '8px';
    row.innerHTML = `
      <div style="display:flex;gap:12px;align-items:center">
        ${p.image_url?`<img src="${p.image_url}" style="width:120px;height:80px;object-fit:cover">`:`<div style="width:120px;height:80px;background:#eee"></div>`}
        <div style="flex:1">
          <strong>${p.title}</strong>
          <div style="color:#666">${p.description||''}</div>
          ${p.category ? `<div style="color:#999;font-size:.85rem">${p.category}</div>`:''}
          <div style="color:#999;font-size:.85rem">Tag: ${p.tag || 'mychoice'}</div>
        </div>
        <div style="display:flex;gap:8px">
          <button class="btn btn-edit" data-id="${p.id}">Edit</button>
          <button class="btn btn-delete" data-id="${p.id}">Delete</button>
        </div>
      </div>`;
    container.appendChild(row);
  }
  container.querySelectorAll('.btn-delete').forEach(b=>{
    b.addEventListener('click', async (ev)=>{
      const id = ev.currentTarget.dataset.id;
      const ok = await showConfirm('Delete product?');
      if(!ok) return;
      // Use POST JSON admin endpoint to avoid DELETE/cors/cookie issues
      const res = await authedFetch('/api/admin/delete-product', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({id: Number(id)})
      });
      if(res.ok){
        adminLoadProducts(); listProducts();
        alert('Deleted');
      } else {
        const txt = await res.text().catch(()=>'<no body>');
        console.error('delete failed', res.status, txt);
        alert('Delete failed: ' + res.status + '\n' + txt);
      }
    });
  });
  container.querySelectorAll('.btn-edit').forEach(b=>{
    b.addEventListener('click', async (ev)=>{
      const id = ev.currentTarget.dataset.id;
      // open dedicated edit modal populated with product data
      const res = await authedFetch('/api/products/'+id);
      if(!res.ok){ alert('Product not found'); return; }
      const p = await res.json();
      showProductEditModal(p);
    });
  });
}

// showProductEditModal opens the edit modal and populates fields
function showProductEditModal(p){
  const modal = document.getElementById('product-edit-modal');
  const form = document.getElementById('product-edit-form');
  if(!modal || !form) return;
  form.querySelector('[name="id"]').value = p.id || '';
  form.querySelector('[name="title"]').value = p.title || '';
  form.querySelector('[name="description"]').value = p.description || '';
  form.querySelector('[name="price"]').value = p.price || '';
  const catSel = document.getElementById('edit-product-category');
  if(catSel) catSel.value = p.category_id || 0;
  const tagSel = document.getElementById('edit-product-tag');
  if(tagSel) tagSel.value = p.tag || 'mychoice';
  const ext = form.querySelector('[name="external_url"]');
  if(ext) ext.value = p.external_url || '';
  // reset file chooser display
  const editFileEl = document.getElementById('edit-product-file');
  if(editFileEl) editFileEl.value = '';
  const chosenEditFileNameEl = document.getElementById('chosen-edit-file-name');
  if(chosenEditFileNameEl) chosenEditFileNameEl.textContent = 'Chưa chọn tệp nào';
  modal.classList.remove('hidden');
  modal.classList.add('open');
  // ensure category options are synced
  loadCategories().then(cats=>{
    const sel = document.getElementById('edit-product-category');
    if(sel){
      sel.innerHTML = '<option value="0">— Chọn danh mục —</option>' + cats.map(c=>`<option value="${c.id}">${c.name}</option>`).join('');
      sel.value = p.category_id || 0;
    }
  }).catch(()=>{});
}

// wire edit modal interactions
document.addEventListener('DOMContentLoaded', ()=>{
  const editModal = document.getElementById('product-edit-modal');
  const editForm = document.getElementById('product-edit-form');
  const closeBtn = document.getElementById('edit-modal-close');
  if(closeBtn && editModal){ closeBtn.addEventListener('click', ()=> editModal.classList.add('hidden')); }
  // file chooser in modal
  const chooseEditBtn = document.getElementById('choose-edit-file-btn');
  const editFileInput = document.getElementById('edit-product-file');
  const editFileName = document.getElementById('chosen-edit-file-name');
  if(chooseEditBtn && editFileInput && editFileName){
    chooseEditBtn.addEventListener('click', ()=> editFileInput.click());
    editFileInput.addEventListener('change', ()=>{
      const f = editFileInput.files && editFileInput.files[0];
      editFileName.textContent = f ? f.name : 'Chưa chọn tệp nào';
    });
  }
  if(editForm){
    editForm.addEventListener('submit', async (e)=>{
      e.preventDefault();
      const id = editForm.querySelector('[name="id"]').value;
      const fd = new FormData(editForm);
      try{
        const res = await authedFetch('/api/products/'+id, {method:'PUT', body: fd});
        if(res.ok){
          alert('Product updated');
          editModal.classList.add('hidden');
          adminLoadProducts(); listProducts();
        } else {
          const txt = await res.text().catch(()=>'<no body>');
          alert('Update failed: '+txt);
        }
      }catch(err){ console.error('edit submit error', err); alert('Request failed'); }
    });
  }
});

// Admin category management removed from frontend to simplify UI. Categories are edited directly in the DB.

document.addEventListener('DOMContentLoaded', ()=>{
  loadProfile();
  listProducts();
  const productForm = document.getElementById('product-form');
  const adminPanel = document.getElementById('admin-panel');
  const profileForm = document.getElementById('profile-form');
  const categoryForm = document.getElementById('category-form');
  const searchInput = document.getElementById('product-search');

  if(searchInput){
    searchInput.addEventListener('input', (e)=>{
      filterText = e.target.value;
      renderProducts();
    });
  }

  if(adminPanel){
    adminLoadProducts();
    loadProfile(true);
    loadCategories();
    if(!adminToken){
      const warn = document.getElementById('token-warning');
      if(warn) warn.classList.remove('hidden');
    }
    // attach delegated click handler for admin product actions (edit/delete)
    const adminProductsContainer = document.getElementById('admin-products');
    if(adminProductsContainer && !adminProductsContainer.dataset.delegationAttached){
      adminProductsContainer.addEventListener('click', async (ev)=>{
        const editBtn = ev.target.closest('.btn-edit');
        const delBtn = ev.target.closest('.btn-delete');
        if(editBtn && adminProductsContainer.contains(editBtn)){
          ev.preventDefault(); ev.stopPropagation();
          const id = editBtn.dataset.id;
          if(!id) return;
          const res = await authedFetch('/api/products/'+id);
          if(!res.ok){ alert('Product not found'); return; }
          const p = await res.json();
          showProductEditModal(p);
          return;
        }
        if(delBtn && adminProductsContainer.contains(delBtn)){
          ev.preventDefault(); ev.stopPropagation();
          const id = delBtn.dataset.id;
          if(!id) return;
          const ok = await showConfirm('Delete product?');
          if(!ok) return;
          const res = await authedFetch('/api/admin/delete-product', {method: 'POST', headers: {'Content-Type':'application/json'}, body: JSON.stringify({id: Number(id)})});
          if(res.ok){ adminLoadProducts(); listProducts(); alert('Deleted'); }
          else{ const txt = await res.text().catch(()=>'<no body>'); alert('Delete failed: '+txt); }
          return;
        }
      });
      adminProductsContainer.dataset.delegationAttached = '1';
    }
  }
  // public categories (index page)
  loadPublicCategories();

  // topbar removed — no copy/back handlers needed

  // tabs behavior: Shopee opens external shop
  document.querySelectorAll('.tab').forEach(t=>{
    t.addEventListener('click', (ev)=>{
      const el = ev.currentTarget;
      document.querySelectorAll('.tab').forEach(x=>x.classList.remove('active'));
      el.classList.add('active');
      const txt = el.textContent.trim().toLowerCase();
      if(txt === 'shopee') filterTab = 'shopee';
      else filterTab = 'my';
      renderProducts();
    });
  });

  if(productForm){
    productForm.addEventListener('submit', async (e)=>{
      e.preventDefault();
      const fd = new FormData(productForm);
      const editId = document.getElementById('product-id').value;
      // client-side: ensure shopee tag includes a link
      const tagVal = fd.get('tag') || 'mychoice';
      const ext = (fd.get('external_url') || '').toString().trim();
      if(tagVal === 'shopee' && ext === ''){
        alert('Vui lòng nhập link bán hàng khi chọn tag Shopee');
        return;
      }
      if(editId){
        const res = await authedFetch('/api/products/'+editId, {method:'PUT', body: fd});
        if(res.ok){
          alert('Product updated');
          productForm.reset();
          document.getElementById('product-id').value = '';
          adminLoadProducts();
          listProducts();
        } else {
          const txt = await res.text();
          alert('Error: '+txt);
        }
      } else {
        const res = await authedFetch('/api/products',{method:'POST',body:fd});
        if(res.ok){
          alert('Product added');
          productForm.reset();
          adminLoadProducts();
          listProducts();
        } else {
          const txt = await res.text();
          alert('Error: '+txt);
        }
      }

      // wire custom file chooser button and display filename
      const chooseBtn = document.getElementById('choose-file-btn');
      const fileInput = document.getElementById('product-file');
      const fileNameEl = document.getElementById('chosen-file-name');
      if(chooseBtn && fileInput){
        chooseBtn.addEventListener('click', ()=> fileInput.click());
        fileInput.addEventListener('change', ()=>{
          const f = fileInput.files && fileInput.files[0];
          if(f){
            fileNameEl.textContent = f.name;
          } else {
            fileNameEl.textContent = 'Chưa chọn tệp nào';
          }
        });
      }

      // avatar chooser wiring
      const chooseAvatarBtn = document.getElementById('choose-avatar-btn');
      const avatarInput = document.getElementById('avatar-file');
      const avatarNameEl = document.getElementById('chosen-avatar-name');
      if(chooseAvatarBtn && avatarInput){
        chooseAvatarBtn.addEventListener('click', ()=> avatarInput.click());
        avatarInput.addEventListener('change', ()=>{
          const f = avatarInput.files && avatarInput.files[0];
          if(f){ avatarNameEl.textContent = f.name; } else { avatarNameEl.textContent = 'Chưa chọn ảnh'; }
        });
      }
    });
  }

  if(categoryForm){
    categoryForm.addEventListener('submit', async (e)=>{
      e.preventDefault();
      const name = categoryForm.querySelector('[name="name"]').value.trim();
      if(!name) return;
      const res = await authedFetch('/api/categories',{method:'POST',headers:{'Content-Type':'application/json'},body: JSON.stringify({name})});
      if(res.ok){ categoryForm.reset(); loadCategories(); adminLoadProducts(); listProducts(); }
      else{ const txt = await res.text().catch(()=>'<no body>'); alert('Add category failed: '+txt); }
    });
  }

  // tag select toggles external field
  const tagSel = document.getElementById('product-tag');
  if(tagSel){ tagSel.addEventListener('change', toggleExternalField); toggleExternalField(); }

  if(profileForm){
    profileForm.addEventListener('submit', async (e)=>{
      e.preventDefault();
      const fd = new FormData(profileForm);
      const res = await authedFetch('/api/profile',{method:'PUT',body:fd});
      if(res.ok){
        alert('Profile updated');
        loadProfile(true);
      } else {
        const txt = await res.text();
        alert('Update failed: '+txt);
      }
    });
  }

  // no admin social CRUD UI — socials are fixed to FB/IG/TikTok and use icons in static/img

  if(categoryForm){
    // categoryForm removed from UI; no handler needed
  }
});

