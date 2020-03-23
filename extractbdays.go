package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/drive/v3"
)

const (
	filesPath   = "F:\\Scripts\\acLinks"
	imgsPath    = filesPath + "\\imgs"
	dataPath    = "F:\\Scripts\\acLinks\\acVillagerData.txt"
	linkBase    = "https://nookipedia.com/wiki/"
	imgLinkBase = linkBase + "File:"
	credFile    = "credentials.json"
	tokFile     = "token.json"
	calFile     = "calendarid.json"
	folFile     = "folderid.json"
	layoutISO   = "2006-01-02"
	dryRun      = false
	oneRun      = false
)

var monthMap = map[string]int{
	"january": 1,
	"february": 2,
	"march": 3,
	"april": 4,
	"may": 5,
	"june": 6,
	"july": 7,
	"august": 8,
	"september": 9,
	"october": 10,
	"november": 11,
	"december": 12,
}
var downloadedVillagers = map[string]struct{}{}

func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func calendarIdFromFile(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	val, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}

	type Message struct {
		CalendarId string `json:"calendar_id"`
	}
	var m Message
	err = json.Unmarshal(val, &m)
	return m.CalendarId, err
}

func folderIdFromFile(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	val, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}

	type Message struct {
		FolderId string `json:"folder_id"`
	}
	var m Message
	err = json.Unmarshal(val, &m)
	return m.FolderId, err
}

func fileToEventAttachment(file *drive.File) *calendar.EventAttachment {
	return &calendar.EventAttachment{
		Title: file.Name,
		FileId: file.Id,
		FileUrl: file.WebViewLink,
		IconLink: file.IconLink,
		MimeType: file.MimeType,
	}
}

func loadDriveFiles(fillMap *map[string]*calendar.EventAttachment, srv *drive.Service, folId string) {
	lclFillMap := *fillMap
	var fileList *drive.FileList
	var err error
	first := true
	for first || fileList.NextPageToken != "" {
		query := srv.Files.List().Q("'" + folId + "' in parents").PageSize(10).
			Fields("nextPageToken, files(id, name, webViewLink, iconLink, mimeType)")
		if !first {
			query = query.PageToken(fileList.NextPageToken)
		}
		first = false
		fileList, err = query.Do()
		if err != nil {
			log.Fatalf("Unable to retrieve files: %v", err)
		}
		if len(fileList.Files) == 0 {
			fmt.Println("No files found.")
		} else {
			for _, file := range fileList.Files {
				//fmt.Printf("%s (%s)\n", file.Name, file.Id)
				lclFillMap[file.Name[:len(file.Name) - 4]] = fileToEventAttachment(file)
			}
		}
	}
	*fillMap = lclFillMap
}

func addBirthday(srv *calendar.Service, namesMap map[string]string, driveImgs map[string]*calendar.EventAttachment, calId, fname, bday string) string {
	start, err := time.Parse(layoutISO, "2020-" + bday)
	if err != nil {
		log.Fatalf("Unable to parse \"%v\": %v", "2020-" + bday, err)
	}
	end := start.AddDate(0, 0, 1)

	cleanName := namesMap[fname]
	cutIndex := strings.Index(cleanName, "(")
	if cutIndex >= 0 {
		cleanName = cleanName[:cutIndex]
	}
	cleanName = strings.Trim(cleanName, " ")

	possName := cleanName
	if strings.ToLower(possName)[len(possName) - 1] != 's' {
		possName += "'s"
	} else {
		possName += "'"
	}

	bdayEvent := &calendar.Event{
		Summary: possName + " Birthday",
		Description: "It's " + possName + " birthday today!\n" + linkBase + fname,
		Start: &calendar.EventDateTime{
			Date: start.Format(layoutISO),
			TimeZone: "America/Toronto",
		},
		End: &calendar.EventDateTime{
			Date: end.Format(layoutISO),
			TimeZone: "America/Toronto",
		},
		Recurrence: []string {
			"RRULE:FREQ=YEARLY",
		},
		Transparency: "transparent",
	}

	var outEvent *calendar.Event
	if !dryRun {
		outEvent, err = srv.Events.Insert(calId, bdayEvent).Do()
		if err != nil {
			log.Fatalf("Unable to add a new event: %v", err)
		}

		atmt, ok := driveImgs[fname]
		if ok {
			updateEvent := &calendar.Event{
				Attachments: []*calendar.EventAttachment{
					atmt,
				},
			}
			_, err := srv.Events.Patch(calId, outEvent.Id, updateEvent).SupportsAttachments(true).Do()
			if err != nil {
				log.Fatalf("Unable to add attachment %v to event %v: %v", fname, outEvent.Id, err)
			}
		}
	}

	log.Printf("Created birthday event for %v on %v!", namesMap[fname], bday)

	if dryRun {
		return "-1"
	}
	return outEvent.Id
}

