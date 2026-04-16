// Package tgbotapi has functions and types used for interacting with
// the Telegram Bot API.
package tgbotapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// HTTPClient is the type needed for the bot to perform HTTP requests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// RequestExecutor is the interface for making requests to the Telegram Bot API.
type RequestExecutor interface {
	SetDebug(debug bool)
	Debug() bool
	SetApiEndpoint(apiEndpoint string)
	MakeRequest(endpoint string, params Params) (*APIResponse, error)
	MakeRequestWithContext(ctx context.Context, endpoint string, params Params) (*APIResponse, error)
	Request(c Chattable) (*APIResponse, error)
	RequestWithContext(ctx context.Context, c Chattable) (*APIResponse, error)
	UploadFiles(endpoint string, params Params, files []RequestFile) (*APIResponse, error)
}
type BaseExecutor struct {
	token       string
	apiEndpoint string
	client      HTTPClient
	debug       bool
}

func (b *BaseExecutor) SetDebug(debug bool) {
	b.debug = debug
}

func (b *BaseExecutor) Debug() bool {
	return b.debug
}

// SetAPIEndpoint changes the Telegram Bot API endpoint used by the instance.
func (b *BaseExecutor) SetApiEndpoint(apiEndpoint string) {
	b.apiEndpoint = apiEndpoint
}

func (b *BaseExecutor) MakeRequest(endpoint string, params Params) (*APIResponse, error) {
	return b.MakeRequestWithContext(context.Background(), endpoint, params)
}

