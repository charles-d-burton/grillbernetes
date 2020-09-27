package main

// +build aix darwin dragonfly freebsd linux nacl netbsd openbsd solaris

func setLocal(key, value string) {
	//Does nothing, not used
}

func getLocalString(key string) string {
	//Does nothing, not used
	return ""
}
