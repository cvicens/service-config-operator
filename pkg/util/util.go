package util

import (

)

// NVL returns def if str is null
func NVL(str string, def string) string {
	if len(str) == 0 {
		return def
	}
	return str
}