func (b *BaseExecutor) MakeRequestWithContext(ctx context.Context, endpoint string, params Params) (*APIResponse, error) {
	if b.debug {
		log.Printf("Endpoint: %s, params: %v\n", endpoint, params)
	}

	method := fmt.Sprintf(b.apiEndpoint, b.token, endpoint)

	values := buildParams(params)

	req, err := http.NewRequestWithContext(ctx, "POST", method, strings.NewReader(values.Encode()))
	if err != nil {
		return &APIResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp APIResponse
	bytes, err := b.decodeAPIResponse(resp.Body, &apiResp)
	if err != nil {
		return &apiResp, err
	}

	if b.debug {
		log.Printf("Endpoint: %s, response: %s\n", endpoint, string(bytes))
	}

	if !apiResp.Ok {
		var parameters ResponseParameters

		if apiResp.Parameters != nil {
			parameters = *apiResp.Parameters
		}

		return &apiResp, &Error{
			Code:               apiResp.ErrorCode,
			Message:            apiResp.Description,
			ResponseParameters: parameters,
		}
	}

	return &apiResp, nil
}

func (b *BaseExecutor) Request(c Chattable) (*APIResponse, error) {
	return b.RequestWithContext(context.Background(), c)
}

func (b *BaseExecutor) RequestWithContext(ctx context.Context, c Chattable) (*APIResponse, error) {
	params, err := c.params()
	if err != nil {
		return nil, err
	}

	if t, ok := c.(Fileable); ok {
		files := t.files()

		// If we have files that need to be uploaded, we should delegate the
		// request to UploadFile.
		if hasFilesNeedingUpload(files) {
			return b.UploadFiles(t.method(), params, files)
		}

		// However, if there are no files to be uploaded, there's likely things
		// that need to be turned into params instead.
		for _, file := range files {
			params[file.Name] = file.Data.SendData()
		}
	}

	return b.MakeRequestWithContext(ctx, c.method(), params)
}

func (b *BaseExecutor) UploadFiles(endpoint string, params Params, files []RequestFile) (*APIResponse, error) {
	r, w := io.Pipe()
	m := multipart.NewWriter(w)

	go func() {
		defer w.Close()
		defer m.Close()

		for field, value := range params {
			if err := m.WriteField(field, value); err != nil {
				w.CloseWithError(err)
				return
			}
		}

		for _, file := range files {
			if file.Data.NeedsUpload() {
				name, reader, err := file.Data.UploadData()
				if err != nil {
					w.CloseWithError(err)
					return
				}

				part, err := m.CreateFormFile(file.Name, name)
				if err != nil {
					w.CloseWithError(err)
					return
				}

				if _, err := io.Copy(part, reader); err != nil {
					w.CloseWithError(err)
					return
				}

				if closer, ok := reader.(io.ReadCloser); ok {
					if err = closer.Close(); err != nil {
						w.CloseWithError(err)
						return
					}
				}
			} else {
				value := file.Data.SendData()

				if err := m.WriteField(file.Name, value); err != nil {
					w.CloseWithError(err)
					return
				}
			}
		}
	}()

	if b.debug {
		log.Printf("Endpoint: %s, params: %v, with %d files\n", endpoint, params, len(files))
	}

	method := fmt.Sprintf(b.apiEndpoint, b.token, endpoint)

	req, err := http.NewRequest("POST", method, r)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", m.FormDataContentType())

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp APIResponse
	bytes, err := b.decodeAPIResponse(resp.Body, &apiResp)
	if err != nil {
		return &apiResp, err
	}

	if b.debug {
		log.Printf("Endpoint: %s, response: %s\n", endpoint, string(bytes))
	}

	if !apiResp.Ok {
		var parameters ResponseParameters

		if apiResp.Parameters != nil {
			parameters = *apiResp.Parameters
		}

		return &apiResp, &Error{
			Message:            apiResp.Description,
			ResponseParameters: parameters,
		}
	}

	return &apiResp, nil
}

func (b *BaseExecutor) decodeAPIResponse(responseBody io.Reader, resp *APIResponse) ([]byte, error) {
	if !b.debug {
		dec := json.NewDecoder(responseBody)
		err := dec.Decode(resp)
		return nil, err
	}

	// if debug, read response body
	data, err := io.ReadAll(responseBody)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, resp)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func NewBaseExecutor(token, apiEndpoint string, client HTTPClient, debug bool) *BaseExecutor {
	return &BaseExecutor{
		token:       token,
		apiEndpoint: apiEndpoint,
		client:      client,
		debug:       debug,
	}
}

func NewApiExecutor(token string) *BaseExecutor {
	return NewBaseExecutor(token, APIEndpoint, &http.Client{}, false)
}

// BotAPI allows you to interact with the Telegram Bot API.
type BotAPI struct {
	Token           string          `json:"token"`
	Buffer          int             `json:"buffer"`
	Self            User            `json:"-"`
	Executor        RequestExecutor `json:"-"`
	shutdownChannel chan interface{}
}

type Listener struct {
	ctx     context.Context
	Matcher func(Update) bool
	Handler func(Update)
}

// MultipleListenerBotAPI is a thread-scoped BotAPI instance.
type MultipleListenerBotAPI struct {
	*BotAPI

	listeners   []*Listener
	listenersMu sync.RWMutex
}

// NewMultipleListenerBotAPI creates a new MultipleListenerBotAPI instance.
func NewMultipleListenerBotAPI(ctx context.Context, bot *BotAPI, timeout int) *MultipleListenerBotAPI {
	s := &MultipleListenerBotAPI{
		BotAPI:    bot,
		listeners: make([]*Listener, 0),
	}
	updates := bot.GetUpdatesChanContext(ctx, UpdateConfig{
		Timeout: timeout,
	})

	go s.run(ctx, updates)

	return s
}

func (s *MultipleListenerBotAPI) Stop() {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()
	for _, l := range s.listeners {
		l.ctx.Done()
	}
	s.listeners = make([]*Listener, 0)
}

func (s *MultipleListenerBotAPI) run(ctx context.Context, updates <-chan Update) {
	for {
		select {
		case <-ctx.Done():
			return
		case update := <-updates:
			s.handleUpdate(update)
		}
	}
}

func (s *MultipleListenerBotAPI) handleUpdate(update Update) {
	s.listenersMu.RLock()
	defer s.listenersMu.RUnlock()
	log.Printf("handleUpdate: %v", update)
	for _, listener := range s.listeners {
		if listener.Matcher(update) {
			go listener.Handler(update)
		}
	}
}

// AddListener adds a listener to the session.
func (s *MultipleListenerBotAPI) AddListener(ctx context.Context, matcher func(Update) bool, handler func(Update)) *Listener {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()

	l := &Listener{
		ctx:     ctx,
		Matcher: matcher,
		Handler: handler,
	}

	s.listeners = append(s.listeners, l)

	return l
}

// RemoveListener removes a listener from the session.
func (s *MultipleListenerBotAPI) RemoveListener(l *Listener) {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()

	for i, listener := range s.listeners {
		if listener == l {
			l.ctx.Done()
			s.listeners = append(s.listeners[:i], s.listeners[i+1:]...)
			return
		}
	}
}

func NewBot(executor RequestExecutor) (*BotAPI, error) {
	return NewBotAPIWithClient(executor)
}

// NewBotAPI creates a new BotAPI instance.
//
// It requires a token, provided by @BotFather on Telegram.
func NewBotAPI(token string) (*BotAPI, error) {
	return NewBotAPIWithClient(NewBaseExecutor(token, APIEndpoint, &http.Client{}, false))
}

// NewBotAPIWithAPIEndpoint creates a new BotAPI instance
// and allows you to pass API endpoint.
//
// It requires a token, provided by @BotFather on Telegram and API endpoint.
func NewBotAPIWithAPIEndpoint(token, apiEndpoint string) (*BotAPI, error) {
	return NewBotAPIWithClient(NewBaseExecutor(token, apiEndpoint, &http.Client{}, false))
}

// NewBotAPIWithClient creates a new BotAPI instance
// and allows you to pass a http.Client.
//
// It requires a token, provided by @BotFather on Telegram and API endpoint.
func NewBotAPIWithClient(executor RequestExecutor) (*BotAPI, error) {
	bot := &BotAPI{
		Executor:        executor,
		Buffer:          100,
		shutdownChannel: make(chan interface{}),
	}

	self, err := bot.GetMe()
	if err != nil {
		return nil, err
	}

	bot.Self = self

	return bot, nil
}

// WithExecutor returns a new BotAPI instance with the given executor.
func (bot *BotAPI) WithExecutor(executor RequestExecutor) *BotAPI {
	newBot := *bot
	newBot.Executor = executor

	return &newBot
}

func buildParams(in Params) url.Values {
	if in == nil {
		return url.Values{}
	}

	out := url.Values{}

	for key, value := range in {
		out.Set(key, value)
	}

	return out
}

// MakeRequest makes a request to a specific endpoint with our token.
func (bot *BotAPI) MakeRequest(endpoint string, params Params) (*APIResponse, error) {
	return bot.MakeRequestWithContext(context.Background(), endpoint, params)
}

// MakeRequestWithContext makes a request to a specific endpoint with our token.
func (bot *BotAPI) MakeRequestWithContext(ctx context.Context, endpoint string, params Params) (*APIResponse, error) {
	if bot.Executor == nil {
		return nil, errors.New("executor is nil")
	}
	return bot.Executor.MakeRequestWithContext(ctx, endpoint, params)
}

// decodeAPIResponse decode response and return slice of bytes if debug enabled.
// If debug disabled, just decode http.Response.Body stream to APIResponse struct
// for efficient memory usage
func (bot *BotAPI) decodeAPIResponse(responseBody io.Reader, resp *APIResponse) ([]byte, error) {
	if bot.Executor == nil || !bot.Executor.Debug() {
		dec := json.NewDecoder(responseBody)
		err := dec.Decode(resp)
		return nil, err
	}

	// if debug, read response body
	data, err := io.ReadAll(responseBody)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, resp)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// UploadFiles makes a request to the API with files.
func (bot *BotAPI) UploadFiles(endpoint string, params Params, files []RequestFile) (*APIResponse, error) {
	if bot.Executor == nil {
		return nil, errors.New("executor is nil")
	}
	return bot.Executor.UploadFiles(endpoint, params, files)
}

// GetFileDirectURL returns direct URL to file
//
// It requires the FileID.
func (bot *BotAPI) GetFileDirectURL(fileID string) (string, error) {
	file, err := bot.GetFile(FileConfig{fileID})

	if err != nil {
		return "", err
	}

	return file.Link(bot.Token), nil
}

// GetMe fetches the currently authenticated bot.
//
// This method is called upon creation to validate the token,
// and so you may get this data from BotAPI.Self without the need for
// another request.
func (bot *BotAPI) GetMe() (User, error) {
	resp, err := bot.MakeRequest("getMe", nil)
	if err != nil {
		return User{}, err
	}

	var user User
	err = json.Unmarshal(resp.Result, &user)

	return user, err
}

// IsMessageToMe returns true if message directed to this bot.
//
// It requires the Message.
func (bot *BotAPI) IsMessageToMe(message Message) bool {
	return strings.Contains(message.Text, "@"+bot.Self.UserName)
}

func hasFilesNeedingUpload(files []RequestFile) bool {
	for _, file := range files {
		if file.Data.NeedsUpload() {
			return true
		}
	}

	return false
}

// Request sends a Chattable to Telegram, and returns the APIResponse.
func (bot *BotAPI) Request(c Chattable) (*APIResponse, error) {
	return bot.RequestWithContext(context.Background(), c)
}

// RequestWithContext sends a Chattable to Telegram with a context, and returns the APIResponse.
func (bot *BotAPI) RequestWithContext(ctx context.Context, c Chattable) (*APIResponse, error) {
	if bot.Executor == nil {
		return nil, errors.New("executor is nil")
	}
	return bot.Executor.RequestWithContext(ctx, c)
}

// Send will send a Chattable item to Telegram and provides the
// returned Message.
func (bot *BotAPI) Send(c Chattable) (Message, error) {
	var message Message

	resp, err := bot.Request(c)
	if err != nil {
		return message, err
	}

	err = json.Unmarshal(resp.Result, &message)

	return message, err
}

// SendMediaGroup sends a media group and returns the resulting messages.
func (bot *BotAPI) SendMediaGroup(config MediaGroupConfig) ([]Message, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return nil, err
	}

	var messages []Message
	err = json.Unmarshal(resp.Result, &messages)

	return messages, err
}

