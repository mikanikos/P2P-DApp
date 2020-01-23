package whisper

type Topic [TopicLength]byte

// BytesToTopic converts from the byte array representation of a topic
// into the Topic type.
func BytesToTopic(b []byte) (t Topic) {
	sz := TopicLength
	if x := len(b); x < TopicLength {
		sz = x
	}
	for i := 0; i < sz; i++ {
		t[i] = b[i]
	}
	return t
}

//// String converts a topic byte array to a string representation.
//func (t *Topic) String() string {
//	return hexutil.Encode(t[:])
//}

//// MarshalText returns the hex representation of t.
//func (t Topic) MarshalText() ([]byte, error) {
//	return []byte(t[:]).MarshalText()
//}
//
//// UnmarshalText parses a hex representation to a topic.
//func (t *Topic) UnmarshalText(input []byte) error {
//	return hexutil.UnmarshalFixedText("Topic", input, t[:])
//}
