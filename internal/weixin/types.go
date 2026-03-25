package weixin

type QRStartResponse struct {
	QRCode         string `json:"qrcode"`
	QRCodeImageURL string `json:"qrcode_img_content"`
}

type QRStatusResponse struct {
	Status     string `json:"status"`
	BotToken   string `json:"bot_token"`
	ILinkBotID string `json:"ilink_bot_id"`
	BaseURL    string `json:"baseurl"`
	UserID     string `json:"ilink_user_id"`
}

type GetUpdatesRequest struct {
	GetUpdatesBuf string `json:"get_updates_buf"`
}

type GetUpdatesResponse struct {
	Ret                  int       `json:"ret"`
	ErrCode              int       `json:"errcode"`
	ErrMessage           string    `json:"errmsg"`
	Messages             []Message `json:"msgs"`
	GetUpdatesBuf        string    `json:"get_updates_buf"`
	LongPollingTimeoutMs int       `json:"longpolling_timeout_ms"`
}

type Message struct {
	FromUserID   string        `json:"from_user_id"`
	ToUserID     string        `json:"to_user_id"`
	CreateTimeMs int64         `json:"create_time_ms"`
	ContextToken string        `json:"context_token"`
	MessageType  int           `json:"message_type"`
	ItemList     []MessageItem `json:"item_list"`
}

type MessageItem struct {
	Type     int       `json:"type"`
	TextItem *TextItem `json:"text_item,omitempty"`
}

type TextItem struct {
	Text string `json:"text"`
}

type SendMessageRequest struct {
	Msg OutboundMessage `json:"msg"`
}

type OutboundMessage struct {
	FromUserID   string         `json:"from_user_id"`
	ToUserID     string         `json:"to_user_id"`
	ClientID     string         `json:"client_id"`
	MessageType  int            `json:"message_type"`
	MessageState int            `json:"message_state"`
	ContextToken string         `json:"context_token,omitempty"`
	ItemList     []OutboundItem `json:"item_list"`
}

type OutboundItem struct {
	Type     int           `json:"type"`
	TextItem *OutboundText `json:"text_item,omitempty"`
}

type OutboundText struct {
	Text string `json:"text"`
}

type AckResponse struct {
	Ret        int    `json:"ret"`
	ErrCode    int    `json:"errcode"`
	ErrMessage string `json:"errmsg"`
}

type GetConfigRequest struct {
	ILinkUserID  string `json:"ilink_user_id"`
	ContextToken string `json:"context_token,omitempty"`
}

type GetConfigResponse struct {
	Ret          int    `json:"ret"`
	TypingTicket string `json:"typing_ticket"`
}

type SendTypingRequest struct {
	ILinkUserID  string `json:"ilink_user_id"`
	TypingTicket string `json:"typing_ticket"`
	Status       int    `json:"status"`
}
