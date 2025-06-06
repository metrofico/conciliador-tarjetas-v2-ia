package utils

import "strconv"

func StringToInt(str string) int {
	// Convertir el string a int
	value, err := strconv.Atoi(str)
	if err != nil {
		return 0
	}

	return value
}
