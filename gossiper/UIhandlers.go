package gossiper

// FileGUI struct
type FileGUI struct {
	Name     string
	MetaHash string
	Size     int64
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
