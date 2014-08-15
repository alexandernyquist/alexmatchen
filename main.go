package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"html/template"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	daysToShow    = 3
	cacheDuration = 10 * time.Hour
	tvmatchenUrl  = "http://www.tvmatchen.nu/"
)

var (
	multipleSpaces = regexp.MustCompile(`\s+`)
	leagues        = []string{"Premier League", "Ligue 1", "Championship", "Allsvenskan"}
	schedule       map[string][]*match
	lastRefresh    time.Time
	mu             sync.RWMutex
)

type (
	match struct {
		name    string
		league  string
		channel string
		time    string
	}
)

// Convert a match to a pretty printable string.
func (m *match) String() string {
	return fmt.Sprintf("* %s %s (%s, %s)", m.time, m.name, m.league, m.channel)
}

// Refresh data from TV-matchen.
func refreshSchedule() {
	mu.Lock()
	fmt.Printf("Refreshing schedule..")
	defer func() {
		lastRefresh = time.Now()
		mu.Unlock()
		fmt.Println(".done")
	}()

	doc, err := goquery.NewDocument(tvmatchenUrl)
	if err != nil {
		panic(err)
	}

	schedule = make(map[string][]*match, daysToShow)

	days := doc.Find("h2.day-name")
	days.Each(func(i int, s *goquery.Selection) {
		if i >= daysToShow {
			return
		}

		day := s.Find("span.day-name-inner")
		date, _ := day.Attr("id")
		date = strings.Replace(date, "match-day-", "", -1)

		schedule[date] = []*match{}

		matchTable := s.Next()
		matchTable.Find(".sport-name-fotboll").Each(func(mi int, ms *goquery.Selection) {
			name := ms.Find(".match-name").Text()
			league := ms.Find(".league").Text()

			ms.Find(".league").Find("a").Each(func(ai int, as *goquery.Selection) {
				league = strings.Replace(league, as.Text(), "", -1)
			})

			league = strings.Replace(league, "\n", " ", -1)
			league = multipleSpaces.ReplaceAllString(league, " ")
			league = strings.Trim(league, " ")

			interested := false

			// Check if we are interested in this league
			for _, l := range leagues {
				if strings.Contains(league, l) {
					interested = true
					break
				}
			}

			if !interested {
				return
			}

			channelElement := ms.Find(".channel .channel-item")
			channel, _ := channelElement.Attr("title")

			time := ms.Find(".time .field-content").Text()

			schedule[date] = append(schedule[date], &match{
				name:    name,
				league:  league,
				channel: channel,
				time:    time,
			})
		})
	})
}

func main() {
	refreshSchedule()

	t := template.New("t")
	t, err := t.Parse(htmlTemplate)
	if err != nil {
		panic(err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("Incoming request..\n")

		// Check if we should update schedule
		elapsed := time.Since(lastRefresh)
		if elapsed > cacheDuration {
			refreshSchedule()
		}

		err = t.Execute(w, schedule)
		if err != nil {
			panic(err)
		}
	})

	fmt.Printf("Server listening on :8080\n")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}
}

const (
	htmlTemplate = `
<html>
	<head>
		<title>Match på TV:n</title>
	</head>
	<body>
		Fotboll på TV:n.
		
		{{range $day, $matches := .}}
			<h2>{{ $day }}</h2>
			<ul>
				{{range $match := $matches}}
					<li>{{$match.String}}</li>
				{{end}}
			</ul>
		{{end}}
	</body>
</html>
`
)
