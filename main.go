package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

func getTokenMiro() (string, error) {
	token := os.Getenv("TOKEN_MIRO")
	if token == "" {
		return "", errors.New("environmental variable TOKEN_MIRO does not set")
	}
	return token, nil
}

func getBoardID() (string, error) {
	boardId := os.Getenv("BOARD_ID")
	if boardId == "" {
		return "", errors.New("environmental variable BOARD_ID does not set")
	}
	return boardId, nil
}

func getBoardName(id string, token string) (string, string) {
	url := "https://api.miro.com/v1/boards/" + id
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("Could not make request")
		return "", ""
	}
	defer res.Body.Close()
	var data map[string]interface{}
	err = json.NewDecoder(res.Body).Decode(&data)
	if err != nil {
		log.Println("error in decoding response body")
		return "", ""
	}
	return data["name"].(string), data["viewLink"].(string)
}

func getWidgets(id string, token string) []byte {
	url := "https://api.miro.com/v1/boards/" + id + "/widgets/"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic("Could not make request")
	}
	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)
	return body
}

const (
	timeToSleep = time.Duration(10 * time.Minute)
)

func sendNotification(boardId string, chatID int64) error {
	boardName := boards[boardId].Name
	link := boards[boardId].Link
	log.Println("On board " + boardName + " changes were made: " + link)
	reqBody := Message{
		ChatID: chatID,
		Text:   "On board " + boardName + " changes were made: " + link,
	}
	return sendMessage(reqBody)
}

func notifyOnChangesWorker(boardId string, token string, quit chan bool, chatID int64) error {
	haveChangesBefore := false
	widgetsOld := getWidgets(boardId, token)
	for {
		select {
		case <-quit:
			close(quit)
			return nil
		default:
			time.Sleep(timeToSleep)
			widgetsNew := getWidgets(boardId, token)
			res := bytes.Compare(widgetsOld, widgetsNew)
			haveChangesNow := res != 0
			if haveChangesBefore && !haveChangesNow {
				err := sendNotification(boardId, chatID)
				if err != nil {
					log.Println(err.Error())
					return err
				}
			}
			widgetsOld = widgetsNew
			haveChangesBefore = haveChangesNow
		}
	}
}

type Board struct {
	Name       string
	Link       string
	StopWorker chan bool
}

var boards = make(map[string]Board)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Print("No .env file found")
	}
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe("0.0.0.0:7000", nil))
}

func handler(response http.ResponseWriter, req *http.Request) {
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Println("error during copy request ", err)
		return
	}
	reader := ioutil.NopCloser(bytes.NewReader(data))
	tryToHandleMessage(reader)
}

func tryToHandleMessage(req io.ReadCloser) {
	var body = webhookMessage{}
	if err := json.NewDecoder(req).Decode(&body); err != nil {
		log.Println("could not decode request body to Message ", err)
		return
	}

	if body.Message.Text == "" {
		return
	}

	chatID := body.Message.Chat.ID

	if body.Message.Text == "/start" {
		if err := startMonitoringBoard(chatID); err != nil {
			log.Println("could not start monitoring board ", err)
		}
		return
	}

	if body.Message.Text == "/stop" {
		if err := stopMonitoringBoard(chatID); err != nil {
			log.Println("could not stop monitoring board ", err)
		}
		return
	}

	if body.Message.Text == "/help" {
		if err := showHelp(chatID); err != nil {
			log.Println("could not send message to show help ", err)
		}
		return
	}
}

func startMonitoringBoard(chatID int64) error {
	boardId, err := getBoardID()
	if err != nil {
		return err
	}
	token, err := getTokenMiro()
	if err != nil {
		return err
	}
	boardName, link := getBoardName(boardId, token)
	stop := make(chan bool, 2)
	board := Board{Name: boardName, Link: link, StopWorker: stop}
	boards[boardId] = board
	go notifyOnChangesWorker(boardId, token, stop, chatID)
	log.Print("Start monitoring board " + boardName + " (ID: " + boardId + ")")
	return nil
}

func stopMonitoringBoard(chatID int64) error {
	boardId, err := getBoardID()
	if err != nil {
		return err
	}
	boards[boardId].StopWorker <- true
	token, err := getTokenMiro()
	if err != nil {
		return err
	}
	boardName, _ := getBoardName(boardId, token)
	log.Print("Stop monitoring board " + boardName + " (ID: " + boardId + ")")
	return nil
}

func showHelp(chatID int64) error {
	reqBody := Message{
		ChatID: chatID,
		Text:   "Bot will inform you about any changes on a board.\nTo start monitoring a board type /start and send the board ID in the next message.\nTo stop monitoring a board type /stop and send the board ID in the next message.",
	}
	return sendMessage(reqBody)
}

func sendMessage(reqBody interface{}) error {
	reqBytes, err := json.Marshal(&reqBody)
	if err != nil {
		return err
	}
	log.Println(string(reqBytes))

	token := os.Getenv("TOKEN_BOT")
	res, err := http.Post("https://api.telegram.org/bot"+token+"/sendMessage", "application/json", bytes.NewBuffer(reqBytes))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return errors.New("unexpected status " + res.Status)
	}
	return nil
}

type User struct {
	Id        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	UserName  string `json:"username"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type Message struct {
	ChatID   int64  `json:"chat_id"`
	Text     string `json:"text"`
	Chat     Chat   `json:"chat"`
	FromUser User   `json:"from"`
}

type webhookCallbackQuery struct {
	CallbackQuery CallbackQuery `json:"callback_query"`
}

type webhookMessage struct {
	Message Message `json:"message"`
}

type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

type Answer struct {
	Message
	ChatID      int64                `json:"chat_id"`
	ReplyMarkup InlineKeyboardMarkup `json:"reply_markup"`
}

type CallbackQuery struct {
	Id       string `json:"id"`
	FromUser User   `json:"from"`
	Message  Answer `json:"message"`
	Data     string `json:"data"`
}

type AnswerCallbackQuery struct {
	CallbackQueryId string `json:"callback_query_id"`
	Text            string `json:"text"`
}