// GetUserProfilePhotos gets a user's profile photos.
//
// It requires UserID.
// Offset and Limit are optional.
func (bot *BotAPI) GetUserProfilePhotos(config UserProfilePhotosConfig) (UserProfilePhotos, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return UserProfilePhotos{}, err
	}

	var profilePhotos UserProfilePhotos
	err = json.Unmarshal(resp.Result, &profilePhotos)

	return profilePhotos, err
}

// GetFile returns a File which can download a file from Telegram.
//
// Requires FileID.
func (bot *BotAPI) GetFile(config FileConfig) (File, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return File{}, err
	}

	var file File
	err = json.Unmarshal(resp.Result, &file)

	return file, err
}

// GetUpdates fetches updates.
// If a WebHook is set, this will not return any data!
//
// Offset, Limit, Timeout, and AllowedUpdates are optional.
// To avoid stale items, set Offset to one higher than the previous item.
// Set Timeout to a large number to reduce requests, so you can get updates
// instantly instead of having to wait between requests.
func (bot *BotAPI) GetUpdates(config UpdateConfig) ([]Update, error) {
	return bot.GetUpdatesWithContext(context.Background(), config)
}

// GetUpdatesWithContext fetches updates with a context.
func (bot *BotAPI) GetUpdatesWithContext(ctx context.Context, config UpdateConfig) ([]Update, error) {
	resp, err := bot.RequestWithContext(ctx, config)
	if err != nil {
		return []Update{}, err
	}

	var updates []Update
	err = json.Unmarshal(resp.Result, &updates)

	return updates, err
}

