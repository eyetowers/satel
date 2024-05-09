package satel

func isUserCodeValid(usercode string) bool {
	if len(usercode) != 4 {
		return false
	}

	for _, char := range usercode {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
