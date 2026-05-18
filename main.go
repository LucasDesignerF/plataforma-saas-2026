package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/joho/godotenv"
	"github.com/skip2/go-qrcode"
	"golang.org/x/oauth2"
)

var (
	oauthConfig  *oauth2.Config
	store        *sessions.CookieStore
	usersFile    = "data/users.json"
	productsFile = "data/products.json"
	ordersFile   = "data/orders.json"
	ticketsFile  = "data/tickets.json"
	dataMutex    sync.Mutex
	pixName      string
	pixCity      string
	pixKey       string
	adminIDs     map[string]bool
)

var discordEndpoint = oauth2.Endpoint{
	AuthURL:  "https://discord.com/api/oauth2/authorize",
	TokenURL: "https://discord.com/api/oauth2/token",
}

// URL do proxy (ngrok)
const discordProxyBase = "https://26b4-45-189-231-187.ngrok-free.app"

// Transporte personalizado que redireciona as chamadas para o Worker
type discordProxyRoundTripper struct{}

func (t *discordProxyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Redireciona apenas requisições para os endpoints da API do Discord (token e user info)
	if req.URL.Host == "discord.com" {
		if req.URL.Path == "/oauth2/token" || req.URL.Path == "/api/users/@me" {
			newURL := discordProxyBase + req.URL.Path
			newReqURL, err := url.Parse(newURL)
			if err != nil {
				return nil, err
			}
			req.URL = newReqURL
			req.Header.Set("User-Agent", "NexusPlatforms/1.0 (Go backend via Cloudflare)")
		}
	}
	return http.DefaultTransport.RoundTrip(req)
}

// Configura o cliente HTTP padrão para usar o proxy (todas as chamadas HTTP da aplicação)
func initHTTPClient() {
	http.DefaultClient.Timeout = 10 * time.Second
	http.DefaultClient.Transport = &discordProxyRoundTripper{}
}

// Structs (devem vir antes do init que as utiliza)
type User struct {
	ID                string    `json:"id"`
	DiscordID         string    `json:"discord_id"`
	Username          string    `json:"username"`
	GlobalName        string    `json:"global_name"`
	Email             string    `json:"email"`
	AvatarURL         string    `json:"avatar_url"`
	AccessToken       string    `json:"access_token"`
	RefreshToken      string    `json:"refresh_token"`
	TokenExpiry       time.Time `json:"token_expiry"`
	IsAdmin           bool      `json:"is_admin"`
	PurchasedProducts []string  `json:"purchased_products"`
}

type Product struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	Price           float64 `json:"price"`
	Category        string  `json:"category"`
	Stock           int     `json:"stock"`
	DeliveryType    string  `json:"delivery_type"`
	DeliveryContent string  `json:"delivery_content"`
}

type Order struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	ProductID   string     `json:"product_id"`
	ProductName string     `json:"product_name"`
	Amount      float64    `json:"amount"`
	Status      string     `json:"status"`
	PixCode     string     `json:"pix_code"`
	PixQRBase64 string     `json:"pix_qr_base64"`
	CreatedAt   time.Time  `json:"created_at"`
	ApprovedAt  *time.Time `json:"approved_at,omitempty"`
}

type Message struct {
	Author    string    `json:"author"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
}

type Ticket struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Subject   string    `json:"subject"`
	Messages  []Message `json:"messages"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func init() {
	// Configura o cliente HTTP global com o proxy
	initHTTPClient()

	if err := godotenv.Load(); err != nil {
		log.Println(".env não encontrado, usando variáveis de ambiente")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("APP_PORT")
		if port == "" {
			port = "8080"
		}
	}
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		secret = "default-secret-change-me"
	}
	store = sessions.NewCookieStore([]byte(secret))

	clientID := os.Getenv("DISCORD_CLIENT_ID")
	clientSecret := os.Getenv("DISCORD_CLIENT_SECRET")
	redirectURL := os.Getenv("DISCORD_REDIRECT_URI")
	if redirectURL == "" {
		redirectURL = "http://localhost:" + port + "/auth/callback"
	}
	oauthConfig = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"identify", "email"},
		Endpoint:     discordEndpoint,
	}
	// Não atribuímos oauthConfig.Client – o padrão http.DefaultClient já está configurado com o proxy

	pixName = os.Getenv("PIX_NAME")
	pixCity = os.Getenv("PIX_CITY")
	pixKey = os.Getenv("PIX_KEY")
	if pixName == "" || pixCity == "" || pixKey == "" {
		log.Fatal("PIX_NAME, PIX_CITY, PIX_KEY obrigatórios no .env")
	}

	adminIDs = make(map[string]bool)
	adminIDsStr := os.Getenv("ADMIN_IDS")
	if adminIDsStr != "" {
		for _, id := range strings.Split(adminIDsStr, ",") {
			adminIDs[strings.TrimSpace(id)] = true
		}
	}

	os.MkdirAll("data", 0755)
	os.MkdirAll("static/downloads", 0755)
	ensureJSONFile(usersFile, []User{})
	ensureJSONFile(productsFile, []Product{})
	ensureJSONFile(ordersFile, []Order{})
	ensureJSONFile(ticketsFile, []Ticket{})
}