// GetWebhookInfo allows you to fetch information about a webhook and if
// one currently is set, along with pending update count and error messages.
func (bot *BotAPI) GetWebhookInfo() (WebhookInfo, error) {
	resp, err := bot.MakeRequest("getWebhookInfo", nil)
	if err != nil {
		return WebhookInfo{}, err
	}

	var info WebhookInfo
	err = json.Unmarshal(resp.Result, &info)

	return info, err
}

// GetUpdatesChan starts and returns a channel for getting updates.
func (bot *BotAPI) GetUpdatesChan(config UpdateConfig) UpdatesChannel {
	return bot.GetUpdatesChanContext(context.Background(), config)
}

// GetUpdatesChanContext starts and returns a channel for getting updates.
func (bot *BotAPI) GetUpdatesChanContext(ctx context.Context, config UpdateConfig) UpdatesChannel {
	ch := make(chan Update, bot.Buffer)

	go func() {
		for {
			select {
			case <-bot.shutdownChannel:
				close(ch)
				return
			case <-ctx.Done():
				close(ch)
				return
			default:
			}

			updates, err := bot.GetUpdatesWithContext(ctx, config)
			if err != nil {
				if err == context.Canceled {
					return
				}
				log.Println(err)
				log.Println("Failed to get updates, retrying in 3 seconds...")
				
				select {
				case <-bot.shutdownChannel:
					return
				case <-ctx.Done():
					return
				case <-time.After(time.Second * 3):
				}

				continue
			}

			for _, update := range updates {
				if update.UpdateID >= config.Offset {
					config.Offset = update.UpdateID + 1
					ch <- update
				}
			}
		}
	}()

	return ch
}

