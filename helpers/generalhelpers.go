package helpers

import (
	"net"
)

// BaseAddress of the program
const BaseAddress = "127.0.0.1"

// Message struct
type Message struct {
	Text        string
	Destination *string
	File        *string
	Request     *[]byte
	Keywords    *string
	Budget      *uint64
}

// ErrorCheck to log errors
func ErrorCheck(err error) {
	if err != nil {
		panic(err)
		//os.Exit(1)
	}
}

// DifferenceString to do the difference between two string sets
func DifferenceString(list1, list2 []*net.UDPAddr) []*net.UDPAddr {
	mapList2 := make(map[string]struct{}, len(list2))
	for _, x := range list2 {
		mapList2[x.String()] = struct{}{}
	}
	var difference []*net.UDPAddr
	for _, x := range list1 {
		_, check := mapList2[x.String()]
		if !check {
			difference = append(difference, x)
		}
	}
	return difference
}

// GetArrayStringFromAddresses array conversion from address to string
func GetArrayStringFromAddresses(peers []*net.UDPAddr) []string {
	list := make([]string, 0)
	for _, p := range peers {
		list = append(list, p.String())
	}
	return list
}

// RemoveDuplicatesFromSlice utility
func RemoveDuplicatesFromSlice(slice []string) []string {
	found := make(map[string]bool)
	for i := range slice {
		found[slice[i]] = true
	}

	result := []string{}
	for key := range found {
		result = append(result, key)
	}
	return result
}
