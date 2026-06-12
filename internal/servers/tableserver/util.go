package tableserver

// preferNoContent reports whether the client asked for a no-content response.
func preferNoContent(prefer string) bool {
	return prefer == "return-no-content"
}
