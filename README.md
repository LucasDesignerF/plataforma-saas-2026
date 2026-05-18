# Nexus Platforms - Plataforma SaaS de Produtos Digitais

**Nexus Platforms** é uma plataforma completa para venda de produtos digitais com autenticação via Discord OAuth, pagamentos via PIX (QR Code e copia e cola), entrega automática, sistema de tickets de suporte e um poderoso painel administrativo. Desenvolvida em **Go (Golang)** com armazenamento local em JSON, oferece uma interface moderna, responsiva e com fundo animado (grid + orbs) e efeitos glassmorphism.

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/LucasDesignerF/plataforma-saas-2026?style=flat-square)
![GitHub repo size](https://img.shields.io/github/repo-size/LucasDesignerF/plataforma-saas-2026?style=flat-square)
![GitHub last commit](https://img.shields.io/github/last-commit/LucasDesignerF/plataforma-saas-2026?style=flat-square)
![GitHub license](https://img.shields.io/github/license/LucasDesignerF/plataforma-saas-2026?style=flat-square)
![GitHub language count](https://img.shields.io/github/languages/count/LucasDesignerF/plataforma-saas-2026?style=flat-square)
![GitHub top language](https://img.shields.io/github/languages/top/LucasDesignerF/plataforma-saas-2026?style=flat-square)
![Discord](https://img.shields.io/discord/1351699619310141532?label=Discord&logo=discord&color=5865F2&style=flat-square)
![GitHub stars](https://img.shields.io/github/stars/LucasDesignerF/plataforma-saas-2026?style=flat-square)
![GitHub forks](https://img.shields.io/github/forks/LucasDesignerF/plataforma-saas-2026?style=flat-square)
![GitHub issues](https://img.shields.io/github/issues/LucasDesignerF/plataforma-saas-2026?style=flat-square)
![PRs welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square)

---

## ✨ Funcionalidades

- 🔐 **Autenticação com Discord OAuth2** – login seguro e persistente (refresh automático do token).
- 🛒 **Carrinho de compras** – adicione produtos ao carrinho e finalize a compra com PIX.
- 💳 **Pagamento via PIX** – geração de QR Code e código copia e cola (Payloader personalizado).
- 📦 **Entrega automática** – após aprovação do pedido pelo admin, o produto (texto, chave ou arquivo) é liberado no dashboard do cliente.
- 🎫 **Sistema de tickets** – comunicação bidirecional entre cliente e suporte (histórico de mensagens).
- 👑 **Painel Admin completo** – gestão de produtos, pedidos, tickets e usuários (tornar admin/excluir).
- 📊 **Métricas do sistema** – consumo de RAM, CPU e uso de disco.
- 🎨 **Design moderno** – tema escuro, grid animado, orbs brilhantes, glassmorphism, totalmente responsivo.
- 🍪 **Banner de cookies** – conformidade com LGPD.

---

## 🛠️ Tecnologias utilizadas

| Camada          | Tecnologia                                                                 |
|-----------------|----------------------------------------------------------------------------|
| Backend         | Go 1.21+ (Gorilla Mux, sessions, OAuth2, QRCode, godotenv)                |
| Frontend        | HTML5, CSS3, JavaScript (fetch API)                                       |
| Ícones          | Boxicons                                                                  |
| Fontes          | Google Fonts (Inter)                                                      |
| Armazenamento   | JSON local (arquivos `data/*.json`)                                       |
| Autenticação    | Discord OAuth2                                                            |
| Pagamento       | PIX via Payloader personalizado (sem gateway externo)                     |

---

## 🚀 Como executar o projeto

### Pré-requisitos

- [Go](https://go.dev/dl/) 1.21 ou superior
- Uma aplicação registrada no [Discord Developer Portal](https://discord.com/developers/applications) (para obter `Client ID` e `Client Secret`)
- Chave PIX (e‑mail, CPF, CNPJ ou telefone)

### Passo a passo

1. **Clone o repositório**

   ```bash
   git clone https://github.com/LucasDesignerF/plataforma-saas-2026.git
   cd plataforma-saas-2026
   ```

2. **Configure as variáveis de ambiente**

   Crie um arquivo `.env` na raiz do projeto com base no exemplo abaixo.  
   **Edite os valores** com os dados reais do seu Discord e da sua chave PIX.

   ```env
   # Discord OAuth2
   DISCORD_CLIENT_ID=seu_client_id
   DISCORD_CLIENT_SECRET=seu_client_secret
   DISCORD_REDIRECT_URI=http://localhost:8080/auth/callback

   # PIX (Payloader)
   PIX_NAME=Nexus
   PIX_CITY=SuaCidade
   PIX_KEY=sua_chave_pix

   # App
   APP_PORT=8080
   SESSION_SECRET=uma_string_aleatoria_e_segura

   # Admin Discord IDs (separados por vírgula)
   ADMIN_IDS=seu_discord_id,outro_id_opcional
   ```

3. **Baixe as dependências Go**

   ```bash
   go mod tidy
   ```

4. **Execute o servidor**

   ```bash
   go run main.go
   ```

5. **Acesse a aplicação**

   Abra o navegador em [http://localhost:8080](http://localhost:8080).

> **Nota:** O primeiro usuário a fazer login com Discord será automaticamente promovido a **admin** (se o seu `DISCORD_ID` estiver listado em `ADMIN_IDS`). Caso contrário, você pode adicionar seu ID manualmente no `.env` e reiniciar o servidor.

---

## 📁 Estrutura do projeto

```
plataforma-saas-2026/
├── main.go                 # Backend completo (rotas, handlers, storage)
├── go.mod / go.sum         # Gerenciamento de dependências
├── .env                    # Configurações sensíveis (não commitado)
├── data/                   # Banco de dados JSON (criado automaticamente)
│   ├── users.json
│   ├── products.json
│   ├── orders.json
│   └── tickets.json
├── templates/              # Páginas HTML
│   ├── index.html
│   ├── products.html
│   ├── documentation.html
│   ├── dashboard.html
│   ├── admin.html
│   └── ticket.html
└── static/                 # Arquivos estáticos (CSS, JS, downloads)
    ├── style.css
    ├── script.js
    └── downloads/          # Arquivos enviados pelo admin
```

---

## 🔌 Endpoints principais

| Método | Rota                         | Descrição                                      | Acesso |
|--------|------------------------------|------------------------------------------------|--------|
| GET    | `/`                          | Página inicial                                 | Público |
| GET    | `/products`                  | Catálogo de produtos                           | Público |
| GET    | `/api/products`              | Lista de produtos (público)                    | Público |
| GET    | `/login`                     | Redireciona para Discord OAuth                 | Público |
| GET    | `/auth/callback`             | Callback do Discord (cria/atualiza usuário)    | Público |
| GET    | `/dashboard`                 | Dashboard do cliente (requer login)            | Usuário |
| GET    | `/api/user/me`               | Dados do usuário logado                        | Usuário |
| GET    | `/api/user/orders`           | Pedidos do usuário logado                      | Usuário |
| GET    | `/api/user/products`         | Produtos comprados (entregues)                 | Usuário |
| GET    | `/api/user/tickets`          | Tickets do usuário                             | Usuário |
| POST   | `/api/ticket`                | Criar novo ticket                              | Usuário |
| POST   | `/api/ticket/reply`          | Responder a um ticket (usuário)                | Usuário |
| POST   | `/api/order`                 | Criar pedido (compra direta)                   | Usuário |
| POST   | `/api/cart/checkout`         | Finalizar carrinho (múltiplos produtos)        | Usuário |
| GET    | `/admin`                     | Painel administrativo                          | Admin |
| GET    | `/admin/products`            | Listar todos os produtos                       | Admin |
| POST   | `/admin/product/create`      | Criar produto                                  | Admin |
| POST   | `/admin/product/update`      | Atualizar produto                              | Admin |
| POST   | `/admin/product/delete`      | Excluir produto                                | Admin |
| POST   | `/admin/product/upload`      | Fazer upload de arquivo (para produto tipo file) | Admin |
| GET    | `/admin/orders`              | Listar todos os pedidos                        | Admin |
| POST   | `/admin/order/approve`       | Aprovar pedido (entrega automática)            | Admin |
| POST   | `/admin/order/reject`        | Rejeitar pedido                                | Admin |
| GET    | `/admin/tickets`             | Listar todos os tickets                        | Admin |
| POST   | `/admin/ticket/reply`        | Responder ticket (admin)                       | Admin |
| GET    | `/admin/users`               | Listar todos os usuários                       | Admin |
| PUT    | `/admin/users/{id}/admin`    | Alternar status de admin                       | Admin |
| DELETE | `/admin/users/{id}`          | Excluir usuário                                | Admin |
| GET    | `/api/admin/stats`           | Métricas do sistema (RAM, CPU, disco)          | Admin |

---

## 🤝 Contribuição

Contribuições são bem‑vindas! Sinta‑se à vontade para abrir **issues** ou enviar **pull requests**.  
Para grandes alterações, por favor, discuta primeiro o que você gostaria de mudar.

---

## 📞 Suporte e comunidade

- **Discord:** [https://discord.gg/sFhQDjW534](https://discord.gg/sFhQDjW534)
- **GitHub Issues:** [https://github.com/LucasDesignerF/plataforma-saas-2026/issues](https://github.com/LucasDesignerF/plataforma-saas-2026/issues)

---

## 📄 Licença

Este projeto está sob a licença **MIT**. Consulte o arquivo [LICENSE](LICENSE) para mais informações.

---

## 🙏 Agradecimentos

- [Discord OAuth2](https://discord.com/developers/docs/topics/oauth2)
- [Biblioteca go-qrcode](https://github.com/skip2/go-qrcode)
- [Gorilla Web Toolkit](https://www.gorillatoolkit.org/)

---

Desenvolvido com 💜 por [LucasDesignerF](https://github.com/LucasDesignerF) – Nexus Platforms
