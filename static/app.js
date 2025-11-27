// Minimal frontend JS cho trang linktree & admin
const currencyFormatter = new Intl.NumberFormat('vi-VN',{style:'currency',currency:'VND'});
const tokenKey = 'admin_token';
const tokenFromURL = new URLSearchParams(window.location.search).get('token');
if(tokenFromURL){ sessionStorage.setItem(tokenKey, tokenFromURL); }
const adminToken = sessionStorage.getItem(tokenKey) || '';

let allProducts = [];
let filterText = '';
let filterCategory = 0;
let publicCategories = [];
let adminCategories = [];

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
    if(bioEl) bioEl.textContent = data.bio || '';
    if(avatarEl && data.avatar_url) avatarEl.src = data.avatar_url;

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

async function fetchPublicCategories(){
  try{
    const res = await fetch('/api/categories');
    if(!res.ok) return;
    const data = await res.json();
    publicCategories = data.map(c=>({...c,id:Number(c.id)}));
    renderCategoryChips();
  }catch(err){
    console.error('categories load error', err);
  }
}

function renderCategoryChips(){
  const chips = document.getElementById('category-chips');
  if(!chips) return;
  chips.innerHTML = '';
  const createChip = (id, label)=>{
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'chip'+(filterCategory===id?' active':'');
    btn.textContent = label;
    btn.addEventListener('click', ()=>{
      filterCategory = id;
      document.querySelectorAll('#category-chips .chip').forEach(ch=>ch.classList.remove('active'));
      btn.classList.add('active');
      renderProducts();
    });
    chips.appendChild(btn);
  };
  createChip(0,'Tất cả');
  publicCategories.forEach(cat=> createChip(cat.id, cat.name));
}

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
    const matchCat = filterCategory === 0 || p.category_id === filterCategory;
    return matchText && matchCat;
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
    <div style="margin-top:1.2rem">
      <a class="btn primary" href="https://www.instagram.com/_huientram?igsh=NWVxb3NpbWRheTl2&utm_source=qr" target="_blank" rel="noreferrer">Nhắn Instagram để chốt</a>
    </div>
  `;
  modal.classList.remove('hidden');
  modal.classList.add('open');
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
        </div>
        <div style="display:flex;gap:8px">
          <button class="btn-edit" data-id="${p.id}">Edit</button>
          <button class="btn-delete" data-id="${p.id}">Delete</button>
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
      const res = await authedFetch('/api/products/'+id);
      if(!res.ok){ alert('Product not found'); return; }
      const p = await res.json();
      document.getElementById('product-id').value = p.id;
      document.querySelector('#product-form [name="title"]').value = p.title;
      document.querySelector('#product-form [name="description"]').value = p.description;
      document.querySelector('#product-form [name="price"]').value = p.price;
      const select = document.getElementById('product-category');
      if(select) select.value = p.category_id || '';
      document.getElementById('product-form').scrollIntoView({behavior:'smooth'});
    });
  });
}

async function loadAdminCategories(){
  const select = document.getElementById('product-category');
  const list = document.getElementById('category-list');
  if(!select && !list) return;
  const res = await authedFetch('/api/categories');
  if(!res.ok){
    if(list) list.innerHTML = '<div class="muted">Không tải được danh mục.</div>';
    return;
  }
  const data = await res.json();
  adminCategories = data.map(c=>({...c,id:Number(c.id)}));
  if(select){
    select.innerHTML = `<option value="">Không phân loại</option>` + adminCategories.map(c=>`<option value="${c.id}">${c.name}</option>`).join('');
  }
  renderAdminCategoryList();
}

function renderAdminCategoryList(){
  const list = document.getElementById('category-list');
  if(!list) return;
  if(!adminCategories.length){
    list.innerHTML = '<p class="muted">Chưa có danh mục.</p>';
    return;
  }
  list.innerHTML = '';
  adminCategories.forEach(cat=>{
    const row = document.createElement('div');
    row.className = 'cat-row';
    row.innerHTML = `
      <span>${cat.name}</span>
      <div class="actions">
        <button class="btn-edit" data-id="${cat.id}">Sửa</button>
        <button class="btn-delete" data-id="${cat.id}">Xoá</button>
      </div>`;
    row.querySelector('.btn-edit').addEventListener('click', async ()=>{
      const newName = await showPrompt('Tên mới', cat.name);
      if(!newName) return;
      const res = await authedFetch('/api/categories/'+cat.id,{
        method:'PUT',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({name:newName})
      });
      if(res.ok){
        await loadAdminCategories();
        await listProducts();
      }else{
        const txt = await res.text().catch(()=>'<no body>');
        alert('Cập nhật thất bại: '+txt);
      }
    });
    row.querySelector('.btn-delete').addEventListener('click', async ()=>{
      const ok = await showConfirm('Xoá danh mục này? Các sản phẩm thuộc danh mục sẽ chuyển sang "Không phân loại".');
      if(!ok) return;
      const res = await authedFetch('/api/admin/delete-category', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({id: Number(cat.id)})
      });
      if(res.ok){
        await loadAdminCategories();
        await listProducts();
      }else{
        const txt = await res.text().catch(()=>'<no body>');
        alert('Xoá thất bại: '+txt);
      }
    });
    list.appendChild(row);
  });
}

document.addEventListener('DOMContentLoaded', ()=>{
  loadProfile();
  listProducts();
  fetchPublicCategories();
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
    loadAdminCategories();
    if(!adminToken){
      const warn = document.getElementById('token-warning');
      if(warn) warn.classList.remove('hidden');
    }
  }

  if(productForm){
    productForm.addEventListener('submit', async (e)=>{
      e.preventDefault();
      const fd = new FormData(productForm);
      const editId = document.getElementById('product-id').value;
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
    });
  }

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

  if(categoryForm){
    categoryForm.addEventListener('submit', async (e)=>{
      e.preventDefault();
      const fd = new FormData(categoryForm);
      const name = fd.get('name');
      const res = await authedFetch('/api/categories',{
        method:'POST',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({name})
      });
      if(res.ok){
        categoryForm.reset();
        loadAdminCategories();
        listProducts();
      }else{
        const txt = await res.text();
        alert('Thêm danh mục thất bại: '+txt);
      }
    });
  }
});

