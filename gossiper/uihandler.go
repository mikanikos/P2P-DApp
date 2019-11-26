package gossiper

// FileGUI struct
type FileGUI struct {
	Name     string
	MetaHash string
	Size     int64
}

// UIHandler struct
type UIHandler struct {
	filesIndexed    chan *FileGUI
	filesDownloaded chan *FileGUI
	filesSearched   []FileGUI
	latestRumors    chan *RumorMessage
	latestTLC       chan *TLCMessage
}

// NewUIHandler create new file handler
func NewUIHandler() *UIHandler {
	return &UIHandler{
		filesIndexed:    make(chan *FileGUI, 3),
		filesDownloaded: make(chan *FileGUI, 3),
		filesSearched:   make([]FileGUI, 0),
		latestRumors:    make(chan *RumorMessage, latestMessagesBuffer),
		latestTLC:       make(chan *TLCMessage, latestMessagesBuffer),
	}
}

// GetMessagesList for GUI
func GetMessagesList(channel chan *RumorMessage) []RumorMessage {

	bufferLength := len(channel)
	messages := make([]RumorMessage, bufferLength)
	for i := 0; i < bufferLength; i++ {
		message := <-channel
		messages[i] = *message
	}

	return messages
}

// GetFilesList for GUI
func GetFilesList(channel chan *FileGUI) []FileGUI {

	bufferLength := len(channel)
	files := make([]FileGUI, bufferLength)
	for i := 0; i < bufferLength; i++ {
		file := <-channel
		files[i] = *file
	}
	return files
}