func ensureJSONFile(path string, defaultContent interface{}) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		data, _ := json.MarshalIndent(defaultContent, "", "  ")
		os.WriteFile(path, data, 0644)
	}
}

func readJSON(file string, v interface{}) error {
	dataMutex.Lock()
	defer dataMutex.Unlock()
	f, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	return json.Unmarshal(f, v)
}

func writeJSON(file string, v interface{}) error {
	dataMutex.Lock()
	defer dataMutex.Unlock()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, data, 0644)
}

func getCurrentUser(r *http.Request) *User {
	session, _ := store.Get(r, "session")
	userID, ok := session.Values["user_id"].(string)
	if !ok || userID == "" {
		return nil
	}
	var users []User
	if err := readJSON(usersFile, &users); err != nil {
		return nil
	}
	for i, u := range users {
		if u.ID == userID {
			if time.Now().After(u.TokenExpiry) {
				refreshUserToken(&users[i])
				return &users[i]
			}
			return &u
		}
	}
	return nil
}

func refreshUserToken(user *User) {
	token := &oauth2.Token{
		AccessToken:  user.AccessToken,
		RefreshToken: user.RefreshToken,
		Expiry:       user.TokenExpiry,
	}
	src := oauthConfig.TokenSource(context.Background(), token)
	newToken, err := src.Token()
	if err != nil {
		log.Println("Erro ao atualizar token:", err)
		return
	}
	user.AccessToken = newToken.AccessToken
	user.RefreshToken = newToken.RefreshToken
	user.TokenExpiry = newToken.Expiry

	var users []User
	readJSON(usersFile, &users)
	for i, u := range users {
		if u.ID == user.ID {
			users[i] = *user
			break
		}
	}
	writeJSON(usersFile, users)
}

func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if getCurrentUser(r) == nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next(w, r)
	}
}

func requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getCurrentUser(r)
		if user == nil || !user.IsAdmin {
			http.Error(w, "Acesso negado", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func generatePixPayload(txid string, amount float64) (code string, qrBase64 string) {
	payload := fmt.Sprintf("00020126580014BR.GOV.BCB.PIX0136%s5204000053039865404%.2f5802BR5925%s6007%s62070503***6304", txid, amount, pixName, pixCity)
	crc := computeCRC16(payload)
	code = payload + fmt.Sprintf("%04X", crc)

	pngBytes, err := qrcode.Encode(code, qrcode.Medium, 256)
	if err != nil {
		log.Println("Erro QR Code:", err)
		return code, ""
	}
	qrBase64 = base64.StdEncoding.EncodeToString(pngBytes)
	return code, qrBase64
}

func computeCRC16(payload string) uint16 {
	crc := uint16(0xFFFF)
	for _, c := range []byte(payload) {
		crc ^= uint16(c) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc = crc << 1
			}
		}
	}
	return crc & 0xFFFF
}

