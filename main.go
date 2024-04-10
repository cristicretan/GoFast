package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt"

	_ "github.com/mattn/go-sqlite3"
)

var jwtKey = []byte("deadbeef")

const secretFlag = "ACS_KEYSIGHT_CTF{run_forr3st_run}"

type Claims struct {
	Username string `json:"username"`
	jwt.StandardClaims
}

type Product struct {
	ID    int
	Name  string
	Price int
}

type PurchaseLog struct {
	ID          int
	UserID      int
	ProductID   int
	ProductName string
	PaidAmount  int
	VDate       string // Using string for simplicity; you might want to use time.Time with proper formatting
}

type User struct {
	ID          int
	Username    string
	Password    string
	Token       string
	Balance     int
	PurchaseLog []PurchaseLog
}

var db *sql.DB

func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "./test.db")
	if err != nil {
		log.Fatal(err)
	}

	// SQL statement for creating the users table
	createUserTableSQL := `CREATE TABLE IF NOT EXISTS users (
        "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
        "username" TEXT NOT NULL UNIQUE,
        "password" TEXT NOT NULL,
        "token" TEXT,
        "balance" INTEGER NOT NULL
    );`

	// SQL statement for creating the purchase_logs table
	createPurchaseLogTableSQL := `CREATE TABLE IF NOT EXISTS purchase_logs (
        "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
        "user_id" INTEGER NOT NULL,
        "product_id" INTEGER NOT NULL,
        "paid_amount" INTEGER NOT NULL,
        "v_date" TEXT NOT NULL
    );`

	createProductTableSQL := `CREATE TABLE IF NOT EXISTS products (
		id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		price INTEGER NOT NULL
	);`

	// Execute the statement for creating the users table
	_, err = db.Exec(createUserTableSQL)
	if err != nil {
		log.Fatal("Failed to create users table:", err)
	}

	// Execute the statement for creating the purchase_logs table
	_, err = db.Exec(createPurchaseLogTableSQL)
	if err != nil {
		log.Fatal("Failed to create purchase_logs table:", err)
	}

	_, err = db.Exec(createProductTableSQL)
	if err != nil {
		log.Fatal("Failed to create products table:", err)
	}

	// Initialize products
	products := []Product{
		{Name: "🍔 Quantum Burger", Price: 5},
		{Name: "🥤 Schroedinger's Soda", Price: 20},
		{Name: "🎩 Houdini's Hat", Price: 21},
	}

	for _, p := range products {
		_, err = db.Exec("INSERT OR IGNORE INTO products (name, price) VALUES (?, ?)", p.Name, p.Price)
		if err != nil {
			log.Printf("Failed to insert product %s: %v", p.Name, err)
		}
	}
}

func serveTemplate(w http.ResponseWriter, r *http.Request, templateName string, data interface{}) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate") // HTTP 1.1.
	w.Header().Set("Pragma", "no-cache")                                   // HTTP 1.0.
	w.Header().Set("Expires", "0")                                         // Proxies.
	lp := filepath.Join("templates/", templateName)
	tmpl, err := template.ParseFiles(lp)
	if err != nil {
		log.Printf("Error parsing template: %v", err) // Log parsing errors
		http.Error(w, "Internal Server Error", 500)
		return
	}
	err = tmpl.Execute(w, data)
	if err != nil {
		log.Printf("Error executing template: %v", err) // Log execution errors
		http.Error(w, "Internal Server Error", 500)
	}
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	// For simplicity, assuming a token cookie to check if logged in
	// This is not secure and should be improved for real applications
	cookie, err := r.Cookie("token")
	if err != nil || cookie.Value == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	// Here, you should validate the token and retrieve user data
	// Redirecting to home for this example
	http.Redirect(w, r, "/home", http.StatusSeeOther)
}

