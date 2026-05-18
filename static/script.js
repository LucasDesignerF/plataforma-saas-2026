// Função para exibir QR Code em modal (exemplo)
function showPixQR(qrBase64, pixCode, amount) {
  const modal = document.createElement('div');
  modal.className = 'modal glass-card';
  modal.style.position = 'fixed';
  modal.style.top = '50%';
  modal.style.left = '50%';
  modal.style.transform = 'translate(-50%, -50%)';
  modal.style.zIndex = '1000';
  modal.style.minWidth = '300px';
  modal.style.textAlign = 'center';
  modal.innerHTML = `
    <h3>Pague via PIX</h3>
    <p>Valor: R$ ${amount}</p>
    <img src="data:image/png;base64,${qrBase64}" style="width:200px;margin:1rem auto;">
    <p><strong>Código copia e cola:</strong></p>
    <textarea rows="3" readonly style="width:100%;">${pixCode}</textarea>
    <button onclick="navigator.clipboard.writeText('${pixCode}')">Copiar código</button>
    <button onclick="this.parentElement.remove()">Fechar</button>
  `;
  document.body.appendChild(modal);
}

// Função para comprar produto (chamada via fetch)
async function buyProduct(productId) {
  const formData = new FormData();
  formData.append('product_id', productId);
  const resp = await fetch('/api/order', { method: 'POST', body: formData });
  const data = await resp.json();
  if (data.qr_base64) {
    showPixQR(data.qr_base64, data.pix_code, data.amount);
  } else {
    alert('Erro ao gerar pagamento');
  }
}

// Carregar produtos no catálogo
async function loadProducts() {
  const resp = await fetch('/admin/products');
  const products = await resp.json();
  const container = document.querySelector('.products-grid');
  if (!container) return;
  container.innerHTML = '';
  products.forEach(p => {
    const card = document.createElement('div');
    card.className = 'glass-card fade-in';
    card.innerHTML = `
      <h3>${p.name}</h3>
      <p>${p.description}</p>
      <p><strong>R$ ${p.price}</strong></p>
      <button onclick="buyProduct('${p.id}')">Comprar com PIX</button>
    `;
    container.appendChild(card);
  });
}

document.addEventListener('DOMContentLoaded', () => {
  if (window.location.pathname === '/products') loadProducts();
});