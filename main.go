package main

import (
	"database/sql"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	_ "github.com/mattn/go-sqlite3"
	"html/template"
	"net/http"
	"strconv"
	"strings"
)

var db *sql.DB

type PageData struct {
	Id		int
	Title	string
	Desc	string
	Date	string
	Link	string
}

type Data struct {
	News 	[]PageData
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("templates/index.html")
	rows, err := db.Query("SELECT * FROM aggr ORDER BY id DESC LIMIT 100;")
	if err != nil {
		fmt.Println(err.Error())
		w.WriteHeader(http.StatusNotFound)
		return
	}
	var data Data
	for rows.Next() {
		pageData := PageData{}
		err = rows.Scan(&pageData.Id, &pageData.Title, &pageData.Desc, &pageData.Date, &pageData.Link)
		data.News = append(data.News, pageData)
	}
	t.Execute(w, data)
}

func handleFindNewsByName(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	if name == "" {
		w.Write([]byte("<div>You insert nothing for searching. Try to reload page or change query.</div>"))
		return
	}
	fmt.Printf("Got %s for search\n", name)
	var resp []byte

	rows, err := db.Query("select * from aggr where title like ?", "%" + name + "%")
	if err != nil {
		fmt.Println(err.Error())
		w.WriteHeader(http.StatusNotFound)
		return
	}

	for rows.Next() {
		pageData := PageData{}
		err = rows.Scan(&pageData.Id, &pageData.Title, &pageData.Desc, &pageData.Date, &pageData.Link)

		cur := fmt.Sprintf(`<div class="news-container">
					<div class="news">
						<a class="news-ref" href="%s">%s</a>
						<p>%s</p>
						<p class="news-time">%s</p>
					</div>
				</div>`, pageData.Link, pageData.Title, pageData.Desc, pageData.Date)


		resp = append(resp, []byte(cur)...)
	}

	if len(resp) == 0 {
		w.Write([]byte("<div>Nothing found :c. Try to reload page or change query.</div>"))
		return
	}
	w.Write(resp)
}

func handleRunAggregation(w http.ResponseWriter, r *http.Request) {
	algo := r.FormValue("algo")
	if algo == "" {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}
	site := r.FormValue("site")
	site = strings.Trim(site," ")
	num, err := strconv.ParseInt(algo, 10, 8)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var complete, all int

	switch num {
	case 1:
		fmt.Println("Mail.ru parsing started")
		complete, all = parseMail()
	case 2:
		fmt.Println("Yandex.ru parsing started")
		complete, all = parseYandex()

	case 3:
		if site == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Println("RssMail parsing started from site: ", site)
		complete, all = parseRssMail(site)
	case 4:
		if site == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Println("RssYandex parsing started from site", site)
		complete, all = parseRssYandex(site)
	default:
		w.WriteHeader(http.StatusNotFound)
		return
	}
	fmt.Println("Parsing is over. Complete/All : ", complete, "/", all)
	w.Write([]byte(fmt.Sprintf("%d/%d", complete, all)))
	return
}

func parseMail() (complete, all int){
	url := "https://mail.ru"
	// here i can get only title+reference
	doc, err := goquery.NewDocument(url)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	all = doc.Find(".news__list__item").Each(func(i int, s *goquery.Selection) {
		link, err := s.Find("a").Attr("href")
		if !err {
		}
		title := s.Text()
		if addNews(link, title, "", "") != -1 {
			complete++
		}
		//fmt.Println("Link : ", link, " text : ", s.Text())
	}).Length()
	return
}

func parseYandex() (complete, all int){
	url := "https://yandex.ru"
	// here i can get only title+reference
	doc, err := goquery.NewDocument(url)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	all = doc.Find(".list__item-content").Each(func(i int, s *goquery.Selection) {
		link, err := s.Attr("href")
		if !err {
		}
		title := s.Text()
		if addNews(link, title, "", "") != -1 {
			complete++
		}
	}).Length()
	return
}

// https://news.mail.ru/rss/economics/66/
// https://news.mail.ru/rss/sport/
func parseRssMail(site string) (complete, all int){
	// in this case i can get Title+PreBody+Datetime+link
	doc, err := goquery.NewDocument(site)
	if err != nil {
		fmt.Println(err.Error())
	}
	all = doc.Find("item").Each(func(i int, s *goquery.Selection) {
		link := s.Find("guid").Text()
		title := s.Find("title").Text()
		title = strings.Replace(title, "<![CDATA[", "", -1)
		title = strings.Replace(title, "]]>", "", -1)
		desc := s.Find("description").Text()
		desc = strings.Replace(desc, "<![CDATA[", "", -1)
		desc = strings.Replace(desc, "]]>", "", -1)
		pubDate := s.Find("pubDate").Text()
		//fmt.Println("Link : ", link, "\tTitle :", title, "\tDesc : ", desc, "\tpubDate : ", pubDate)

		if addNews(link, title, desc, pubDate) != -1 {
			complete++
		}
	}).Length()
	return
}

// https://news.yandex.ru/communal.rss
// https://news.yandex.ru/science.rss
func parseRssYandex(site string) (complete, all int){
	doc, err := goquery.NewDocument(site)
	if err != nil {
		fmt.Println(err.Error())
	}

	all = doc.Find("item").Each(func(i int, s *goquery.Selection) {
		link := s.Find("guid").Text()
		title := s.Find("title").Text()
		desc := s.Find("description").Text()
		pubDate := s.Find("pubDate").Text()
		//fmt.Println("Link : ", link, "\tTitle :", title, "\tDesc : ", desc, "\tpubDate : ", pubDate)
		if addNews(link, title, desc, pubDate) != -1 {
			complete++
		}
	}).Length()
	return
}

func addNews(link, title, desc, date string) (int64) {
	stmt, err := db.Prepare("INSERT INTO aggr(title, description, date, link) values(?,?,?,?)")
	if err != nil {
		fmt.Println(err.Error())
		return -1
	}
	res, err := stmt.Exec(title, desc, date, link)
	if err != nil {
		fmt.Println(err.Error())
		return -1
	}

	id, err := res.LastInsertId()
	if err != nil {
		fmt.Println(err.Error())
		return -1
	}

	return id
}

func main() {
	val, err := sql.Open("sqlite3", "./sqlite.db")
	db = val
	if err != nil {
		panic(err.Error())
	}
	http.Handle("/static/",
		http.StripPrefix("/static/",
			http.FileServer(http.Dir("static"))))
	http.HandleFunc("/run", handleRunAggregation)
	http.HandleFunc("/drop", handleDrop)
	http.HandleFunc("/search", handleFindNewsByName)
	http.HandleFunc("/", handleIndex)
	http.ListenAndServe(":8088", nil)
}

func handleDrop(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Drop was called")
	res, err := db.Exec("DELETE FROM aggr;")
	if err != nil {
		fmt.Println(err.Error())
		w.WriteHeader(http.StatusNotFound)
		return
	}
	countDeleted, err := res.RowsAffected()
	if err != nil {
		fmt.Println(err.Error())
		w.WriteHeader(http.StatusNotFound)
		return
	}
	fmt.Println("Rows deleted: ", countDeleted)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Rows deleted: %d", countDeleted)))
}