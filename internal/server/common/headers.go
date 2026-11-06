package common

import "connectrpc.com/connect"

func GetLocaleFromRequest[T any](req *connect.Request[T]) string {
	acceptLang := req.Header().Get("Accept-Language")

	if acceptLang != "" && len(acceptLang) >= 2 {
		return acceptLang[:2]
	}

	return "en"
}