func main() {
	r := mux.NewRouter()
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	r.HandleFunc("/", homeHandler)
	r.HandleFunc("/products", productsHandler)
	r.HandleFunc("/documentation", documentationHandler)
	r.HandleFunc("/login", loginHandler)
	r.HandleFunc("/auth/callback", authCallbackHandler)
	r.HandleFunc("/logout", logoutHandler)

	// Endpoint de ping para manter o serviço ativo (keep-alive)
	r.HandleFunc("/ping", pingHandler)

	// API pública
	r.HandleFunc("/api/products", apiProductsHandler)
	r.HandleFunc("/api/user/me", apiUserMeHandler)

	// Rotas privadas do usuário comum
	r.HandleFunc("/dashboard", requireAuth(dashboardHandler))
	r.HandleFunc("/api/user/orders", requireAuth(userOrdersHandler))
	r.HandleFunc("/api/user/products", requireAuth(userProductsHandler))
	r.HandleFunc("/api/user/tickets", requireAuth(userTicketsHandler))
	r.HandleFunc("/api/ticket", requireAuth(createTicketHandler))
	r.HandleFunc("/api/ticket/reply", requireAuth(userReplyTicketHandler))
	r.HandleFunc("/api/order", requireAuth(createOrderHandler))
	r.HandleFunc("/api/cart/checkout", requireAuth(cartCheckoutHandler)).Methods("POST")

	// Rotas administrativas
	r.HandleFunc("/admin", requireAdmin(adminPanelHandler))
	r.HandleFunc("/admin/products", requireAdmin(adminProductsHandler))
	r.HandleFunc("/admin/orders", requireAdmin(adminOrdersHandler))
	r.HandleFunc("/admin/tickets", requireAdmin(adminTicketsHandler))
	r.HandleFunc("/admin/users", requireAdmin(adminUsersHandler)).Methods("GET")
	r.HandleFunc("/admin/users/{id}/admin", requireAdmin(adminToggleAdminHandler)).Methods("PUT")
	r.HandleFunc("/admin/users/{id}", requireAdmin(adminDeleteUserHandler)).Methods("DELETE")
	r.HandleFunc("/admin/order/approve", requireAdmin(approveOrderHandler))
	r.HandleFunc("/admin/order/reject", requireAdmin(rejectOrderHandler))
	r.HandleFunc("/admin/ticket/reply", requireAdmin(adminReplyTicketHandler))
	r.HandleFunc("/admin/product/create", requireAdmin(createProductHandler))
	r.HandleFunc("/admin/product/update", requireAdmin(updateProductHandler))
	r.HandleFunc("/admin/product/delete", requireAdmin(deleteProductHandler))
	r.HandleFunc("/admin/product/upload", requireAdmin(uploadProductFileHandler))
	r.HandleFunc("/api/admin/stats", requireAdmin(adminStatsHandler))

	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("APP_PORT")
		if port == "" {
			port = "8080"
		}
	}
	log.Printf("Servidor rodando em http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

// Handler de ping (keep-alive)
func pingHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("pong"))
}

// ==================== HANDLERS PÚBLICOS ====================

func homeHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/index.html")
}
func productsHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/products.html")
}
func documentationHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/documentation.html")
}
func loginHandler(w http.ResponseWriter, r *http.Request) {
	url := oauthConfig.AuthCodeURL("state", oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusFound)
}
func logoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	session.Values["user_id"] = ""
	session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusFound)
}

