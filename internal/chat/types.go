package chat

type Account struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Domain string `json:"domain"`
	// ClientID é o cliente OAuth do projeto pelo qual a conta entrou; a sessão
	// só renova com as credenciais desse mesmo projeto.
	ClientID string `json:"clientId"`
}

// Project é um cliente OAuth importado de um projeto Google Cloud. O secret
// fica no chaveiro, sob projectKey(ClientID).
type Project struct {
	ClientID  string `json:"clientId"`
	ProjectID string `json:"projectId"`
}

type State struct {
	Projects        []Project     `json:"projects"`
	Accounts        []Account     `json:"accounts"`
	ActiveAccountID string        `json:"activeAccountId"`
	Notifications   Notifications `json:"notifications"`
}

// Notifications diz se o app procura mensagens novas e de quantos em quantos
// segundos.
type Notifications struct {
	On    bool `json:"on"`
	Every int  `json:"every"`
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
	Email       string `json:"email"`
	Type        string `json:"type"`
	IsAnonymous bool   `json:"isAnonymous"`
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
	Projects        []Project `json:"projects"`
	Accounts        []Account `json:"accounts"`
	ActiveAccountID string    `json:"activeAccountId"`
	// Zero nos dois quer dizer o padrão: ligado, a cada minuto.
	NotificationsOff bool `json:"notificationsOff,omitempty"`
	CheckEvery       int  `json:"checkEvery,omitempty"`
	// LegacyClientID é o formato antigo, de um projeto só. Open migra.
	LegacyClientID string `json:"clientId,omitempty"`
}

type Credentials struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	ProjectID    string `json:"project_id,omitempty"`
}

type Vault interface {
	Get(string) ([]byte, error)
	Set(string, []byte) error
	Delete(string) error
}
