package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Database struct {
	db *sql.DB
}

type Page struct {
	Title string
	Body  []byte
}

func (p *Page) new(db *sql.DB) (sql.Result, error) {
	return db.Exec("INSERT INTO pages (title, content) VALUES ($1, $2)", p.Title, p.Body)
}

func (p *Page) save(db *sql.DB) (sql.Result, error) {
	return db.Exec("UPDATE pages SET content = $2 WHERE title = $1", p.Title, p.Body)
}

func loadPage(db *sql.DB, title string) (*Page, error) {
	row := db.QueryRow("SELECT title, content FROM pages WHERE title = $1 LIMIT 1", title)
	return rowToPage(row)
}

func printSqlResult(res sql.Result) {
	id, _ := res.LastInsertId()
	fmt.Printf("id inserted: %v", id)
	rowsNum, _ := res.RowsAffected()
	fmt.Printf("Amount of rows affected: %v", rowsNum)
}

func rowToPage(row *sql.Row) (*Page, error) {
	p := Page{}
	if err := row.Scan(&p.Title, &p.Body); err != nil {
		return nil, err
	}
	return &p, nil
}

func rowsToPages(rows *sql.Rows) (*[]Page, error) {
	pages := []Page{}
	for rows.Next() {
		p := Page{}
		if err := rows.Scan(&p.Title, &p.Body); err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	return &pages, nil
}

var templates = template.Must(template.ParseFiles("templates/edit.html", "templates/view.html"))
var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	// ParseFiles only uses filename
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string, db *sql.DB) {
	p, err := loadPage(db, title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	renderTemplate(w, "view", p)
}

func editHandler(w http.ResponseWriter, r *http.Request, title string, db *sql.DB) {
	p, err := loadPage(db, title)
	if err != nil {
		p = &Page{Title: title}
	}
	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string, db *sql.DB) {
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	res, err := p.save(db)
	if err != nil {
		res, err := p.new(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		printSqlResult(res)
	}
	printSqlResult(res)
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/view/FrontPage", http.StatusFound)
}

// Validation
func (db Database) makeHandler(fn func(http.ResponseWriter, *http.Request, string, *sql.DB)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}

		fn(w, r, m[2], db.db)
	}
}

func main() {
	db, err := sql.Open("pgx", "postgresql://localhost:5432/wiki")
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to database")

	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(1 * time.Second)
	db.SetConnMaxLifetime(30 * time.Second)
	wikiDB := Database{db}

	http.HandleFunc("/view/", wikiDB.makeHandler(viewHandler))
	http.HandleFunc("/edit/", wikiDB.makeHandler(editHandler))
	http.HandleFunc("/save/", wikiDB.makeHandler(saveHandler))
	http.HandleFunc("/", rootHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
