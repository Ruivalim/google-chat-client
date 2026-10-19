package chat

type Account struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Domain string `json:"domain"`
}

type State struct {
	Configured      bool      `json:"configured"`
	ClientID        string    `json:"clientId"`
	Accounts        []Account `json:"accounts"`
	ActiveAccountID string    `json:"activeAccountId"`
}

type Space struct {
	Name                string `json:"name"`
	DisplayName         string `json:"displayName"`
	SpaceType           string `json:"spaceType"`
	SpaceThreadingState string `json:"spaceThreadingState"`
	LastActiveTime      string `json:"lastActiveTime"`
	SpaceURI            string `json:"spaceUri"`
}

type SpacePage struct {
	Spaces        []Space `json:"spaces"`
	NextPageToken string  `json:"nextPageToken"`
}

type User struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Type        string `json:"type"`
}

type Thread struct {
	Name string `json:"name"`
}

type Attachment struct {
	ContentName string `json:"contentName"`
	DownloadURI string `json:"downloadUri"`
}

type Message struct {
	Name        string       `json:"name"`
	Text        string       `json:"text"`
	CreateTime  string       `json:"createTime"`
	Sender      User         `json:"sender"`
	Thread      Thread       `json:"thread"`
	ThreadReply bool         `json:"threadReply"`
	Attachment  []Attachment `json:"attachment,omitempty"`
}

type MessagePage struct {
	Messages      []Message `json:"messages"`
	NextPageToken string    `json:"nextPageToken"`
}

type config struct {
	ClientID        string    `json:"clientId"`
	Accounts        []Account `json:"accounts"`
	ActiveAccountID string    `json:"activeAccountId"`
}

type Credentials struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type Vault interface {
	Get(string) ([]byte, error)
	Set(string, []byte) error
	Delete(string) error
}
