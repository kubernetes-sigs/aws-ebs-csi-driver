package template

import (
	"fmt"
	"html/template"
	"strings"
)

// Disable functions
func html(args ...interface{}) (string, error) {
	return "", fmt.Errorf("cannot call 'html' function")
}

func js(args ...interface{}) (string, error) {
	return "", fmt.Errorf("cannot call 'js' function")
}

func call(args ...interface{}) (string, error) {
	return "", fmt.Errorf("cannot call 'call' function")
}

func urlquery(args ...interface{}) (string, error) {
	return "", fmt.Errorf("cannot call 'urlquery' function")
}

func contains(arg1, arg2 string) bool {
	return strings.Contains(arg2, arg1)
}

func substring(start, end int, arg string) string {
	if start < 0 {
		return arg[:end]
	}

	if end < 0 || end > len(arg) {
		return arg[start:]
	}

	return arg[start:end]
}

func field(delim string, idx int, arg string) (string, error) {
	w := strings.Split(arg, delim)
	if idx >= len(w) {
		return "", fmt.Errorf("extractWord: cannot index into split string; index = %d, length = %d", idx, len(w))
	}
	return w[idx], nil
}

func index(arg1, arg2 string) int {
	return strings.Index(arg2, arg1)
}

func lastIndex(arg1, arg2 string) int {
	return strings.LastIndex(arg2, arg1)
}

func newFuncMap() template.FuncMap {
	return template.FuncMap{
		"html":      html,
		"js":        js,
		"call":      call,
		"urlquery":  urlquery,
		"contains":  contains,
		"toUpper":   strings.ToUpper,
		"toLower":   strings.ToLower,
		"substring": substring,
		"field":     field,
		"index":     index,
		"lastIndex": lastIndex,
	}
}