func generateToken(username string) (string, error) {
	expirationTime := time.Now().Add(30 * time.Minute)
	claims := &Claims{
		Username: username,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)

	return tokenString, err
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		serveTemplate(w, r, "login.html", nil)
	case http.MethodPost:
		username := r.FormValue("username")
		password := r.FormValue("password")

		var user User
		err := db.QueryRow("SELECT id, username, password, token, balance FROM users WHERE username = ?", username).Scan(&user.ID, &user.Username, &user.Password, &user.Token, &user.Balance)
		if err != nil {
			if err == sql.ErrNoRows {
				// User not found, create new user with a new token
				token, err := generateToken(username) // Generate a secure token instead
				if err != nil {
					http.Error(w, "Failed to generate token", http.StatusInternalServerError)
					return
				}

				_, err = db.Exec("INSERT INTO users (username, password, token, balance) VALUES (?, ?, ?, ?)", username, password, token, 20)
				if err != nil {
					http.Error(w, "Failed to create user", http.StatusInternalServerError)
					return
				}
				user = User{Username: username, Password: password, Token: token, Balance: 20}
			} else {
				http.Error(w, "Database error", http.StatusInternalServerError)
				return
			}
		}

		if user.Password != password {
			http.Error(w, "Invalid login credentials", http.StatusBadRequest)
			return
		}

		// Set the user's token as a cookie
		http.SetCookie(w, &http.Cookie{
			Name:  "token",
			Value: user.Token,
			Path:  "/",
		})
		// Redirect to home
		http.Redirect(w, r, "/home", http.StatusSeeOther)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func validateToken(tokenString string) (*Claims, bool) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if err != nil {
		return nil, false
	}

	if !token.Valid {
		return nil, false
	}

	return claims, true
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("token")
	if err != nil || cookie.Value == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	token := cookie.Value

	// Validate the token and get user details
	claims, isValid := validateToken(token)
	if !isValid {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	var user User
	err = db.QueryRow("SELECT id, username, balance FROM users WHERE username = ?", claims.Username).Scan(&user.ID, &user.Username, &user.Balance)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Fetch products for the user to potentially buy
	productRows, err := db.Query("SELECT id, name, price FROM products")
	if err != nil {
		log.Printf("Error fetching products: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer productRows.Close()

	var products []Product
	for productRows.Next() {
		var p Product
		if err := productRows.Scan(&p.ID, &p.Name, &p.Price); err != nil {
			log.Printf("Error scanning product: %v", err)
			continue
		}
		products = append(products, p)
	}

	// Fetch purchase logs for the user to show purchase history
	rows, err := db.Query(`
    SELECT pl.id, pl.user_id, pl.product_id, pl.paid_amount, pl.v_date, p.name AS product_name
    FROM purchase_logs AS pl
    INNER JOIN products AS p ON pl.product_id = p.id
    WHERE pl.user_id = ?`, user.ID)

	if err != nil {
		log.Printf("Error fetching purchase logs: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var purchaseLogs []PurchaseLog
	for rows.Next() {
		var pl PurchaseLog
		if err := rows.Scan(&pl.ID, &pl.UserID, &pl.ProductID, &pl.PaidAmount, &pl.VDate, &pl.ProductName); err != nil {
			log.Printf("Error scanning purchase log: %v", err)
			continue
		}
		purchaseLogs = append(purchaseLogs, pl)
	}

	showFlag := false

	// Retrieve the cookie
	if cookie, err := r.Cookie("specialUserCheck"); err == nil && cookie.Value == "true" {
		showFlag = true
		// Optionally, delete the cookie after showing the flag
		http.SetCookie(w, &http.Cookie{
			Name:   "specialUserCheck",
			Value:  "",
			Path:   "/",
			MaxAge: -1, // Deletes the cookie
		})
	}

	// Prepare the data for the template, including products and purchase logs
	data := struct {
		Username    string
		Balance     int
		PurchaseLog []PurchaseLog
		Products    []Product
		ShowFlag    bool
	}{
		Username:    user.Username,
		Balance:     user.Balance,
		PurchaseLog: purchaseLogs,
		Products:    products,
		ShowFlag:    showFlag,
	}

	serveTemplate(w, r, "home.html", data)
}

func buyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("token")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	claims, isValid := validateToken(cookie.Value)
	if !isValid {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	productID, err := strconv.Atoi(r.FormValue("product_id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	// Get product price
	var price int
	err = db.QueryRow("SELECT price FROM products WHERE id = ?", productID).Scan(&price)
	if err != nil {
		http.Error(w, "Product not found", http.StatusBadRequest)
		return
	}

	// Get user's balance
	var balance int
	err = db.QueryRow("SELECT balance FROM users WHERE username = ?", claims.Username).Scan(&balance)
	if err != nil || balance < price {
		http.Error(w, "Insufficient balance", http.StatusBadRequest)
		return
	}

	// Simulate delay (VERY dangerous in real applications)
	time.Sleep(100 * time.Millisecond)

	if balance >= price {
		// Update user's balance
		_, err = db.Exec("UPDATE users SET balance = balance - ? WHERE username = ?", price, claims.Username)
		if err != nil {
			http.Error(w, "Failed to update user balance", http.StatusInternalServerError)
			return
		}

		// Insert a purchase log
		_, err = db.Exec("INSERT INTO purchase_logs (user_id, product_id, paid_amount, v_date) VALUES ((SELECT id FROM users WHERE username = ?), ?, ?, ?)", claims.Username, productID, price, time.Now().Format("2006-01-02"))
		if err != nil {
			http.Error(w, "Failed to log purchase", http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/home", http.StatusSeeOther)
}

func sellHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("token")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	claims, isValid := validateToken(cookie.Value)
	if !isValid {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	purchaseID, err := strconv.Atoi(r.FormValue("purchase_id"))
	if err != nil {
		http.Error(w, "Invalid purchase ID", http.StatusBadRequest)
		return
	}

	var productName string
	var productId int
	err = db.QueryRow(`
    SELECT p.name, p.id 
    FROM purchase_logs pl
    INNER JOIN products p ON pl.product_id = p.id
    WHERE pl.id = ? AND pl.user_id = (SELECT id FROM users WHERE username = ?)`,
		purchaseID, claims.Username).Scan(&productName, &productId)
	if err != nil {
		http.Error(w, "Product name not found or not owned by user", http.StatusBadRequest)
		return
	}

	if productId == 3 {
		// Set a special cookie or take other actions
		http.SetCookie(w, &http.Cookie{
			Name:     "specialUserCheck", // Cryptic name
			Value:    "true",
			Path:     "/",
			HttpOnly: true,
			MaxAge:   3600, // Expires after one hour
		})
	}

	// Directly update user's balance and delete the purchase log without using a transaction
	var paidAmount int
	err = db.QueryRow("SELECT paid_amount FROM purchase_logs WHERE id = ? AND user_id = (SELECT id FROM users WHERE username = ?)", purchaseID, claims.Username).Scan(&paidAmount)
	if err != nil {
		http.Error(w, "Purchase not found or not owned by user", http.StatusBadRequest)
		return
	}

	_, err = db.Exec("UPDATE users SET balance = balance + ? WHERE username = ?", paidAmount, claims.Username)
	if err != nil {
		http.Error(w, "Failed to update user balance", http.StatusInternalServerError)
		return
	}

	_, err = db.Exec("DELETE FROM purchase_logs WHERE id = ?", purchaseID)
	if err != nil {
		http.Error(w, "Failed to delete purchase log", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/home", http.StatusSeeOther)
}

func main() {
	initDB()
	_, err := db.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		log.Fatal("Failed to set WAL mode;", err)
	}

	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/home", homeHandler)
	http.HandleFunc("/buy", buyHandler)
	http.HandleFunc("/sell", sellHandler)

	fmt.Println("Server started at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