// StopReceivingUpdates stops the go routine which receives updates
func (bot *BotAPI) StopReceivingUpdates() {
	close(bot.shutdownChannel)
}

// ListenForWebhook registers a http handler for a webhook.
func (bot *BotAPI) ListenForWebhook(pattern string) UpdatesChannel {
	ch := make(chan Update, bot.Buffer)

	http.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		update, err := bot.HandleUpdate(r)
		if err != nil {
			errMsg, _ := json.Marshal(map[string]string{"error": err.Error()})
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(errMsg)
			return
		}

		ch <- *update
	})

	return ch
}

// ListenForWebhookRespReqFormat registers a http handler for a single incoming webhook.
func (bot *BotAPI) ListenForWebhookRespReqFormat(w http.ResponseWriter, r *http.Request) UpdatesChannel {
	ch := make(chan Update, bot.Buffer)

	func(w http.ResponseWriter, r *http.Request) {
		defer close(ch)

		update, err := bot.HandleUpdate(r)
		if err != nil {
			errMsg, _ := json.Marshal(map[string]string{"error": err.Error()})
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(errMsg)
			return
		}

		ch <- *update
	}(w, r)

	return ch
}

// HandleUpdate parses and returns update received via webhook
func (bot *BotAPI) HandleUpdate(r *http.Request) (*Update, error) {
	if r.Method != http.MethodPost {
		err := errors.New("wrong HTTP method required POST")
		return nil, err
	}

	var update Update
	err := json.NewDecoder(r.Body).Decode(&update)
	if err != nil {
		return nil, err
	}

	return &update, nil
}

// WriteToHTTPResponse writes the request to the HTTP ResponseWriter.
//
// It doesn't support uploading files.
//
// See https://core.telegram.org/bots/api#making-requests-when-getting-updates
// for details.
func WriteToHTTPResponse(w http.ResponseWriter, c Chattable) error {
	params, err := c.params()
	if err != nil {
		return err
	}

	if t, ok := c.(Fileable); ok {
		if hasFilesNeedingUpload(t.files()) {
			return errors.New("unable to use http response to upload files")
		}
	}

	values := buildParams(params)
	values.Set("method", c.method())

	w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
	_, err = w.Write([]byte(values.Encode()))
	return err
}

// GetChat gets information about a chat.
func (bot *BotAPI) GetChat(config ChatInfoConfig) (Chat, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return Chat{}, err
	}

	var chat Chat
	err = json.Unmarshal(resp.Result, &chat)

	return chat, err
}

// GetChatAdministrators gets a list of administrators in the chat.
//
// If none have been appointed, only the creator will be returned.
// Bots are not shown, even if they are an administrator.
func (bot *BotAPI) GetChatAdministrators(config ChatAdministratorsConfig) ([]ChatMember, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return []ChatMember{}, err
	}

	var members []ChatMember
	err = json.Unmarshal(resp.Result, &members)

	return members, err
}

// GetChatMembersCount gets the number of users in a chat.
func (bot *BotAPI) GetChatMembersCount(config ChatMemberCountConfig) (int, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return -1, err
	}

	var count int
	err = json.Unmarshal(resp.Result, &count)

	return count, err
}

