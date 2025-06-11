package util

// EthereumAddressesToStrings converts a slice of EthereumAddress to their lowercase hex string representation.
func EthereumAddressesToStrings(addrs []EthereumAddress) []string {
	strs := make([]string, len(addrs))
	for i, a := range addrs {
		strs[i] = a.Address()
	}
	return strs
}