func main() {
	log.Println("Loading credentials.")

	b, err := ioutil.ReadFile(credFile)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}
	_, err = ioutil.ReadFile(calFile)
	if err != nil {
		log.Fatalf("Unable to read calendar ID file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, calendar.CalendarEventsScope, drive.DriveReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	csrv, err := calendar.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve Calendar client: %v", err)
	}

	// Read the calendar ID
	calId, err := calendarIdFromFile(calFile)
	if err != nil {
		log.Fatalf("Unable to read the calendar ID: %v", err)
	}
	fmt.Println("Calendar ID:", calId)

	// Read the folder ID
	folId, err := folderIdFromFile(folFile)
	if err != nil {
		log.Fatalf("Unable to read the folder ID: %v", err)
	}
	fmt.Println("Folder ID:", folId)

	dsrv, err := drive.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve Drive client: %v", err)
	}

	// Get the folder
	folder, err := dsrv.Files.Get(folId).Do()
	if err != nil {
		log.Fatalf("Unable to get Drive folder: %v", err)
	}

	fmt.Println(folder.Name)
	//dsrv.Files.List().Q(folId + " in parents")

	log.Println("Loading attachment images.")

	driveImgs := make(map[string]*calendar.EventAttachment)
	loadDriveFiles(&driveImgs, dsrv, folId)

	contents, err := ioutil.ReadDir(imgsPath)
	if err != nil {
		fmt.Println(err)
	}

	for _, v := range contents {
		imgName := v.Name()
		imgName = imgName[:len(imgName) - 4]
		downloadedVillagers[imgName] = struct{}{}
	}

	log.Println("Loading villager data.")

	dataFile, err := os.Open(dataPath)
	if err != nil {
		fmt.Println(err)
	}
	defer dataFile.Close()

	names := make(map[string]string)
	imgs := make(map[string]string)
	scanner := bufio.NewScanner(dataFile)
	for scanner.Scan() {
		data := scanner.Text()
		dataParts := strings.Split(data, ",")
		names[dataParts[0]] = dataParts[1]
		imgs[dataParts[0]] = dataParts[2]
	}

	log.Println("Handling birthdays.")

	count := 0
	for fname, cleanName := range names {
		/*name := v.Name()
		ext := path.Ext(name)
		if ext != ".html" {
			continue
		}*/
		//fname := name[:(len(name) - 5)]
		//cleanName, cok := names[fname]
		imgName, ok := imgs[fname]
		if !ok {
			continue
		}
		imgLink := imgLinkBase + imgName
		bday, err := extractBDay(fname + ".html")
		if err != nil && len(bday) > 0 {
			fmt.Println(err)
			return
		}

		_, err = downloadImg(fname, imgName, imgLink)
		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Printf("%d: %v (%v) - %v - %v\n", count, cleanName, fname, bday, imgName)

		if len(bday) == 5 {
			addBirthday(csrv, names, driveImgs, calId, fname, bday)
		} else {
			log.Printf("%v doesn't have a birthday!", cleanName)
		}
		count++
		if oneRun {
			return
		}
	}
}

func extractBDay(fileName string) (string, error) {
	contents, err := ioutil.ReadFile(path.Join(filesPath, fileName))
	if err != nil {
		return "", err
	}

	bdayDetector := regexp.MustCompile("(?ims)Infobox-villager-birthday.*?>(?:<a.*?>)?(.*?)(?:</a>|</td)")
	matches := bdayDetector.FindStringSubmatch(string(contents))
	if len(matches) != 2 {
		return "", nil
	}
	bdayRaw := matches[1]
	bdayWords := bdayRaw
	cutIndex := strings.Index(bdayRaw, "<sup")
	if cutIndex >= 0 {
		bdayWords = bdayRaw[:cutIndex]
	}

	bdayParts := strings.Split(strings.Trim(bdayWords, " "), " ")
	if len(bdayParts) != 2 {
		fmt.Printf("len(bdayParts) != 2: %v\n", bdayParts)
		return "", nil
	}
	bdayMonth := monthMap[strings.ToLower(bdayParts[0])]
	bdayDay, err := strconv.ParseInt(bdayParts[1], 10, 32)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%02d-%02d", bdayMonth, bdayDay), nil
}

func downloadImg(fname, iname, link string) (string, error) {
	if _, ok := downloadedVillagers[fname]; ok {
		return path.Join(imgsPath, fname + ".png"), nil
	}

	ciname, err := url.QueryUnescape(iname)
	if err != nil {
		return "", err
	}
	fmt.Println(ciname)
	escCiname := fmt.Sprintf("%q", ciname)
	escLink := fmt.Sprintf("%q", link)
	fmt.Println(escCiname)
	fmt.Println(link)
	fmt.Println(escLink)
	cmd := exec.Command("bash",
		"-c",
		"`wget64 --verbose --no-directories --page-requisites --accept=" + escCiname + "* -e robots=off --adjust-extension --recursive --level=1 --user-agent='Mozilla/5.0 (Windows NT 10.0; rv:68.0) Gecko/20100101 Firefox/68.0' " + escLink + "`")
	cmd.Dir = imgsPath
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	fmt.Println(cmd.String())
	out, err := cmd.Output()
	fmt.Println(string(out))
	if err != nil {
		return "", err
	}

	finalPath := path.Join(imgsPath, fname + ".png")
	err = os.Rename(path.Join(imgsPath, ciname), finalPath)
	if err != nil {
		return "", err
	}

	return finalPath, nil
}