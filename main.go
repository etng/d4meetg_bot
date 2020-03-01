package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	tb "gopkg.in/tucnak/telebot.v2"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
	"sync"

	"os"
	"time"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetPrefix("[d4meetg]")
	cfg := viper.New()
	cfg.SetConfigName("config") // name of config file (without extension)
	cfg.SetConfigType("yaml")   // REQUIRED if the config file does not have the extension in the name
	cfg.AddConfigPath(".")      // optionally look for config in the working directory
	err := cfg.ReadInConfig()   // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	var (
		token          = os.Getenv("TOKEN") // you must add it to your config vars
		domain         = cfg.GetString("domain")
		port           = cfg.GetString("port")
		downloadPrefix = cfg.GetString("download_prefix")
		downloadDir    = cfg.GetString("download_dir")
	)
	if token == "" {
		token = cfg.GetString("token")
	}
	if token == "" { // Handle errors reading the config file
		panic("token is required")
	}
	pref := tb.Settings{
		Token:  token,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tb.NewBot(pref)
	if err != nil {
		log.Fatal(err)
	}
	b.Handle("/hello", func(m *tb.Message) {
		b.Send(m.Sender, "You entered "+m.Payload)
	})
	b.Handle(tb.OnText, func(m *tb.Message) {
		content := strings.TrimSpace(m.Text)
		if content == "" {
			b.Send(m.Sender, "note can not be empty \n send /note xxx to append note")
		} else {
			if strings.HasPrefix(content, "/") {
				b.Send(m.Sender, "maybe run, use /help to check commandss")
				return
			}
			UserNoteFor(m.Sender).Append(content)
			b.Send(m.Sender, "noted, use /notes to view")
		}
	})
	b.Handle(tb.OnPhoto, func(m *tb.Message) {
		name_parts := []string{}
		if m.OriginalSender != nil && m.OriginalSender != m.Sender {
			name_parts = append(name_parts, m.OriginalSender.Username)
		} else {
			name_parts = append(name_parts, m.Sender.Username)
		}
		name_parts = append(name_parts, m.Photo.FileID)
		filename := strings.Join(name_parts, "/") + ".jpg"
		bm, _ := json.MarshalIndent(m, "", " ")
		log.Printf("raw message is %s", bm)
		url := fmt.Sprintf("http://%s:%s%s/%s", domain, port, downloadPrefix, filename)
		realFilename := filepath.Join(downloadDir, filename)
		os.MkdirAll(filepath.Dir(realFilename), 0777)
		if e := b.Download(&m.Photo.File, realFilename); e != nil {
			b.Send(m.Sender, fmt.Sprintf("fail to download %s for %s ", m.Photo.FileURL, e))
		} else {
			b.Send(m.Sender, fmt.Sprintf("downloaded %s", url))
		}

	})
	b.Handle("/notes", func(m *tb.Message) {
		notes := UserNoteFor(m.Sender).Read()
		if notes == "" {
			b.Send(m.Sender, "you have note notes\n send /note xxx to append note")
		} else {
			b.Send(m.Sender, notes)
		}
	})
	b.Handle("/note", func(m *tb.Message) {
		content := strings.TrimSpace(m.Payload)
		if content == "" {
			b.Send(m.Sender, "note can not be empty \n send /note xxx to append note")
		} else {
			UserNoteFor(m.Sender).Append(content)
			b.Send(m.Sender, "noted, use /notes to view")
		}
	})

	b.Handle("/clear_notes", func(m *tb.Message) {
		UserNoteFor(m.Sender).Clear()
		b.Send(m.Sender, "I have nothing to lose now!")
	})
	b.Handle("/help", func(m *tb.Message) {
		b.Send(m.Sender, `
																																											* /help             show this help
																																											* /note xxx         save note xxx
																																											* /clear_notes         clear your notes
																																											* /notes            view your notes
																																											* forward photo message   save your photo and give your an url
																																											`)
	})

	go b.Start()

	router := gin.Default()
	router.Static(downloadPrefix, downloadDir)
	go router.Run(fmt.Sprintf(":%s", port))
	chStop := make(chan int)
	<-chStop

}

var dtLayout = "2006-01-02 15:04:05"

type UserNote struct {
	User *tb.User

	filenameOnce sync.Once
	filename     string
}

func UserNoteFor(u *tb.User) *UserNote {
	un := &UserNote{
		User: u,
	}
	return un
}
func (un *UserNote) Filename() string {
	un.filenameOnce.Do(func() {
		un.filename = fmt.Sprintf("notes/%s.txt", un.User.Username)
		os.MkdirAll(filepath.Dir(un.filename), 0777)
	})
	return un.filename
}
func (un *UserNote) Read() string {
	if bContent, e := ioutil.ReadFile(un.Filename()); e == nil {
		return string(bContent)
	}
	return ""
}
func (un *UserNote) Append(msg string) {
	content := fmt.Sprintf(`
%s:
%s`, time.Now().Format(dtLayout), msg)
	if oldContent, e := ioutil.ReadFile(un.Filename()); e == nil {
		content = strings.Join([]string{content, string(oldContent)}, "\n\n")
	}
	ioutil.WriteFile(un.Filename(), []byte(strings.TrimSpace(content)), 0666)
}
func (un *UserNote) Clear() {
	ioutil.WriteFile(un.Filename(), []byte(""), 0666)
}