// GetChatMember gets a specific chat member.
func (bot *BotAPI) GetChatMember(config GetChatMemberConfig) (ChatMember, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return ChatMember{}, err
	}

	var member ChatMember
	err = json.Unmarshal(resp.Result, &member)

	return member, err
}

// GetGameHighScores allows you to get the high scores for a game.
func (bot *BotAPI) GetGameHighScores(config GetGameHighScoresConfig) ([]GameHighScore, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return []GameHighScore{}, err
	}

	var highScores []GameHighScore
	err = json.Unmarshal(resp.Result, &highScores)

	return highScores, err
}

// GetInviteLink get InviteLink for a chat
func (bot *BotAPI) GetInviteLink(config ChatInviteLinkConfig) (string, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return "", err
	}

	var inviteLink string
	err = json.Unmarshal(resp.Result, &inviteLink)

	return inviteLink, err
}

// GetStickerSet returns a StickerSet.
func (bot *BotAPI) GetStickerSet(config GetStickerSetConfig) (StickerSet, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return StickerSet{}, err
	}

	var stickers StickerSet
	err = json.Unmarshal(resp.Result, &stickers)

	return stickers, err
}

// StopPoll stops a poll and returns the result.
func (bot *BotAPI) StopPoll(config StopPollConfig) (Poll, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return Poll{}, err
	}

	var poll Poll
	err = json.Unmarshal(resp.Result, &poll)

	return poll, err
}

// GetMyCommands gets the currently registered commands.
func (bot *BotAPI) GetMyCommands() ([]BotCommand, error) {
	return bot.GetMyCommandsWithConfig(GetMyCommandsConfig{})
}

// GetMyCommandsWithConfig gets the currently registered commands with a config.
func (bot *BotAPI) GetMyCommandsWithConfig(config GetMyCommandsConfig) ([]BotCommand, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return nil, err
	}

	var commands []BotCommand
	err = json.Unmarshal(resp.Result, &commands)

	return commands, err
}

// CopyMessage copy messages of any kind. The method is analogous to the method
// forwardMessage, but the copied message doesn't have a link to the original
// message. Returns the MessageID of the sent message on success.
func (bot *BotAPI) CopyMessage(config CopyMessageConfig) (MessageID, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return MessageID{}, err
	}

	var messageID MessageID
	err = json.Unmarshal(resp.Result, &messageID)

	return messageID, err
}

// AnswerWebAppQuery sets the result of an interaction with a Web App and send a
// corresponding message on behalf of the user to the chat from which the query originated.
func (bot *BotAPI) AnswerWebAppQuery(config AnswerWebAppQueryConfig) (SentWebAppMessage, error) {
	var sentWebAppMessage SentWebAppMessage

	resp, err := bot.Request(config)
	if err != nil {
		return sentWebAppMessage, err
	}

	err = json.Unmarshal(resp.Result, &sentWebAppMessage)
	return sentWebAppMessage, err
}

// GetMyDefaultAdministratorRights gets the current default administrator rights of the bot.
func (bot *BotAPI) GetMyDefaultAdministratorRights(config GetMyDefaultAdministratorRightsConfig) (ChatAdministratorRights, error) {
	var rights ChatAdministratorRights

	resp, err := bot.Request(config)
	if err != nil {
		return rights, err
	}

	err = json.Unmarshal(resp.Result, &rights)
	return rights, err
}

// EscapeText takes an input text and escape Telegram markup symbols.
// In this way we can send a text without being afraid of having to escape the characters manually.
// Note that you don't have to include the formatting style in the input text, or it will be escaped too.
// If there is an error, an empty string will be returned.
//
// parseMode is the text formatting mode (ModeMarkdown, ModeMarkdownV2 or ModeHTML)
// text is the input string that will be escaped
func EscapeText(parseMode string, text string) string {
	var replacer *strings.Replacer

	if parseMode == ModeHTML {
		replacer = strings.NewReplacer("<", "&lt;", ">", "&gt;", "&", "&amp;")
	} else if parseMode == ModeMarkdown {
		replacer = strings.NewReplacer("_", "\\_", "*", "\\*", "`", "\\`", "[", "\\[")
	} else if parseMode == ModeMarkdownV2 {
		replacer = strings.NewReplacer(
			"_", "\\_", "*", "\\*", "[", "\\[", "]", "\\]", "(",
			"\\(", ")", "\\)", "~", "\\~", "`", "\\`", ">", "\\>",
			"#", "\\#", "+", "\\+", "-", "\\-", "=", "\\=", "|",
			"\\|", "{", "\\{", "}", "\\}", ".", "\\.", "!", "\\!",
		)
	} else {
		return ""
	}

	return replacer.Replace(text)
}