func apiProductsHandler(w http.ResponseWriter, r *http.Request) {
	var products []Product
	readJSON(productsFile, &products)
	type PublicProduct struct {
		ID          string  `json:"id"`
		Name        string  `json:"name"`
		Description string  `json:"description"`
		Price       float64 `json:"price"`
		Category    string  `json:"category"`
		Stock       int     `json:"stock"`
	}
	publicList := []PublicProduct{}
	for _, p := range products {
		publicList = append(publicList, PublicProduct{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			Price:       p.Price,
			Category:    p.Category,
			Stock:       p.Stock,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(publicList)
}

func apiUserMeHandler(w http.ResponseWriter, r *http.Request) {
	user := getCurrentUser(r)
	if user == nil {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}
	response := map[string]interface{}{
		"id":          user.ID,
		"discord_id":  user.DiscordID,
		"username":    user.Username,
		"global_name": user.GlobalName,
		"avatar_url":  user.AvatarURL,
		"is_admin":    user.IsAdmin,
		"email":       user.Email,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func authCallbackHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Código não encontrado", http.StatusBadRequest)
		return
	}
	token, err := oauthConfig.Exchange(r.Context(), code)
	if err != nil {
		log.Printf("Erro ao trocar token: %v", err)
		http.Error(w, "Erro ao trocar token: "+err.Error(), http.StatusInternalServerError)
		return
	}
	client := oauthConfig.Client(r.Context(), token)
	resp, err := client.Get("https://discord.com/api/users/@me")
	if err != nil {
		http.Error(w, "Erro ao obter dados do Discord", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var discordUser struct {
		ID         string `json:"id"`
		Username   string `json:"username"`
		GlobalName string `json:"global_name"`
		Email      string `json:"email"`
		Avatar     string `json:"avatar"`
	}
	json.NewDecoder(resp.Body).Decode(&discordUser)

	avatarURL := fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", discordUser.ID, discordUser.Avatar)

	var users []User
	readJSON(usersFile, &users)
	var existing *User
	for i, u := range users {
		if u.DiscordID == discordUser.ID {
			existing = &users[i]
			break
		}
	}

	isAdmin := adminIDs[discordUser.ID]

	if existing == nil {
		newUser := User{
			ID:                generateID(),
			DiscordID:         discordUser.ID,
			Username:          discordUser.Username,
			GlobalName:        discordUser.GlobalName,
			Email:             discordUser.Email,
			AvatarURL:         avatarURL,
			AccessToken:       token.AccessToken,
			RefreshToken:      token.RefreshToken,
			TokenExpiry:       token.Expiry,
			IsAdmin:           isAdmin,
			PurchasedProducts: []string{},
		}
		users = append(users, newUser)
		writeJSON(usersFile, users)
		existing = &newUser
	} else {
		existing.AccessToken = token.AccessToken
		existing.RefreshToken = token.RefreshToken
		existing.TokenExpiry = token.Expiry
		existing.Username = discordUser.Username
		existing.GlobalName = discordUser.GlobalName
		existing.Email = discordUser.Email
		existing.AvatarURL = avatarURL
		existing.IsAdmin = isAdmin
		writeJSON(usersFile, users)
	}
	session, _ := store.Get(r, "session")
	session.Values["user_id"] = existing.ID
	session.Save(r, w)

	if existing.IsAdmin {
		http.Redirect(w, r, "/admin", http.StatusFound)
	} else {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	}
}

func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/dashboard.html")
}

// ========== ENDPOINTS PARA USUÁRIO COMUM ==========

func userOrdersHandler(w http.ResponseWriter, r *http.Request) {
	user := getCurrentUser(r)
	if user == nil {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}
	var orders []Order
	readJSON(ordersFile, &orders)
	userOrders := []Order{}
	for _, o := range orders {
		if o.UserID == user.ID {
			userOrders = append(userOrders, o)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userOrders)
}

func userProductsHandler(w http.ResponseWriter, r *http.Request) {
	user := getCurrentUser(r)
	if user == nil {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}
	var products []Product
	readJSON(productsFile, &products)
	var purchased []Product
	for _, pid := range user.PurchasedProducts {
		for _, p := range products {
			if p.ID == pid {
				purchased = append(purchased, p)
				break
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(purchased)
}

func userTicketsHandler(w http.ResponseWriter, r *http.Request) {
	user := getCurrentUser(r)
	if user == nil {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}
	var tickets []Ticket
	readJSON(ticketsFile, &tickets)
	userTickets := []Ticket{}
	for _, t := range tickets {
		if t.UserID == user.ID {
			userTickets = append(userTickets, t)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userTickets)
}

func createTicketHandler(w http.ResponseWriter, r *http.Request) {
	user := getCurrentUser(r)
	if user == nil {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}
	subject := r.FormValue("subject")
	message := r.FormValue("message")
	if subject == "" || message == "" {
		http.Error(w, "Assunto e mensagem obrigatórios", http.StatusBadRequest)
		return
	}
	ticket := Ticket{
		ID:      generateID(),
		UserID:  user.ID,
		Subject: subject,
		Messages: []Message{
			{Author: "user", Text: message, Timestamp: time.Now()},
		},
		Status:    "open",
		CreatedAt: time.Now(),
	}
	var tickets []Ticket
	readJSON(ticketsFile, &tickets)
	tickets = append(tickets, ticket)
	writeJSON(ticketsFile, tickets)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func userReplyTicketHandler(w http.ResponseWriter, r *http.Request) {
	user := getCurrentUser(r)
	if user == nil {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}
	ticketID := r.FormValue("ticket_id")
	message := r.FormValue("message")
	if ticketID == "" || message == "" {
		http.Error(w, "ticket_id e message são obrigatórios", http.StatusBadRequest)
		return
	}
	var tickets []Ticket
	readJSON(ticketsFile, &tickets)
	var found bool
	for i, t := range tickets {
		if t.ID == ticketID && t.UserID == user.ID && t.Status == "open" {
			tickets[i].Messages = append(tickets[i].Messages, Message{
				Author:    "user",
				Text:      message,
				Timestamp: time.Now(),
			})
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "Ticket não encontrado, não pertence a você ou está fechado", http.StatusNotFound)
		return
	}
	writeJSON(ticketsFile, tickets)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func createOrderHandler(w http.ResponseWriter, r *http.Request) {
	user := getCurrentUser(r)
	if user == nil {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}
	productID := r.FormValue("product_id")
	var products []Product
	readJSON(productsFile, &products)
	var product *Product
	for _, p := range products {
		if p.ID == productID {
			product = &p
			break
		}
	}
	if product == nil {
		http.Error(w, "Produto não encontrado", http.StatusNotFound)
		return
	}
	if product.Stock != -1 && product.Stock <= 0 {
		http.Error(w, "Produto sem estoque", http.StatusBadRequest)
		return
	}
	txid := generateID()[:12]
	pixCode, pixQRBase64 := generatePixPayload(txid, product.Price)
	order := Order{
		ID:          generateID(),
		UserID:      user.ID,
		ProductID:   product.ID,
		ProductName: product.Name,
		Amount:      product.Price,
		Status:      "pending",
		PixCode:     pixCode,
		PixQRBase64: pixQRBase64,
		CreatedAt:   time.Now(),
	}
	var orders []Order
	readJSON(ordersFile, &orders)
	orders = append(orders, order)
	writeJSON(ordersFile, orders)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"order_id":  order.ID,
		"pix_code":  pixCode,
		"qr_base64": pixQRBase64,
		"amount":    product.Price,
		"product":   product.Name,
	})
}

func cartCheckoutHandler(w http.ResponseWriter, r *http.Request) {
	user := getCurrentUser(r)
	if user == nil {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}
	var req struct {
		Items []struct {
			ProductID string `json:"product_id"`
			Quantity  int    `json:"quantity"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}
	if len(req.Items) == 0 {
		http.Error(w, "Carrinho vazio", http.StatusBadRequest)
		return
	}
	var products []Product
	readJSON(productsFile, &products)
	productMap := make(map[string]Product)
	for _, p := range products {
		productMap[p.ID] = p
	}
	var total float64
	var ordersToCreate []Order
	txidBase := generateID()[:12]
	for _, item := range req.Items {
		prod, ok := productMap[item.ProductID]
		if !ok {
			http.Error(w, "Produto não encontrado: "+item.ProductID, http.StatusNotFound)
			return
		}
		if prod.Stock != -1 && prod.Stock < item.Quantity {
			http.Error(w, "Estoque insuficiente para "+prod.Name, http.StatusBadRequest)
			return
		}
		subtotal := prod.Price * float64(item.Quantity)
		total += subtotal
		for q := 0; q < item.Quantity; q++ {
			txid := txidBase + generateID()[:4]
			pixCode, pixQRBase64 := generatePixPayload(txid, prod.Price)
			order := Order{
				ID:          generateID(),
				UserID:      user.ID,
				ProductID:   prod.ID,
				ProductName: prod.Name,
				Amount:      prod.Price,
				Status:      "pending",
				PixCode:     pixCode,
				PixQRBase64: pixQRBase64,
				CreatedAt:   time.Now(),
			}
			ordersToCreate = append(ordersToCreate, order)
		}
	}
	var allOrders []Order
	readJSON(ordersFile, &allOrders)
	allOrders = append(allOrders, ordersToCreate...)
	writeJSON(ordersFile, allOrders)

	txidTotal := generateID()[:12]
	pixCodeTotal, pixQRBase64Total := generatePixPayload(txidTotal, total)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total":     total,
		"pix_code":  pixCodeTotal,
		"qr_base64": pixQRBase64Total,
		"orders":    ordersToCreate,
	})
}

// ========== ADMIN HANDLERS ==========

func adminPanelHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/admin.html")
}
func adminProductsHandler(w http.ResponseWriter, r *http.Request) {
	var products []Product
	readJSON(productsFile, &products)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}
func uploadProductFileHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(50 << 20)
	if err != nil {
		http.Error(w, "Arquivo muito grande", http.StatusBadRequest)
		return
	}
	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Arquivo não enviado", http.StatusBadRequest)
		return
	}
	defer file.Close()
	ext := filepath.Ext(handler.Filename)
	filename := generateID() + ext
	filePath := filepath.Join("static", "downloads", filename)
	dst, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Erro ao salvar arquivo", http.StatusInternalServerError)
		return
	}
	defer dst.Close()
	_, err = io.Copy(dst, file)
	if err != nil {
		http.Error(w, "Erro ao copiar arquivo", http.StatusInternalServerError)
		return
	}
	urlPath := "/static/downloads/" + filename
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"file_url": urlPath})
}
func createProductHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "Erro ao processar formulário", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	description := r.FormValue("description")
	priceStr := r.FormValue("price")
	category := r.FormValue("category")
	stockStr := r.FormValue("stock")
	deliveryType := r.FormValue("delivery_type")
	deliveryContent := r.FormValue("delivery_content")
	price, _ := strconv.ParseFloat(priceStr, 64)
	stock, _ := strconv.Atoi(stockStr)
	product := Product{
		ID:              generateID(),
		Name:            name,
		Description:     description,
		Price:           price,
		Category:        category,
		Stock:           stock,
		DeliveryType:    deliveryType,
		DeliveryContent: deliveryContent,
	}
	var products []Product
	readJSON(productsFile, &products)
	products = append(products, product)
	writeJSON(productsFile, products)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
func updateProductHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "Erro ao processar formulário", http.StatusBadRequest)
		return
	}
	id := r.FormValue("id")
	name := r.FormValue("name")
	description := r.FormValue("description")
	priceStr := r.FormValue("price")
	category := r.FormValue("category")
	stockStr := r.FormValue("stock")
	deliveryType := r.FormValue("delivery_type")
	deliveryContent := r.FormValue("delivery_content")
	price, _ := strconv.ParseFloat(priceStr, 64)
	stock, _ := strconv.Atoi(stockStr)
	var products []Product
	readJSON(productsFile, &products)
	for i, p := range products {
		if p.ID == id {
			products[i].Name = name
			products[i].Description = description
			products[i].Price = price
			products[i].Category = category
			products[i].Stock = stock
			products[i].DeliveryType = deliveryType
			products[i].DeliveryContent = deliveryContent
			break
		}
	}
	writeJSON(productsFile, products)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
func deleteProductHandler(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	var products []Product
	readJSON(productsFile, &products)
	newProducts := []Product{}
	for _, p := range products {
		if p.ID != id {
			newProducts = append(newProducts, p)
		}
	}
	writeJSON(productsFile, newProducts)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
func adminOrdersHandler(w http.ResponseWriter, r *http.Request) {
	var orders []Order
	readJSON(ordersFile, &orders)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}
func approveOrderHandler(w http.ResponseWriter, r *http.Request) {
	orderID := r.FormValue("order_id")
	var orders []Order
	readJSON(ordersFile, &orders)
	var order *Order
	for i, o := range orders {
		if o.ID == orderID {
			order = &orders[i]
			break
		}
	}
	if order == nil {
		http.Error(w, "Pedido não encontrado", http.StatusNotFound)
		return
	}
	if order.Status != "pending" {
		http.Error(w, "Pedido já processado", http.StatusBadRequest)
		return
	}
	now := time.Now()
	order.Status = "approved"
	order.ApprovedAt = &now

	var products []Product
	readJSON(productsFile, &products)
	var product *Product
	for idx := range products {
		if products[idx].ID == order.ProductID {
			product = &products[idx]
			break
		}
	}
	if product != nil {
		if product.Stock > 0 {
			product.Stock--
			writeJSON(productsFile, products)
		}
		var users []User
		readJSON(usersFile, &users)
		for i, u := range users {
			if u.ID == order.UserID {
				users[i].PurchasedProducts = append(users[i].PurchasedProducts, product.ID)
				break
			}
		}
		writeJSON(usersFile, users)
	}
	writeJSON(ordersFile, orders)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
func rejectOrderHandler(w http.ResponseWriter, r *http.Request) {
	orderID := r.FormValue("order_id")
	var orders []Order
	readJSON(ordersFile, &orders)
	for i, o := range orders {
		if o.ID == orderID && o.Status == "pending" {
			orders[i].Status = "rejected"
			break
		}
	}
	writeJSON(ordersFile, orders)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
func adminTicketsHandler(w http.ResponseWriter, r *http.Request) {
	var tickets []Ticket
	readJSON(ticketsFile, &tickets)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tickets)
}
func adminReplyTicketHandler(w http.ResponseWriter, r *http.Request) {
	ticketID := r.FormValue("ticket_id")
	reply := r.FormValue("reply")
	if ticketID == "" || reply == "" {
		http.Error(w, "ticket_id e reply são obrigatórios", http.StatusBadRequest)
		return
	}
	var tickets []Ticket
	readJSON(ticketsFile, &tickets)
	var found bool
	for i, t := range tickets {
		if t.ID == ticketID {
			tickets[i].Messages = append(tickets[i].Messages, Message{
				Author:    "admin",
				Text:      reply,
				Timestamp: time.Now(),
			})
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "Ticket não encontrado", http.StatusNotFound)
		return
	}
	writeJSON(ticketsFile, tickets)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Listar usuários (resumido)
func adminUsersHandler(w http.ResponseWriter, r *http.Request) {
	var users []User
	readJSON(usersFile, &users)
	type PublicUser struct {
		ID         string `json:"id"`
		Username   string `json:"username"`
		GlobalName string `json:"global_name"`
		Email      string `json:"email"`
		IsAdmin    bool   `json:"is_admin"`
	}
	result := []PublicUser{}
	for _, u := range users {
		result = append(result, PublicUser{
			ID:         u.ID,
			Username:   u.Username,
			GlobalName: u.GlobalName,
			Email:      u.Email,
			IsAdmin:    u.IsAdmin,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// Alterar status de admin de um usuário
func adminToggleAdminHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]
	var users []User
	readJSON(usersFile, &users)
	var targetUser *User
	var targetIndex int
	for i, u := range users {
		if u.ID == userID {
			targetUser = &users[i]
			targetIndex = i
			break
		}
	}
	if targetUser == nil {
		http.Error(w, "Usuário não encontrado", http.StatusNotFound)
		return
	}
	// Não permitir alterar o próprio admin (segurança)
	currentUser := getCurrentUser(r)
	if currentUser != nil && currentUser.ID == userID {
		http.Error(w, "Não é possível alterar seu próprio status de admin", http.StatusBadRequest)
		return
	}
	// Inverter o status
	targetUser.IsAdmin = !targetUser.IsAdmin
	users[targetIndex] = *targetUser
	writeJSON(usersFile, users)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"is_admin": targetUser.IsAdmin,
	})
}

// Excluir usuário
func adminDeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]
	var users []User
	readJSON(usersFile, &users)
	var found bool
	newUsers := []User{}
	for _, u := range users {
		if u.ID == userID {
			found = true
			continue
		}
		newUsers = append(newUsers, u)
	}
	if !found {
		http.Error(w, "Usuário não encontrado", http.StatusNotFound)
		return
	}
	writeJSON(usersFile, newUsers)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func adminStatsHandler(w http.ResponseWriter, r *http.Request) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	ramMB := float64(mem.Alloc) / 1024 / 1024
	cpuPercent := float64(runtime.NumGoroutine()) / 100.0
	if cpuPercent > 100 {
		cpuPercent = 100
	}
	var totalDisk int64 = 10 * 1024 * 1024 * 1024
	var usedDisk int64 = 0
	_ = filepath.Walk("data", func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			usedDisk += info.Size()
		}
		return nil
	})
	diskPercent := float64(usedDisk) / float64(totalDisk) * 100
	if totalDisk == 0 {
		diskPercent = 0
	}
	var users []User
	var products []Product
	var orders []Order
	var tickets []Ticket
	readJSON(usersFile, &users)
	readJSON(productsFile, &products)
	readJSON(ordersFile, &orders)
	readJSON(ticketsFile, &tickets)
	pendingOrders := 0
	for _, o := range orders {
		if o.Status == "pending" {
			pendingOrders++
		}
	}
	openTickets := 0
	for _, t := range tickets {
		if t.Status == "open" {
			openTickets++
		}
	}
	response := map[string]interface{}{
		"total_users":    len(users),
		"total_products": len(products),
		"pending_orders": pendingOrders,
		"open_tickets":   openTickets,
		"ram_mb":         roundFloat(ramMB, 1),
		"cpu_percent":    roundFloat(cpuPercent, 1),
		"disk_percent":   roundFloat(diskPercent, 1),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
func roundFloat(val float64, precision int) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)[:12]
}
