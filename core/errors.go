package core

type BridgeError struct {
	Code    string
	Message string
}

func (e BridgeError) Error() string { return e.Message }

func Err(code, message string) error { return BridgeError{Code: code, Message: message} }

func ErrorMap(err error) map[string]any {
	if e, ok := err.(BridgeError); ok {
		return map[string]any{"ok": false, "error": map[string]any{"code": e.Code, "message": e.Message}}
	}
	return map[string]any{"ok": false, "error": map[string]any{"code": "internal_error", "message": "Bridge operation failed"}}
}