// SendMessageDraft sends a message draft and returns true on success.
func (bot *BotAPI) SendMessageDraft(config MessageDraftConfig) (bool, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return false, err
	}

	return resp.Ok, nil
}

// SetMessageReaction sets a message reaction.
// Returns true on success.
func (bot *BotAPI) SetMessageReaction(config SetMessageReactionConfig) (bool, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return false, err
	}

	return resp.Ok, nil
}

// CreateForumTopic creates a new forum topic.
func (bot *BotAPI) CreateForumTopic(config CreateForumTopicConfig) (ForumTopic, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return ForumTopic{}, err
	}

	var topic ForumTopic
	err = json.Unmarshal(resp.Result, &topic)

	return topic, err
}

// EditForumTopic edits a forum topic.
func (bot *BotAPI) EditForumTopic(config EditForumTopicConfig) (bool, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return false, err
	}

	return resp.Ok, nil
}

// CloseForumTopic closes a forum topic.
func (bot *BotAPI) CloseForumTopic(config CloseForumTopicConfig) (bool, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return false, err
	}

	return resp.Ok, nil
}

// DeleteForumTopic deletes a forum topic.
func (bot *BotAPI) DeleteForumTopic(config DeleteForumTopicConfig) (bool, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return false, err
	}

	return resp.Ok, nil
}

// ReopenForumTopic reopens a forum topic.
func (bot *BotAPI) ReopenForumTopic(config ReopenForumTopicConfig) (bool, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return false, err
	}

	return resp.Ok, nil
}

// UnpinAllForumTopicMessages unpins all messages in a forum topic.
func (bot *BotAPI) UnpinAllForumTopicMessages(config UnpinAllForumTopicMessagesConfig) (bool, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return false, err
	}

	return resp.Ok, nil
}

// GetForumTopicIconStickers gets forum topic icon stickers.
func (bot *BotAPI) GetForumTopicIconStickers(config GetForumTopicIconStickersConfig) ([]Sticker, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return nil, err
	}

	var stickers []Sticker
	err = json.Unmarshal(resp.Result, &stickers)

	return stickers, err
}

// EditGeneralForumTopic edits the general forum topic.
func (bot *BotAPI) EditGeneralForumTopic(config EditGeneralForumTopicConfig) (bool, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return false, err
	}

	return resp.Ok, nil
}

// CloseGeneralForumTopic closes the general forum topic.
func (bot *BotAPI) CloseGeneralForumTopic(config CloseGeneralForumTopicConfig) (bool, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return false, err
	}

	return resp.Ok, nil
}

// ReopenGeneralForumTopic reopens the general forum topic.
func (bot *BotAPI) ReopenGeneralForumTopic(config ReopenGeneralForumTopicConfig) (bool, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return false, err
	}

	return resp.Ok, nil
}

// HideGeneralForumTopic hides the general forum topic.
func (bot *BotAPI) HideGeneralForumTopic(config HideGeneralForumTopicConfig) (bool, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return false, err
	}

	return resp.Ok, nil
}

// UnhideGeneralForumTopic unhides the general forum topic.
func (bot *BotAPI) UnhideGeneralForumTopic(config UnhideGeneralForumTopicConfig) (bool, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return false, err
	}

	return resp.Ok, nil
}

// UnpinAllGeneralForumTopicMessages unpins all messages in the general forum topic.
func (bot *BotAPI) UnpinAllGeneralForumTopicMessages(config UnpinAllGeneralForumTopicMessagesConfig) (bool, error) {
	resp, err := bot.Request(config)
	if err != nil {
		return false, err
	}

	return resp.Ok, nil
}
