package main

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// I originally thought about making an Event struct and generalizing the
// problem. Then I realized this is getting used exactly once, and that's silly.
// Then I thought about doing it anyway, and then I spent perhaps a longer time
// convincing myself that "no, it's _really_ not necessary to generalize in this
// case" than it would have taken to build. Oh well.

type Party struct {
	gorm.Model
	Name         string
	Attendees    []Attendee
	SongRequests []SongRequest
}

type SongRequest struct {
	gorm.Model
	Song    string
	PartyID uint
	Party   Party
}

type State int

const (
	NotResponded State = iota
	Attending
	NotAttending
)

func (s State) String() string {
	switch s {
	case NotResponded:
		return "Not responded"
	case Attending:
		return "Attending"
	case NotAttending:
		return "Not attending"
	}
	panic("Invalid state")
}

type Attendee struct {
	gorm.Model
	Name                     string
	CeremonyResponse         State
	RehearsalResponse        State
	InvitedToRehearsalDinner bool
	PartyID                  uint
	Party                    Party
}

//go:embed templates
var templateFS embed.FS

//go:embed public
var staticFS embed.FS

func main() {
	admin_pass := os.Getenv("ADMIN_PASSWORD")
	if admin_pass == "" {
		panic("No admin password provided")
	}
	db, err := gorm.Open(sqlite.Open("attendees.db"), &gorm.Config{})
	db.AutoMigrate(&Party{}, &Attendee{}, &SongRequest{})
	if err != nil {
		panic(err)
	}
	tmpl, err := template.ParseFS(templateFS, "templates/**")
	if err != nil {
		panic(err)
	}

	staticFS, err := fs.Sub(staticFS, "public")
	if err != nil {
		panic(err)
	}
	http.Handle("/", http.FileServer(http.FS(staticFS)))

	// These functions aren't really designed with security in mind. Anyone who
	// can do a bit of research on the internet should be able to get the names
	// of a lot of my close friends and relatives anyway, so trying to set up
	// perfect cryptographic security to prevent false form submissions doesn't
	// do that much good when the "secrets" (names) are easily guessed anyway.
	// The only attack that I'm doing any effort to prevent here is enumerating
	// a list of all the guests who will be attending, for my guests' privacy.
	http.HandleFunc("/rsvp/", func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rsvp/" {
			http.NotFound(rw, r)
			return
		}
		if r.Method == "GET" {
			err := tmpl.ExecuteTemplate(rw, "rsvp-find-party.html", map[string]interface{}{})
			if err != nil {
				log.Println(err)
			}
			return
		}
		if r.Method != "POST" {
			http.Error(rw, "Expecting GET or POST", http.StatusMethodNotAllowed)
			return
		}
		err := r.ParseForm()
		if err != nil {
			log.Println(err)
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		steps, ok := r.Form["step"]
		if !ok {
			http.Error(rw, "Required parameter 'step' not found", http.StatusBadRequest)
			return
		}
		step := steps[0]

		switch step {
		case "find-party": // Find the invitee; display first form (either rehearsal dinner or ceremony, as needed)
			// TODO: bypass this step if already registered
			names, ok := r.Form["name"]
			if !ok {
				http.Error(rw, "Required parameter 'name' not found", http.StatusBadRequest)
				return
			}
			name := names[0]
			var attendee Attendee
			res := db.Where("lower(name) = ?", strings.ToLower(name)).Preload("Party.Attendees").First(&attendee)
			if res.RowsAffected != 1 {
				err := tmpl.ExecuteTemplate(rw, "rsvp-find-party.html", map[string]interface{}{
					"Status": template.HTML("We're having trouble finding your invitation. " +
						"Please verify that your name is spelled the same way it is on your invitation, " +
						"or <a href='mailto:kathe.trahan@gmail.com'>contact Käthe and Chandler</a>."),
					"Name": attendee.Name,
				})
				if err != nil {
					log.Println(err)
				}
				return
			}
			if attendee.CeremonyResponse != NotResponded {
				db.Where("name = ?", attendee.Name).Preload("Party.SongRequests").First(&attendee)
				err := tmpl.ExecuteTemplate(rw, "rsvp-confirm.html", map[string]interface{}{
					"Name":  attendee.Name,
					"Songs": attendee.Party.SongRequests,
				})
				if err != nil {
					log.Println(err)
				}
				return
			}
			// For any given party, either the entirety of the party is invited
			// to the rehearsal dinner, or no one is, so we can generalize this
			// to show the rehearsal dinner form iff
			if attendee.InvitedToRehearsalDinner {
				err := tmpl.ExecuteTemplate(rw, "rsvp-show-party.html", map[string]interface{}{
					"Event":     "Rehearsal Dinner",
					"Date":      "Friday, September 2, 2022",
					"Time":      "6:00 in the evening",
					"Location":  template.HTML(`<a href="https://goo.gl/maps/mA5SgjGvSEFDaMDf8">The Trahans&rsquo;<br>285 Harbor Lane<br>Shoreview, MN 55126</a>`),
					"DressCode": template.HTML("Dressy casual<br>We will be outside, so layers may be appropriate."),
					"Attendees": attendee.Party.Attendees,
					"Step":      "confirm-rehearsal-dinner",
					"Button":    "Next event",
				})
				if err != nil {
					log.Println(err)
				}
				return
			} else { // Everyone is invited to the ceremony
				err := tmpl.ExecuteTemplate(rw, "rsvp-show-party.html", map[string]interface{}{
					"Event":     "Ceremony and Reception",
					"Date":      "Saturday, September 3, 2022",
					"Time":      "4:00 in the afternoon",
					"Location":  template.HTML(`<a href="https://goo.gl/maps/rQG8BiwwJcwydun17">Silverwood Great Hall<br>2500 County Road E<br>St. Anthony, MN 55421</a>`),
					"DressCode": template.HTML("Semiformal attire requested."),
					"Attendees": attendee.Party.Attendees,
					"Step":      "confirm-ceremony",
					"Button":    "Confirm choices",
				})
				if err != nil {
					log.Println(err)
				}
				return
			}
		case "confirm-rehearsal-dinner":
			names, ok := r.Form["name"]
			if !ok {
				http.Error(rw, "Required parameter 'name' not found", http.StatusBadRequest)
				return
			}
			for i, name := range names {
				var attendee Attendee
				res := db.Where("name = ?", name).First(&attendee)
				if res.RowsAffected != 1 {
					log.Printf("Could not find %v", name)
					http.Error(rw, "Could not find name", http.StatusInternalServerError)
				}
				if r.Form[fmt.Sprintf("attending[%d]", i)][0] == "yes" {
					attendee.RehearsalResponse = Attending
				} else {
					attendee.RehearsalResponse = NotAttending
				}
				db.Save(&attendee)
			}
			name := names[0]
			var attendee Attendee
			res := db.Where("name = ?", name).Preload("Party.Attendees").First(&attendee)
			if res.RowsAffected != 1 {
				log.Printf("Could not find attendee %v", name)
				return
			}
			err := tmpl.ExecuteTemplate(rw, "rsvp-show-party.html", map[string]interface{}{
				"Event":     "Ceremony and Reception",
				"Date":      "Saturday, September 3, 2022",
				"Time":      "4:00 in the afternoon",
				"Location":  template.HTML(`<a href="https://goo.gl/maps/rQG8BiwwJcwydun17">Silverwood Great Hall<br>2500 County Road E<br>St. Anthony, MN 55421</a>`),
				"DressCode": template.HTML("Semiformal attire requested."),
				"Attendees": attendee.Party.Attendees,
				"Step":      "confirm-ceremony",
				"Button":    "Confirm choices",
			})
			if err != nil {
				log.Println(err)
			}
			return
		case "confirm-ceremony":
			names, ok := r.Form["name"]
			if !ok {
				http.Error(rw, "Required parameter 'name' not found", http.StatusBadRequest)
				return
			}
			for i, name := range names {
				var attendee Attendee
				res := db.Where("name = ?", name).First(&attendee)
				if res.RowsAffected != 1 {
					log.Printf("Could not find %v", name)
					http.Error(rw, "Could not find name", http.StatusBadRequest)
					return
				}
				ceremonyResponse, ok := r.Form[fmt.Sprintf("attending[%d]", i)]
				if !ok {
					http.Error(rw, fmt.Sprintf("Please submit a response for %v", name), http.StatusBadRequest)
					return
				}
				if ceremonyResponse[0] == "yes" {
					attendee.CeremonyResponse = Attending
				} else {
					attendee.CeremonyResponse = NotAttending
				}
				db.Save(&attendee)
			}
			name := names[0]
			var attendee Attendee
			res := db.Where("name = ?", name).Preload("Party.SongRequests").First(&attendee)
			if res.RowsAffected != 1 {
				log.Printf("Could not find attendee %v", name)
				http.Error(rw, "Could not find attendee", http.StatusBadRequest)
				return
			}
			err := tmpl.ExecuteTemplate(rw, "rsvp-confirm.html", map[string]interface{}{
				"Name":  name,
				"Songs": attendee.Party.SongRequests,
			})
			if err != nil {
				log.Println(err)
			}
			return
		case "add-song":
			names, ok := r.Form["name"]
			if !ok {
				http.Error(rw, "Required parameter 'name' not found", http.StatusBadRequest)
				return
			}
			name := names[0]
			var attendee Attendee
			res := db.Where("name = ?", name).First(&attendee)
			if res.RowsAffected != 1 {
				log.Printf("Could not find attendee %v", name)
				http.Error(rw, "Could not find attendee", http.StatusBadRequest)
				return
			}
			songs, ok := r.Form["song"]
			if !ok {
				http.Error(rw, "Required parameter 'song' not found", http.StatusBadRequest)
				return
			}
			song := songs[0]
			songRequest := SongRequest{
				Song:    song,
				PartyID: attendee.PartyID,
			}
			db.Save(&songRequest)
			db.Where("name = ?", name).Preload("Party.SongRequests").First(&attendee)
			err := tmpl.ExecuteTemplate(rw, "rsvp-confirm.html", map[string]interface{}{
				"Name":  name,
				"Songs": attendee.Party.SongRequests,
			})
			if err != nil {
				log.Println(err)
			}
			return
		}
	})

	http.HandleFunc("/attendees", func(w http.ResponseWriter, r *http.Request) {
		_, p, ok := r.BasicAuth()
		if !ok || p != admin_pass {
			w.Header().Set("WWW-Authenticate", "basic")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		var parties []Party
		db.Preload("Attendees.Party.SongRequests").Find(&parties)
		err := tmpl.ExecuteTemplate(w, "attendees.html", map[string]interface{}{
			"Parties": parties,
		})
		if err != nil {
			log.Println(err)
		}
	})
	log.Println("Serving on localhost:8080")
	log.Fatal(http.ListenAndServe("127.0.0.1:8080", nil))
}
